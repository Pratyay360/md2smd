package smd

import (
	"fmt"
	"strings"

	"github.com/adrg/frontmatter"
	"gopkg.in/yaml.v3"
)

// convertAsciinema turns asciinema footage (badge image, optionally wrapped in
// a link to the player page) into the playable embed.
func convertAsciinema(input string) string {
	input = linkedAsciinemaRe.ReplaceAllStringFunc(input, func(match string) string {
		return asciinemaEmbed(extractCastID(match))
	})
	return asciinemaImageRe.ReplaceAllStringFunc(input, func(match string) string {
		loc := asciinemaImageRe.FindStringSubmatch(match)
		if loc == nil {
			return match
		}
		return asciinemaEmbed(loc[1])
	})
}

// asciinemaEmbed renders an asciinema cast as its official player embed.
// SuperMD forbids inline HTML, so the <script> tag has to travel inside a
// =html code block, which Zine validates and inlines into the page.
func asciinemaEmbed(castID string) string {
	return fmt.Sprintf("```=html\n<script src=\"https://asciinema.org/a/%s.js\" id=\"asciicast-%s\" async></script>\n```", castID, castID)
}

// extractCastID pulls the cast id out of any asciinema URL variant.
func extractCastID(url string) string {
	if loc := castIDRe.FindStringSubmatch(url); loc != nil {
		return loc[1]
	}
	return ""
}

// convertAsciinemaToMd restores the badge markdown from a player embed.
func convertAsciinemaToMd(input string) string {
	return htmlAsciinemaRe.ReplaceAllStringFunc(input, func(match string) string {
		loc := htmlAsciinemaRe.FindStringSubmatch(match)
		if loc == nil {
			return match
		}
		return fmt.Sprintf("[![asciicast](https://asciinema.org/a/%s.svg)](https://asciinema.org/a/%s)", loc[1], loc[1])
	})
}

// convertLinkedImage rewrites a Markdown linked image. Zine does not support
// nested directives like [[cap]($image...)]($link...), so the pair is emitted
// as an =html code block with a proper <a><img></a> structure instead.
func convertLinkedImage(match string) string {
	parts := linkedImageRe.FindStringSubmatch(match)
	if parts == nil {
		return match
	}
	caption := parts[1]
	imgURL := parts[2]
	imgAlt := parts[3]
	linkURL := parts[4]
	// parts[5] is the link title, ignored: SuperMD has no equivalent yet.
	//
	// The caption is the outer image alt text, so it must become <img alt="...">
	// to satisfy Zine's html validator.
	altText := caption
	if altText == "" {
		altText = imgAlt
	}
	if altText == "" {
		altText = "image"
	}
	altAttr := fmt.Sprintf(" alt=%q", altText)
	captionAttr := ""
	if caption != "" {
		captionAttr = fmt.Sprintf(" title=%q", caption)
	} else if imgAlt != "" {
		captionAttr = fmt.Sprintf(" title=%q", imgAlt)
	}
	return fmt.Sprintf("```=html\n<a href=%q%s><img src=%q%s></a>\n```", linkURL, captionAttr, imgURL, altAttr)
}

func convertImage(match string) string {
	parts := imageRe.FindStringSubmatch(match)
	if parts == nil {
		return match
	}
	caption := parts[1]
	altText := parts[3]
	directive := classifyImageURL(parts[2])
	if altText != "" {
		directive += fmt.Sprintf(".alt(%q)", altText)
	}
	return fmt.Sprintf("[%s](%s)", caption, directive)
}

func convertLink(match string) string {
	parts := linkRe.FindStringSubmatch(match)
	if parts == nil {
		return match
	}
	url := parts[2]
	// Leave text that already holds a Scripty expression untouched.
	if strings.HasPrefix(url, "$") {
		return match
	}
	return fmt.Sprintf("[%s](%s)", parts[1], classifyLinkURL(url))
}

// MdToSmd converts a Markdown document, frontmatter included, to SuperMD.
func MdToSmd(input string) (string, error) {
	matter, body, err := parseFrontmatter(input)
	if err != nil {
		return "", err
	}
	var smdFM string
	if len(matter) > 0 {
		smdFM = "---\n" + mapToZiggy(matter, "") + "---\n"
	}
	processed, blocks := extractFencedBlocks(body)
	processed = convertAsciinema(processed)
	processed = linkedImageRe.ReplaceAllStringFunc(processed, convertLinkedImage)
	processed = imageRe.ReplaceAllStringFunc(processed, convertImage)
	processed = linkRe.ReplaceAllStringFunc(processed, convertLink)
	processed = restoreBlocks(processed, blocks)
	return smdFM + processed, nil
}

// parseFrontmatter decodes the leading frontmatter block, falling back to
// lenient strategies when the YAML is not strictly valid.
func parseFrontmatter(input string) (map[string]interface{}, string, error) {
	var matter map[string]interface{}
	body, err := frontmatter.Parse(strings.NewReader(input), &matter)
	if err == nil {
		return matter, string(body), nil
	}

	fm, _, rest := extractFrontmatter(input)
	if fm == "" {
		return map[string]interface{}{}, input, nil
	}
	// Lenient fallback: body after the frontmatter is content even if the YAML
	// is malformed (e.g. an unquoted colon in "How to Use X: A Guide").
	var parsed map[string]interface{}
	if yamlErr := yaml.Unmarshal([]byte(fm), &parsed); yamlErr == nil && len(parsed) > 0 {
		return parsed, rest, nil
	}
	// Last resort: split each line on its first ':' to salvage scalar fields.
	parsed = make(map[string]interface{})
	for _, line := range strings.Split(fm, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		sep := strings.Index(line, ":")
		if sep < 0 {
			continue
		}
		k := strings.TrimSpace(line[:sep])
		v := strings.Trim(strings.TrimSpace(line[sep+1:]), `"'`)
		if k != "" && v != "" {
			parsed[k] = v
		}
	}
	return parsed, rest, nil
}
