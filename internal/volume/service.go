package volume

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	cboxErr "cbox/internal/errors"
	"cbox/internal/transport"
	"github.com/google/uuid"
)

type Repository interface {
	Create(ctx context.Context, v *Volume) error
	Get(ctx context.Context, idOrName string) (*Volume, error)
	List(ctx context.Context) ([]*Volume, error)
	UpdateHash(ctx context.Context, id string, hash string) error
	Delete(ctx context.Context, idOrName string) error
}

type Service struct {
	repo    Repository
	syncMgr *SyncManager
	logger  *slog.Logger
}

func NewService(repo Repository, syncInterval time.Duration, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		repo:    repo,
		syncMgr: NewSyncManager(syncInterval, logger),
		logger:  logger,
	}
}

func (s *Service) SyncManager() *SyncManager {
	return s.syncMgr
}

func ParseMountSpec(spec string) (*Mount, error) {
	cloud := strings.HasPrefix(spec, GoogleDrivePrefix)
	if cloud {
		spec = strings.TrimPrefix(spec, GoogleDrivePrefix)
	}
	parts := strings.Split(spec, ":")
	if len(parts) < 2 || len(parts) > 3 || parts[0] == "" || parts[1] == "" {
		return nil, fmt.Errorf("invalid volume mount spec: %q (expected source:target[:mode])", spec)
	}

	source := parts[0]
	target := parts[1]
	mode := ModeReadOnly

	if len(parts) >= 3 {
		switch parts[2] {
		case "ro":
			mode = ModeReadOnly
		case "rw":
			mode = ModeReadWrite
		case "output":
			mode = ModeOutput
		case "cache":
			mode = ModeCache
		default:
			return nil, fmt.Errorf("invalid volume mode: %q (expected ro, rw, output, or cache)", parts[2])
		}
	}

	if cloud {
		if err := ValidateGoogleDriveMount(source, target, mode); err != nil {
			return nil, err
		}
		return &Mount{Source: GoogleDrivePrefix + source, Target: target, Mode: mode}, nil
	}

	absSource, err := filepath.Abs(source)
	if err == nil {
		source = absSource
	}

	return &Mount{
		VolumeID: "",
		Source:   source,
		Target:   target,
		Mode:     mode,
	}, nil
}

func (s *Service) CreateVolume(ctx context.Context, name, source string, mode VolumeMode) (*Volume, error) {
	if name == "" {
		return nil, fmt.Errorf("volume name cannot be empty")
	}

	absSource, err := filepath.Abs(source)
	if err == nil {
		source = absSource
	}

	hash, _ := ComputeSourceHash(source)

	v := &Volume{
		ID:           uuid.New().String()[:8],
		Name:         name,
		Source:       source,
		Mode:         mode,
		Immutable:    mode == ModeReadOnly,
		ManifestHash: hash,
		CreatedAt:    time.Now().UTC(),
	}

	if s.repo != nil {
		if err := s.repo.Create(ctx, v); err != nil {
			return nil, err
		}
	}

	return v, nil
}

func (s *Service) GetVolume(ctx context.Context, idOrName string) (*Volume, error) {
	if s.repo != nil {
		return s.repo.Get(ctx, idOrName)
	}
	return nil, cboxErr.ErrNotFound
}

func (s *Service) ListVolumes(ctx context.Context) ([]*Volume, error) {
	if s.repo != nil {
		return s.repo.List(ctx)
	}
	return nil, nil
}

func (s *Service) DeleteVolume(ctx context.Context, idOrName string) error {
	if s.repo != nil {
		return s.repo.Delete(ctx, idOrName)
	}
	return nil
}

func (s *Service) PrepareMounts(ctx context.Context, trans transport.Transport, mounts []Mount, cachedVolumes map[string]string) error {
	for i, mnt := range mounts {
		if IsGoogleDriveMount(mnt) {
			if err := prepareGoogleDriveMount(ctx, trans, mnt); err != nil {
				return err
			}
			continue
		}
		// Ensure local source exists if read-only or read-write
		if mnt.Mode == ModeReadOnly || mnt.Mode == ModeReadWrite {
			if _, err := os.Stat(mnt.Source); err != nil {
				return fmt.Errorf("volume source %s does not exist: %w", mnt.Source, err)
			}
		} else if mnt.Mode == ModeOutput {
			// Ensure local output directory exists
			_ = os.MkdirAll(mnt.Source, 0755)
		}

		hash := ""
		if mnt.VolumeID != "" {
			hash = mnt.VolumeID
		} else {
			computed, err := ComputeSourceHash(mnt.Source)
			if err == nil && computed != "" {
				hash = computed[:12]
			} else {
				hash = uuid.New().String()[:8]
			}
		}

		remoteStorage := fmt.Sprintf("/content/.cbox/volumes/%s", hash)

		// Check if volume data is already cached on remote runtime
		isCached := false
		if cachedVolumes != nil && cachedVolumes[mnt.VolumeID] == hash {
			isCached = true
		}

		if !isCached {
			s.logger.Info("syncing volume to remote runtime", "source", mnt.Source, "remote", remoteStorage)
			// Ensure remote directory exists
			_, _ = trans.Exec(ctx, []string{"mkdir", "-p", remoteStorage}, transport.ExecOptions{})

			// For RO or RW or existing output, sync source to remote
			if mnt.Mode != ModeOutput || dirHasFiles(mnt.Source) {
				if err := trans.SyncTo(ctx, mnt.Source, remoteStorage, transport.SyncOptions{}); err != nil {
					return fmt.Errorf("sync volume %s to remote: %w", mnt.Source, err)
				}
			}

			if cachedVolumes != nil {
				volKey := mnt.VolumeID
				if volKey == "" {
					volKey = mnt.Source
				}
				cachedVolumes[volKey] = hash
			}
		} else {
			s.logger.Info("volume cache hit on remote runtime", "source", mnt.Source, "hash", hash)
		}

		// Now link or mount remoteStorage to mnt.Target
		// mkdir -p target's parent, remove existing target link, symlink remoteStorage to target
		targetParent := filepath.Dir(mnt.Target)
		linkCmd := fmt.Sprintf("mkdir -p %q && rm -rf %q && ln -sfn %q %q", targetParent, mnt.Target, remoteStorage, mnt.Target)
		res, err := trans.Exec(ctx, []string{"bash", "-c", linkCmd}, transport.ExecOptions{})
		if err != nil || res.ExitCode != 0 {
			return fmt.Errorf("link remote volume to %s: %v (exit %d)", mnt.Target, err, res.ExitCode)
		}

		mounts[i].VolumeID = hash
	}

	return nil
}

func dirHasFiles(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	return len(entries) > 0
}
