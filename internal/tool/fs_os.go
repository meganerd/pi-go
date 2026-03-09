package tool

import (
	"io/fs"
	"os"
	"path/filepath"
)

// ReadFile reads the named file and returns its contents.
func (f *OSFileSystem) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// WriteFile writes data to the named file, creating it if necessary.
func (f *OSFileSystem) WriteFile(path string, data []byte, perm uint32) error {
	return os.WriteFile(path, data, fs.FileMode(perm))
}

// Stat returns file info for the named file.
func (f *OSFileSystem) Stat(path string) (FileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return FileInfo{}, err
	}
	return FileInfo{
		Name:  info.Name(),
		Size:  info.Size(),
		IsDir: info.IsDir(),
		Mode:  uint32(info.Mode()),
	}, nil
}

// MkdirAll creates a directory path and all parents that do not yet exist.
func (f *OSFileSystem) MkdirAll(path string, perm uint32) error {
	return os.MkdirAll(path, fs.FileMode(perm))
}

// Glob returns the names of all files matching the pattern.
func (f *OSFileSystem) Glob(pattern string) ([]string, error) {
	return filepath.Glob(pattern)
}

// ReadDir reads the named directory and returns its entries.
func (f *OSFileSystem) ReadDir(path string) ([]DirEntry, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	result := make([]DirEntry, len(entries))
	for i, e := range entries {
		result[i] = DirEntry{
			Name:  e.Name(),
			IsDir: e.IsDir(),
		}
	}
	return result, nil
}

// Remove removes the named file or empty directory.
func (f *OSFileSystem) Remove(path string) error {
	return os.Remove(path)
}
