package cmd

import (
	"fmt"
	"os"

	"github.com/Pratyay360/md2smd/internal/convert"
	"github.com/Pratyay360/md2smd/internal/walk"
	"github.com/spf13/cobra"
)

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
			files, err := walk.Files(arg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error accessing %s: %v\n", arg, err)
				hasError = true
				continue
			}
			for _, f := range files {
				if err := convertFile(f); err != nil {
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

// convertFile converts path in the direction implied by its extension.
func convertFile(path string) error {
	direction := convert.DirectionFor(path)
	outputPath, err := direction.File(path)
	if err != nil {
		return fmt.Errorf("failed to convert %s to %s: %w", path, direction.Ext(), err)
	}
	fmt.Printf("Converted %s -> %s\n", path, outputPath)
	return nil
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
