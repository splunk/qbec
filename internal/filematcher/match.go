package filematcher

import (
	"fmt"
	"io/fs"
	"path/filepath"
)

// Match returns list of files and dirs matching a glob pattern
func Match(pattern string) ([]string, error) {
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
