package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/Pratyay360/md2smd/utils"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "md2smd [file]",
	Short: "Convert between Markdown and SuperMD (Zine) formats",
	Long: `md2smd converts Markdown files to SuperMD (.smd) format used by 
the Zine static site generator, and vice versa. SuperMD is an extension of Markdown that uses Scripty expressions embedded in link syntax for directives like images, links, sections, 
and blocks.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := args[0]

		if _, err := os.Stat(path); os.IsNotExist(err) {
			return fmt.Errorf("file not found: %s", path)
		}
		switch {
		case strings.HasSuffix(path, ".md"), strings.HasSuffix(path, ".mdx"):
			outputPath, err := utils.Md2Smd(path)
			if err != nil {
				return fmt.Errorf("failed to convert MD to SMD: %w", err)
			}
			fmt.Printf("Converted %s -> %s\n", path, outputPath)
			return nil

		case strings.HasSuffix(path, ".smd"):
			outputPath, err := utils.Smd2Md(path)
			if err != nil {
				return fmt.Errorf("failed to convert SMD to MD: %w", err)
			}
			fmt.Printf("Converted %s -> %s\n", path, outputPath)
			return nil

		default:
			return fmt.Errorf("unsupported file extension: %s (must be .md, .mdx, or .smd)", path)
		}
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}