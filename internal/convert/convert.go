// Package convert applies the Markdown/SuperMD converters to files on disk.
package convert

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/Pratyay360/md2smd/internal/smd"
)

// Direction selects which converter a path is run through.
type Direction int

const (
	// ToSmd converts Markdown to SuperMD.
	ToSmd Direction = iota
	// ToMd converts SuperMD to Markdown.
	ToMd
)

// Ext returns the file extension this direction produces.
func (d Direction) Ext() string {
	if d == ToMd {
		return ".md"
	}
	return ".smd"
}

func (d Direction) convert(content string) (string, error) {
	if d == ToMd {
		return smd.SmdToMd(content)
	}
	return smd.MdToSmd(content)
}

// File converts a single file and writes the result next to it, returning the
// path of the file that was written.
func (d Direction) File(inputPath string) (string, error) {
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return "", err
	}
	output, err := d.convert(string(data))
	if err != nil {
		return "", err
	}
	outputPath := rebaseExt(inputPath, d.Ext())
	if err := os.WriteFile(outputPath, []byte(output), 0644); err != nil {
		return "", err
	}
	return outputPath, nil
}

// rebaseExt swaps a path's extension, falling back to appending when the
// current extension already is the target one.
func rebaseExt(path, ext string) string {
	trimmed := strings.TrimSuffix(path, filepath.Ext(path)) + ext
	if trimmed == path {
		return path + ext
	}
	return trimmed
}

// Md2Smd converts a Markdown file to SuperMD, writing the sibling .smd file.
func Md2Smd(inputPath string) (string, error) {
	return ToSmd.File(inputPath)
}

// Smd2Md converts a SuperMD file to Markdown, writing the sibling .md file.
func Smd2Md(inputPath string) (string, error) {
	return ToMd.File(inputPath)
}

// DirectionFor picks the converter to use for a path: .smd is SuperMD and
// anything else is treated as Markdown.
func DirectionFor(path string) Direction {
	if strings.EqualFold(filepath.Ext(path), ".smd") {
		return ToMd
	}
	return ToSmd
}
