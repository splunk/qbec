package filematcher

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

// Match returns list of files and dirs matching a glob pattern
func Match(pattern string) ([]string, error) {
	if IsRemoteFile(pattern) {
		return []string{pattern}, nil
	}
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	// files is nil when pattern does not match any files
	if files == nil {
		return nil, fmt.Errorf("%s: %w", pattern, fs.ErrNotExist)
	}
	var envFiles []string
	for _, f := range files {
		abs, err := filepath.Abs(f)
		if err != nil {
			return nil, err
		}
		envFiles = append(envFiles, abs)
	}
	return envFiles, nil
}

// IsRemoteFile distinguishes remote files from local files
func IsRemoteFile(file string) bool {
	return strings.HasPrefix(file, "http://") || strings.HasPrefix(file, "https://")
}
