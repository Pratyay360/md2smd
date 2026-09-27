package smd

import (
	"fmt"
	"strings"
)

// SmdToMd converts a SuperMD document, frontmatter included, to Markdown.
func SmdToMd(input string) (string, error) {
	fm, _, rest := extractFrontmatter(input)
	var mdFM string
	if fm != "" {
		var err error
		if mdFM, err = ziggyToYaml(fm); err != nil {
			return "", err
		}
	}
	processed, blocks := extractFencedBlocks(rest)
	processed = smdToMdConvert(processed)
	processed = restoreBlocks(processed, blocks)
	processed = htmlLinkedImageRe.ReplaceAllStringFunc(processed, convertHtmlLinkedImage)
	processed = convertAsciinemaToMd(processed)
	if mdFM != "" {
		mdFM = "---\n" + mdFM + "---\n"
	}
	return mdFM + processed, nil
}

// smdToMdConvert rewrites SuperMD directives back into plain Markdown. Linked
// images are handled first, before the generic single-directive scan.
func smdToMdConvert(input string) string {
	// Handle legacy linked images even when they contain $ only in the image
	// part: [[cap]($image...)](https://...) -> [![cap](img)](https://...)
	input = smdLinkedImageLegacyRe.ReplaceAllStringFunc(input, convertSmdLegacyLinkedImage)
	if !strings.Contains(input, "(") || !strings.Contains(input, "$") {
		return input
	}
	// [[caption]($image...)]($link...) -> [![caption](url)](url)
	input = smdLinkedImageRe.ReplaceAllStringFunc(input, convertSmdLinkedImage)
	if !strings.Contains(input, "(") || !strings.Contains(input, "$") {
		return input
	}
	var result strings.Builder
	for i := 0; i < len(input); {
		loc := smdDirectiveRe.FindStringSubmatchIndex(input[i:])
		if loc == nil {
			result.WriteString(input[i:])
			break
		}
		result.WriteString(input[i : i+loc[0]])
		text := input[i+loc[2] : i+loc[3]]
		parenStart := i + loc[1] - 1
		closeParen := findMatchingParen(input, parenStart)
		if closeParen < 0 {
			result.WriteString(input[i+loc[0] : i+loc[1]])
			i += loc[1]
			continue
		}
		dirName, calls := parseDirectiveCalls(input[parenStart+1 : closeParen])
		converted := ""
		if dirName != "" && len(calls) > 0 {
			converted = convertSmdExpression(text, dirName, calls)
		}
		if converted == "" {
			result.WriteString(input[i+loc[0] : i+loc[1]])
			i += loc[1]
			continue
		}
		result.WriteString(converted)
		i = closeParen + 1
	}
	return result.String()
}

func convertSmdExpression(text string, dirName string, calls []directiveCall) string {
	switch dirName {
	case "image":
		return convertSmdImageExpr(text, calls)
	case "link":
		return convertSmdLinkExpr(text, calls)
	}
	return ""
}

func convertSmdImageExpr(caption string, calls []directiveCall) string {
	url, altText := imageExprArgs(calls)
	if url == "" {
		return ""
	}
	switch {
	case altText != "":
		return fmt.Sprintf("![%s](%s %q)", caption, url, altText)
	case caption != "":
		return fmt.Sprintf("![%s](%s)", caption, url)
	default:
		return fmt.Sprintf("![](%s)", url)
	}
}

func convertSmdLinkExpr(text string, calls []directiveCall) string {
	url := linkExprURL(calls)
	if url == "" {
		return ""
	}
	return fmt.Sprintf("[%s](%s)", text, url)
}

// imageExprArgs extracts the target URL and alt text from $image calls.
func imageExprArgs(calls []directiveCall) (url, alt string) {
	for _, c := range calls {
		switch c.function {
		case "url", "asset", "siteAsset", "buildAsset":
			if len(c.args) > 0 {
				url = unquote(c.args[0])
			}
		case "alt":
			if len(c.args) > 0 {
				alt = unquote(c.args[0])
			}
		}
	}
	return url, alt
}

// linkExprURL maps $link calls back onto a plain Markdown target. $link.site()
// has no path, so it resolves to the site root.
func linkExprURL(calls []directiveCall) string {
	url := ""
	for _, c := range calls {
		if c.function == "site" {
			url = "/"
			continue
		}
		if len(c.args) == 0 {
			continue
		}
		switch c.function {
		case "url":
			url = unquote(c.args[0])
		case "ref":
			url = "#" + unquote(c.args[0])
		case "page":
			url = "/" + unquote(c.args[0])
		case "sub":
			url = "./" + unquote(c.args[0])
		case "sibling":
			url = unquote(c.args[0])
		}
	}
	return url
}

func convertSmdLinkedImage(match string) string {
	parts := smdLinkedImageRe.FindStringSubmatch(match)
	if parts == nil {
		return match
	}
	caption := parts[1]
	imgURL, imgAlt := imageExprArgs(parseCalls(parts[2]))
	linkURL := linkExprURL(parseCalls(parts[3]))
	if imgURL == "" || linkURL == "" {
		return match
	}
	if imgAlt != "" {
		return fmt.Sprintf("[![%s](%s %q)](%s)", caption, imgURL, imgAlt, linkURL)
	}
	return fmt.Sprintf("[![%s](%s)](%s)", caption, imgURL, linkURL)
}

func convertSmdLegacyLinkedImage(match string) string {
	parts := smdLinkedImageLegacyRe.FindStringSubmatch(match)
	if parts == nil {
		return match
	}
	caption := parts[1]
	rawURL := parts[3]
	imgURL, imgAlt := imageExprArgs(parseCalls(parts[2]))
	if imgURL == "" {
		return match
	}
	if imgAlt != "" {
		return fmt.Sprintf("[![%s](%s %q)](%s)", caption, imgURL, imgAlt, rawURL)
	}
	return fmt.Sprintf("[![%s](%s)](%s)", caption, imgURL, rawURL)
}

func convertHtmlLinkedImage(match string) string {
	parts := htmlLinkedImageRe.FindStringSubmatch(match)
	if parts == nil {
		return match
	}
	linkURL := parts[1]
	imgURL := parts[3]
	// parts[2] holds extra <a> attributes and parts[4] extra <img> attributes.
	title := ""
	if m := titleAttrRe.FindStringSubmatch(parts[2]); m != nil {
		title = m[1]
	}
	alt := ""
	if m := altAttrRe.FindStringSubmatch(parts[4]); m != nil {
		alt = m[1]
	}
	switch {
	case alt != "":
		return fmt.Sprintf("[![%s](%s %q)](%s)", title, imgURL, alt, linkURL)
	case title != "":
		return fmt.Sprintf("[![%s](%s)](%s)", title, imgURL, linkURL)
	default:
		return fmt.Sprintf("![](%s)", imgURL)
	}
}

func parseCalls(expr string) []directiveCall {
	_, calls := parseDirectiveCalls(expr)
	return calls
}
