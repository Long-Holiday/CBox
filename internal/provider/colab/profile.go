package colab

import (
	"os"
	"path/filepath"
)

type Profile struct {
	Name       string
	HomeDir    string
	ConfigFile string
	OAuthFile  string
}

type ProfileStore struct {
	baseDir string
}

func NewProfileStore(baseDir string) *ProfileStore {
	return &ProfileStore{baseDir: baseDir}
}

func (s *ProfileStore) GetProfile(name string) (*Profile, error) {
	if name == "" {
		name = "default"
	}
	profileDir := filepath.Join(s.baseDir, name)
	homeDir := filepath.Join(profileDir, "home")

	p := &Profile{
		Name:       name,
		HomeDir:    homeDir,
		ConfigFile: filepath.Join(profileDir, "config.json"),
		OAuthFile:  filepath.Join(profileDir, "oauth.json"),
	}
	// The default profile shares credentials with `colab sessions` / cbox setup.
	// Named profiles keep an isolated home for independent Google accounts.
	if name == "default" {
		p.HomeDir = ""
		p.OAuthFile = ""
	}
	return p, nil
}

func (s *ProfileStore) EnsureProfile(name string) (*Profile, error) {
	p, err := s.GetProfile(name)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(p.ConfigFile), 0700); err != nil {
		return nil, err
	}
	if p.HomeDir != "" {
		if err := os.MkdirAll(p.HomeDir, 0700); err != nil {
			return nil, err
		}
	}
	return p, nil
}

func (s *ProfileStore) ListProfiles() ([]*Profile, error) {
	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var list []*Profile
	for _, e := range entries {
		if e.IsDir() {
			p, err := s.GetProfile(e.Name())
			if err == nil {
				list = append(list, p)
			}
		}
	}
	return list, nil
}
