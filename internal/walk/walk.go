// Package walk discovers the files a conversion run should visit.
package walk

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// convertibleExts are the extensions picked up when walking a directory.
// Extension-less files are also collected, since many MDX setups use them.
var convertibleExts = map[string]bool{
	".smd":      true,
	".md":       true,
	".mdx":      true,
	".markdown": true,
	".mkd":      true,
	".mkdown":   true,
	".mdown":    true,
	".mdwn":     true,
	".txt":      true,
}

// Files returns the convertible files at path. A directory is walked
// recursively, skipping hidden directories; any other path is returned as is.
func Files(path string) ([]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return []string{path}, nil
	}
	var files []string
	err = filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// Skip hidden dirs like .git
			if strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if convertibleExts[strings.ToLower(filepath.Ext(p))] || filepath.Ext(p) == "" {
			files = append(files, p)
		}
		return nil
	})
	return files, err
}
