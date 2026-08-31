package utils

import (
	"os"
	"path/filepath"
	"strings"
)

func RepairSmdFile(inputPath string) (string, error) {
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return "", err
	}
	output, err := RepairSmdContent(string(data))
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(inputPath, []byte(output), 0644); err != nil {
		return "", err
	}
	return inputPath, nil
}

func Md2Smd(inputPath string) (string, error) {
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return "", err
	}

	output, err := MdToSmd(string(data))
	if err != nil {
		return "", err
	}

	ext := filepath.Ext(inputPath)
	outputPath := strings.TrimSuffix(inputPath, ext) + ".smd"
	if outputPath == inputPath {
		outputPath = inputPath + ".smd"
	}

	if err := os.WriteFile(outputPath, []byte(output), 0644); err != nil {
		return "", err
	}

	return outputPath, nil
}
