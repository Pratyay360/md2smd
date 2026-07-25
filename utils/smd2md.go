package utils

import (
	"os"
	"path/filepath"
	"strings"
)

func Smd2Md(inputPath string) (string, error) {
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return "", err
	}

	output, err := SmdToMd(string(data))
	if err != nil {
		return "", err
	}

	ext := filepath.Ext(inputPath)
	outputPath := strings.TrimSuffix(inputPath, ext) + ".md"
	if outputPath == inputPath {
		outputPath = inputPath + ".md"
	}

	if err := os.WriteFile(outputPath, []byte(output), 0644); err != nil {
		return "", err
	}

	return outputPath, nil
}
