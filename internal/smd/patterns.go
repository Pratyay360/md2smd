// Package smd converts between Markdown and SuperMD, the Markdown dialect
// used by the Zine static site generator, where images, links, sections and
// blocks are expressed as Scripty directives embedded in link syntax.
package smd

import "regexp"

// Shared building blocks for the SuperMD directive expressions.
const (
	imageExprPat = `\$image\.(?:url|asset|siteAsset|buildAsset)\("[^"]*"\)(?:\.alt\("[^"]*"\))?`
	linkExprPat  = `\$link\.(?:url|ref|page|sub|sibling|site)(?:\("[^"]*"\))?(?:\.[a-zA-Z_]+\([^\)]*\))*`
)

// Asciinema terminal recordings are published as a badge image linking to the
// player page. Both shapes become the official player embed, which is the only
// form Zine accepts (validated =html code block).
const (
	asciinemaHost = `asciinema\.org/a/`
	asciinemaID   = `[A-Za-z0-9_-]+`
)

var (
	// Markdown shapes, matched before conversion to directives.
	linkedImageRe = regexp.MustCompile(`\[!\[([^\]]*)\]\(([^\s)]+)(?:\s+"([^"]*)")?\)\]\(([^\s)]+)(?:\s+"([^"]*)")?\)`)
	imageRe       = regexp.MustCompile(`!\[([^\]]*)\]\(([^\s)]+)(?:\s+"([^"]*)")?\)`)
	linkRe        = regexp.MustCompile(`\[([^\]]+)\]\(([^\s)]+)(?:\s+"([^"]*)")?\)`)

	// Any "[text](" prefix, the entry point for generic directive scanning.
	smdDirectiveRe = regexp.MustCompile(`\[([^\]]*)\]\(`)

	// SuperMD linked images: [[cap]($image...)]($link...)
	smdLinkedImageRe = regexp.MustCompile(`\[\[([^\]]*)\]\((` + imageExprPat + `)\)\]\((` + linkExprPat + `)\)`)
	// Legacy linked image whose outer URL is still a raw http(s) URL.
	smdLinkedImageLegacyRe = regexp.MustCompile(`\[\[([^\]]*)\]\((` + imageExprPat + `)\)\]\((https?://[^\s)]+)\)`)

	// =html linked image, the shape Zine requires for nested directives.
	htmlLinkedImageRe = regexp.MustCompile(`(?s)` + "```" + `=html\n\s*<a\s+href="([^"]*?)"([^>]*)>\s*<img\s+src="([^"]*?)"([^>]*?)>\s*</a>\s*\n` + "```")

	// [![asciicast](https://asciinema.org/a/ID.svg)](https://asciinema.org/a/ID)
	linkedAsciinemaRe = regexp.MustCompile(`\[!\[[^\]]*\]\(\s*https?://` + asciinemaHost + asciinemaID + `(?:\.svg)?\s*(?:"[^"]*")?\)\]\(\s*https?://` + asciinemaHost + asciinemaID + `\s*\)`)
	// ![asciicast](https://asciinema.org/a/ID.svg)
	asciinemaImageRe = regexp.MustCompile(`!\[[^\]]*\]\(\s*https?://` + asciinemaHost + `(` + asciinemaID + `)(?:\.svg)?\s*(?:"[^"]*")?\)`)
	// Reverse: the =html player embed back to the badge markdown form.
	htmlAsciinemaRe = regexp.MustCompile(`(?s)` + "```" + `=html\s*<script\s+src="https?://` + asciinemaHost + `(` + asciinemaID + `)\.js"[^>]*>\s*</script>\s*` + "```")
)

var (
	titleAttrRe = regexp.MustCompile(`title="([^"]*?)"`)
	altAttrRe   = regexp.MustCompile(`alt="([^"]*?)"`)
	castIDRe    = regexp.MustCompile(asciinemaHost + `(` + asciinemaID + `)`)
)
