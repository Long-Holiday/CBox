package volume

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

func ComputeSourceHash(sourcePath string) (string, error) {
	info, err := os.Stat(sourcePath)
	if err != nil {
		return "", err
	}

	h := sha256.New()
	if !info.IsDir() {
		f, err := os.Open(sourcePath)
		if err != nil {
			return "", err
		}
		defer f.Close()
		if _, err := io.Copy(h, f); err != nil {
			return "", err
		}
		return hex.EncodeToString(h.Sum(nil)), nil
	}

	var files []string
	err = filepath.Walk(sourcePath, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !fi.IsDir() {
			rel, _ := filepath.Rel(sourcePath, path)
			files = append(files, rel)
		}
		return nil
	})
	if err != nil {
		return "", err
	}

	sort.Strings(files)
	for _, rel := range files {
		fullPath := filepath.Join(sourcePath, rel)
		fi, err := os.Stat(fullPath)
		if err != nil {
			continue
		}
		// Hash relative path and modification time/size
		_, _ = fmt.Fprintf(h, "%s:%d:%d\n", rel, fi.Size(), fi.ModTime().Unix())
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
