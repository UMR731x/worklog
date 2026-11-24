package workspace

import (
	"os"
	"path/filepath"
)

type DirectoryListing struct {
	Path        string   `json:"path"`
	Parent      string   `json:"parent"`
	Directories []string `json:"directories"`
}

func ListDirectories(path string) (DirectoryListing, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return DirectoryListing{}, err
	}
	entries, err := os.ReadDir(absolute)
	if err != nil {
		return DirectoryListing{}, err
	}
	result := DirectoryListing{
		Path: absolute, Parent: filepath.Dir(absolute), Directories: []string{},
	}
	for _, entry := range entries {
		if entry.IsDir() {
			result.Directories = append(result.Directories, entry.Name())
		}
	}
	return result, nil
}
