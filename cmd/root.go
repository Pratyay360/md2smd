package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/Pratyay360/md2smd/utils"
	"github.com/spf13/cobra"
)

func isSmdFile(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".smd")
}

func collectFiles(path string) ([]string, error) {
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
		ext := strings.ToLower(filepath.Ext(p))
		// For directory walks, only consider markdown/MDX/SMD files.
		// Single-file invocations (collectFiles on a file) are handled earlier
		// and allow any extension via convertSingleFile (any non-.smd -> markdown).
		switch ext {
		case ".smd", ".md", ".mdx", ".markdown", ".mkd", ".mkdown", ".mdown", ".mdwn", ".txt":
			files = append(files, p)
		case "":
			// No extension - treat as potential markdown (common for some MDX setups)
			files = append(files, p)
		default:
			// Skip non-markdown files (images, binaries, etc.)
		}
		return nil
	})
	return files, err
}

var repairSmd bool

func convertSingleFile(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("file not found: %s", path)
	}
	// Repair mode: fix heading levels + strip HTML comments in existing .smd files
	if repairSmd && isSmdFile(path) {
		outputPath, err := utils.RepairSmdFile(path)
		if err != nil {
			return fmt.Errorf("failed to repair SMD %s: %w", path, err)
		}
		fmt.Printf("Repaired %s\n", outputPath)
		return nil
	}
	// Any .smd file -> SMD to MD, otherwise treat as markdown/MDX -> MD to SMD
	// This allows "any type of md/mdx" regardless of extension (case-insensitive)
	if isSmdFile(path) {
		outputPath, err := utils.Smd2Md(path)
		if err != nil {
			return fmt.Errorf("failed to convert SMD to MD %s: %w", path, err)
		}
		fmt.Printf("Converted %s -> %s\n", path, outputPath)
		return nil
	}
	// Default: treat as markdown/MDX (covers .md, .mdx, .markdown, .txt, no ext, etc.)
	outputPath, err := utils.Md2Smd(path)
	if err != nil {
		return fmt.Errorf("failed to convert MD to SMD %s: %w", path, err)
	}
	fmt.Printf("Converted %s -> %s\n", path, outputPath)
	return nil
}

var rootCmd = &cobra.Command{
	Use:   "md2smd [file|dir...]",
	Short: "Convert between Markdown and SuperMD (Zine) formats",
	Long: `md2smd converts Markdown files to SuperMD (.smd) format used by 
the Zine static site generator, and vice versa. SuperMD is an extension of Markdown that uses Scripty expressions embedded in link syntax for directives like images, links, sections, 
and blocks.

Supports any Markdown/MDX flavour regardless of file extension (.md, .mdx, .markdown, .mkd, etc.) -
any non-.smd file is treated as Markdown. Directories are walked recursively and all
convertible files are processed. Extensions are matched case-insensitively.`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var hasError bool
		for _, arg := range args {
			files, err := collectFiles(arg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error accessing %s: %v\n", arg, err)
				hasError = true
				continue
			}
			for _, f := range files {
				if err := convertSingleFile(f); err != nil {
					fmt.Fprintf(os.Stderr, "%v\n", err)
					hasError = true
				}
			}
		}
		if hasError {
			return fmt.Errorf("one or more conversions failed")
		}
		return nil
	},
}

func init() {
	rootCmd.Flags().BoolVar(&repairSmd, "repair", false, "Repair existing .smd files in-place (fix heading levels, strip HTML comments)")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}