package smd

import (
	"fmt"
	"strings"
)

// extractFencedBlocks replaces every ``` / ~~~ fenced code block with an opaque
// placeholder so that link and image rewriting never touches code samples.
// The blocks map holds placeholder -> original block, and is passed to
// restoreBlocks once rewriting is done.
func extractFencedBlocks(input string) (string, map[string]string) {
	blocks := make(map[string]string)
	var result strings.Builder
	i := 0
	idx := 0
	for i < len(input) {
		// Find next fence of either ``` or ~~~ (including language info)
		backIdx := strings.Index(input[i:], "```")
		tildeIdx := strings.Index(input[i:], "~~~")
		var start int
		var fence string
		if backIdx < 0 && tildeIdx < 0 {
			result.WriteString(input[i:])
			break
		}
		if backIdx >= 0 && (tildeIdx < 0 || backIdx < tildeIdx) {
			start = backIdx
			fence = "```"
		} else {
			start = tildeIdx
			fence = "~~~"
		}
		result.WriteString(input[i : i+start])
		// Find closing fence same as opening
		end := strings.Index(input[i+start+3:], fence)
		if end < 0 {
			// No closing fence, treat rest as block
			result.WriteString(input[i+start:])
			break
		}
		end += 3
		// Include fence lines completely: from opening fence to after closing fence.
		// The simple search keeps the language specifier as part of the block.
		block := input[i+start : i+start+end+3]
		placeholder := fmt.Sprintf("\x00CODEBLOCK%d\x00", idx)
		blocks[placeholder] = block
		result.WriteString(placeholder)
		idx++
		i += start + end + 3
	}
	return result.String(), blocks
}

func restoreBlocks(input string, blocks map[string]string) string {
	for placeholder, block := range blocks {
		input = strings.ReplaceAll(input, placeholder, block)
	}
	return input
}

// extractFrontmatter splits a leading `---` fenced frontmatter block off a
// document. The second return value repeats the raw block so callers can tell
// "no frontmatter" from "empty frontmatter"; the third is the remaining body.
func extractFrontmatter(input string) (string, string, string) {
	input = strings.TrimLeft(input, "\n\r\t ")
	if !strings.HasPrefix(input, "---") {
		return "", "", input
	}
	end := strings.Index(input[3:], "---")
	if end < 0 {
		return "", "", input
	}
	fm := input[3 : 3+end]
	rest := input[3+end+3:]
	return fm, fm, rest
}
