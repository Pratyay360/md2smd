package utils
import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/adrg/frontmatter"
	"gopkg.in/yaml.v3"
)
var (
	linkedImageRe     = regexp.MustCompile(`\[!\[([^\]]*)\]\(([^\s)]+)(?:\s+"([^"]*)")?\)\]\(([^\s)]+)(?:\s+"([^"]*)")?\)`)
	imageRe           = regexp.MustCompile(`!\[([^\]]*)\]\(([^\s)]+)(?:\s+"([^"]*)")?\)`)
	linkRe            = regexp.MustCompile(`\[([^\]]+)\]\(([^\s)]+)(?:\s+"([^"]*)")?\)`)
	smdDirectiveRe    = regexp.MustCompile(`\[([^\]]*)\]\(`)
	htmlCommentRe     = regexp.MustCompile(`(?s)<!--.*?-->`)
	headingRe         = regexp.MustCompile(`(?m)^(\s{0,3})(#{1,6})(\s)`)
	mdxImportRe       = regexp.MustCompile(`(?m)^\s*(import|export)\s+.*$`)
	htmlTagRe         = regexp.MustCompile(`<[^>]+>`)
	htmlLineRe        = regexp.MustCompile(`(?m)^\s*<[^>]+>\s*$`)
	hrRe              = regexp.MustCompile(`(?m)^\s*([-*_][ \t]*){3,}\s*$`)
	longDashFMRe      = regexp.MustCompile(`\A\s*-{5,}\s*\n([\s\S]*?)\n\s*-{5,}\s*\n`)
	imageExprPat      = `\$image\.(?:url|asset|siteAsset|buildAsset)\("[^"]*"\)(?:\.alt\("[^"]*"\))?`
	linkExprPat       = `\$link\.(?:url|ref|page|sub|sibling|site)(?:\("[^"]*"\))?(?:\.[a-zA-Z_]+\([^\)]*\))*`
	smdLinkedImageRe  = regexp.MustCompile(`\[\[([^\]]*)\]\((` + `\$image\.(?:url|asset|siteAsset|buildAsset)\("[^"]*"\)(?:\.alt\("[^"]*"\))?` + `)\)\]\((` + `\$link\.(?:url|ref|page|sub|sibling|site)(?:\("[^"]*"\))?(?:\.[a-zA-Z_]+\([^\)]*\))*` + `)\)`)
	// Legacy broken linked image where outer URL is still raw http(s)://... after a previous buggy conversion
	smdLinkedImageLegacyRe = regexp.MustCompile(`\[\[([^\]]*)\]\((` + `\$image\.(?:url|asset|siteAsset|buildAsset)\("[^"]*"\)(?:\.alt\("[^"]*"\))?` + `)\)\]\((https?://[^\s)]+)\)`)
	// =html linked image: ```=html\n<a href="..."...><img src="..."...></a>\n```
	htmlLinkedImageRe = regexp.MustCompile(`(?s)` + "```" + `=html\n\s*<a\s+href="([^"]*?)"([^>]*)>\s*<img\s+src="([^"]*?)"([^>]*?)>\s*</a>\s*\n` + "```")
)
func mapToZiggy(data map[string]interface{}, prefix string) string {
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var lines []string
	for _, k := range keys {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		lines = append(lines, ziggyValue(key, data[k], 0))
	}
	return strings.Join(lines, "")
}
func ziggyValue(key string, v interface{}, depth int) string {
	indent := strings.Repeat("\t", depth)
	switch val := v.(type) {
	case string:
		if isDateString(val) {
			return fmt.Sprintf("%s.%s = .date(%q),\n", indent, key, val)
		}
		return fmt.Sprintf("%s.%s = %q,\n", indent, key, val)
	case int:
		return fmt.Sprintf("%s.%s = %d,\n", indent, key, val)
	case float64:
		return fmt.Sprintf("%s.%s = %v,\n", indent, key, val)
	case bool:
		return fmt.Sprintf("%s.%s = %v,\n", indent, key, val)
	case time.Time:
		return fmt.Sprintf("%s.%s = .date(%q),\n", indent, key, val.Format("2006-01-02T15:04:05"))
	case []interface{}:
		var elems []string
		for _, e := range val {
			elems = append(elems, formatZiggyValueInline(e))
		}
		joined := strings.Join(elems, ", ")
		if strings.Contains(joined, "\n") {
			joined = "\n" + joined + "\n"
		}
		return fmt.Sprintf("%s.%s = [%s],\n", indent, key, joined)
	case map[string]interface{}:
		nestedKeys := make([]string, 0, len(val))
		for sk := range val {
			nestedKeys = append(nestedKeys, sk)
		}
		sort.Strings(nestedKeys)
		var lines []string
		for _, sk := range nestedKeys {
			lines = append(lines, ziggyValue(key+"."+sk, val[sk], depth))
		}
		return strings.Join(lines, "")
	case map[interface{}]interface{}:
		type kv struct {
			k   interface{}
			str string
		}
		var kvs []kv
		for k := range val {
			kvs = append(kvs, kv{k, fmt.Sprintf("%v", k)})
		}
		sort.Slice(kvs, func(i, j int) bool { return kvs[i].str < kvs[j].str })
		var lines []string
		for _, kv := range kvs {
			lines = append(lines, ziggyValue(key+"."+kv.str, val[kv.k], depth))
		}
		return strings.Join(lines, "")
	default:
		return fmt.Sprintf("%s.%s = %v,\n", indent, key, v)
	}
}
func formatZiggyValueInline(v interface{}) string {
	switch val := v.(type) {
	case string:
		return fmt.Sprintf("%q", val)
	case int:
		return fmt.Sprintf("%d", val)
	case float64:
		return fmt.Sprintf("%v", val)
	case bool:
		return fmt.Sprintf("%v", val)
	case nil:
		return "null"
	case time.Time:
		return fmt.Sprintf(".date(%q)", val.Format("2006-01-02T15:04:05"))
	case []interface{}:
		var elems []string
		for _, e := range val {
			elems = append(elems, formatZiggyValueInline(e))
		}
		return "[" + strings.Join(elems, ", ") + "]"
	case map[string]interface{}:
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var fields []string
		for _, k := range keys {
			fields = append(fields, fmt.Sprintf(".%s = %s", k, formatZiggyValueInline(val[k])))
		}
		return "{" + strings.Join(fields, ", ") + "}"
	case map[interface{}]interface{}:
		type kv2 struct {
			k   interface{}
			str string
		}
		var kvs []kv2
		for k := range val {
			kvs = append(kvs, kv2{k, fmt.Sprintf("%v", k)})
		}
		sort.Slice(kvs, func(i, j int) bool { return kvs[i].str < kvs[j].str })
		var fields []string
		for _, kv := range kvs {
			fields = append(fields, fmt.Sprintf(".%s = %s", kv.str, formatZiggyValueInline(val[kv.k])))
		}
		return "{" + strings.Join(fields, ", ") + "}"
	default:
		return fmt.Sprintf("%v", val)
	}
}

func ziggyToYaml(ziggyStr string) (string, error) {
	if strings.TrimSpace(ziggyStr) == "" {
		return "", nil
	}
	data := ziggyToMap(ziggyStr)
	if data == nil {
		return ziggyStr, nil
	}
	out, err := yaml.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("failed to marshal YAML frontmatter: %w", err)
	}
	return string(out), nil
}
func ziggyToMap(ziggy string) map[string]interface{} {
	data := make(map[string]interface{})
	for _, line := range strings.Split(ziggy, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		line = strings.TrimSuffix(line, ",")
		line = strings.TrimSuffix(line, ";")
		line = strings.TrimPrefix(line, ".") 
		if line == "" {
			continue
		}
		parts := splitZiggyKeyValue(line)
		if parts == nil {
			continue
		}
		keys := strings.Split(parts[0], ".") 
		setNestedValue(data, keys, parseZiggyValue(parts[1]))
	}
	return data
}
func parseZiggyValue(value string) interface{} {
	value = strings.TrimSpace(value)
	if len(value) == 0 {
		return ""
	}
	if (strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`)) ||
		(strings.HasPrefix(value, `'`) && strings.HasSuffix(value, `'`)) {
		return value[1 : len(value)-1]
	}
	if value == "true" {
		return true
	}
	if value == "false" {
		return false
	}
	if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
		inner := strings.TrimSpace(value[1 : len(value)-1])
		if inner == "" {
			return []interface{}{}
		}
		var items []interface{}
		// BUG FIX: Use splitScriptyArgs to respect commas inside quoted strings
		for _, item := range splitScriptyArgs(inner) {
			items = append(items, parseZiggyValue(item))
		}
		return items
	}
	if strings.HasPrefix(value, "{") && strings.HasSuffix(value, "}") {
		inner := strings.TrimSpace(value[1 : len(value)-1])
		if inner == "" {
			return map[string]interface{}{}
		}
		result := make(map[string]interface{})
		for _, field := range splitScriptyArgs(inner) {
			field = strings.TrimSpace(field)
			eqIdx := strings.Index(field, "=")
			if eqIdx < 0 {
				continue
			}
			fk := strings.TrimSpace(field[:eqIdx])
			fv := strings.TrimSpace(field[eqIdx+1:])
			fk = strings.TrimPrefix(fk, ".")
			result[fk] = parseZiggyValue(fv)
		}
		return result
	}
	if idx := strings.Index(value, "("); idx >= 0 && strings.HasSuffix(value, ")") {
		inner := strings.TrimSpace(value[idx+1 : len(value)-1])
		inner = strings.Trim(inner, "\"'")
		if inner != "" {
			return inner
		}
	}
	if strings.Contains(value, "(") {
		return value
	}
	if strings.Contains(value, ".") {
		var f float64
		if _, err := fmt.Sscanf(value, "%f", &f); err == nil {
			return f
		}
	}
	var i int
	if _, err := fmt.Sscanf(value, "%d", &i); err == nil {
		return i
	}
	return value
}
func splitZiggyKeyValue(line string) []string {
	idx := strings.Index(line, "=")
	if idx < 0 {
		return nil
	}
	key := strings.TrimSpace(line[:idx])
	value := strings.TrimSpace(line[idx+1:])
	return []string{key, value}
}
func setNestedValue(data map[string]interface{}, keys []string, value interface{}) {
	if len(keys) == 1 {
		data[keys[0]] = value
		return
	}
	sub, ok := data[keys[0]]
	if !ok {
		sub = make(map[string]interface{})
		data[keys[0]] = sub
	}
	if m, ok := sub.(map[string]interface{}); ok {
		setNestedValue(m, keys[1:], value)
	}
}
func isDateString(s string) bool {
	if len(s) < 10 {
		return false
	}
	if s[4] != '-' || s[7] != '-' {
		return false
	}
	for i := 0; i < 10; i++ {
		if i == 4 || i == 7 {
			continue
		}
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	if len(s) == 10 {
		return true
	}
	rest := s[10:]
	if rest[0] == 'T' || rest[0] == ' ' {
		return true
	}
	return false
}
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
		// Include fence lines completely: from opening fence to after closing fence
		// Need to handle language specifier on opening fence line (e.g., ```js or ~~~python)
		// Our simple search includes that as part of block content
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
func stripHtmlComments(input string) string {
	return htmlCommentRe.ReplaceAllString(input, "")
}
func stripMdxImports(input string) string {
	return mdxImportRe.ReplaceAllString(input, "")
}
func normalizeThematicBreaks(input string) string {
	// SuperMD via cmark-gfm is strict about thematic breaks: Zine reports
	// "unexpected token" for ---- / ----- etc. Normalize any HR to exactly "---"
	// while preserving code fences (already extracted).
	return hrRe.ReplaceAllString(input, "---")
}
func sanitizeZiggyValue(v interface{}) interface{} {
	// Zine's Page schema only allows specific fields; unknown top-level keys
	// from generic markdown frontmatter (e.g., cover, summary, content_meta)
	// would be ignored or cause errors. Keep only allowed keys + custom fields
	// that are safe. For super compatibility, we keep all but ensure they are
	// serializable; Zine will ignore unknown via custom? To be safe, filter.
	// For now, keep all – Zine allows extra via ? fields? But we ensure
	// required fields exist.
	return v
}
func handleInlineHtml(input string) string {
	// SuperMD forbids inline HTML. To support any MD/MDX flavour (including JSX),
	// we need to make the output valid. Two strategies:
	// - Standalone HTML/JSX block lines (e.g., <div> or <MyComponent />) -> wrap in =html code block (SuperMD escape hatch)
	// - Inline HTML mixed with markdown text -> escape to &lt;/&gt;
	// We handle multi-line HTML blocks by collecting consecutive HTML-tag lines
	// and wrapping the whole block in a single =html fence to preserve structure.
	lines := strings.Split(input, "\n")
	var out []string
	i := 0
	for i < len(lines) {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			out = append(out, line)
			i++
			continue
		}
		if htmlLineRe.MatchString(line) {
			// Collect consecutive HTML-only lines (including blank lines between) as one block
			// to handle cases like <div>\n  content\n</div> where inner content is not pure HTML
			// but part of the block. Heuristic: gather until we find a line that is not HTML-only
			// and not empty, or until closing tag.
			blockLines := []string{trimmed}
			j := i + 1
			// If opening tag without closing on same line, collect until closing tag
			isOpeningBlock := !strings.HasPrefix(trimmed, "</") && !strings.HasSuffix(trimmed, "/>") && strings.HasPrefix(trimmed, "<")
			if isOpeningBlock {
				for j < len(lines) {
					nextTrimmed := strings.TrimSpace(lines[j])
					if nextTrimmed == "" {
						blockLines = append(blockLines, lines[j])
						j++
						continue
					}
					if htmlLineRe.MatchString(lines[j]) || nextTrimmed == "" {
						blockLines = append(blockLines, nextTrimmed)
						j++
						// Stop at closing tag
						if strings.HasPrefix(nextTrimmed, "</") {
							break
						}
						continue
					}
					// Content inside block (e.g., "Block html") - include in block
					if j == i+1 {
						// Include one content line if directly after opening
						blockLines = append(blockLines, lines[j])
						j++
						// Check if next is closing
						if j < len(lines) && htmlLineRe.MatchString(lines[j]) {
							blockLines = append(blockLines, strings.TrimSpace(lines[j]))
							j++
						}
					}
					break
				}
				// If we collected more than one line, wrap as single =html block
				if len(blockLines) > 1 {
					out = append(out, "```=html")
					out = append(out, blockLines...)
					out = append(out, "```")
					i = j
					continue
				}
			}
			// Single HTML line -> wrap individually
			out = append(out, "```=html")
			out = append(out, trimmed)
			out = append(out, "```")
			i++
			continue
		}
		if htmlTagRe.MatchString(line) {
			line = htmlTagRe.ReplaceAllStringFunc(line, func(m string) string {
				return strings.ReplaceAll(strings.ReplaceAll(m, "<", "&lt;"), ">", "&gt;")
			})
		}
		out = append(out, line)
		i++
	}
	return strings.Join(out, "\n")
}
func normalizeHeadings(input string) string {
	matches := headingRe.FindAllStringSubmatch(input, -1)
	if len(matches) == 0 {
		return input
	}
	minLevel := 7
	for _, m := range matches {
		level := len(m[2])
		if level < minLevel {
			minLevel = level
		}
	}
	offset := 0
	if minLevel > 1 {
		offset = minLevel - 1
	}
	lines := strings.Split(input, "\n")
	prevLevel := 0
	for i, line := range lines {
		loc := headingRe.FindStringSubmatchIndex(line)
		if loc == nil {
			continue
		}
		hashes := line[loc[4]:loc[5]]
		level := len(hashes)
		newLevel := level - offset
		if newLevel < 1 {
			newLevel = 1
		}
		if newLevel > 6 {
			newLevel = 6
		}
		if prevLevel != 0 && newLevel > prevLevel+1 {
			newLevel = prevLevel + 1
		}
		if newLevel != level {
			indent := line[loc[2]:loc[3]]
			rest := line[loc[5]:]
			lines[i] = indent + strings.Repeat("#", newLevel) + rest
		}
		prevLevel = newLevel
	}
	return strings.Join(lines, "\n")
}
func classifyImageURL(url string) string {
	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
		return fmt.Sprintf("$image.url(%q)", url)
	}
	if strings.HasPrefix(url, "/") {
		return fmt.Sprintf("$image.siteAsset(%q)", url)
	}
	return fmt.Sprintf("$image.asset(%q)", url)
}
func classifyLinkURL(url string) string {
	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
		return fmt.Sprintf("$link.url(%q).new(true)", url)
	}
	if strings.HasPrefix(url, "#") {
		return fmt.Sprintf("$link.ref(%q)", url[1:])
	}
	if strings.HasPrefix(url, "/") {
		return fmt.Sprintf("$link.page(%q)", strings.TrimPrefix(url, "/"))
	}
	if strings.HasPrefix(url, "./") {
		return fmt.Sprintf("$link.sub(%q)", url[2:])
	}
	return fmt.Sprintf("$link.sibling(%q)", url)
}
func convertLinkedImage(match string) string {
	parts := linkedImageRe.FindStringSubmatch(match)
	if parts == nil {
		return match
	}
	caption := parts[1]
	imgURL := parts[2]
	imgAlt := parts[3]
	linkURL := parts[4]
	// linkTitle parts[5] ignored, no Smd equivalent for title on links yet
	//
	// Zine does not support nested directives like [[cap]($image...)]($link...).
	// Instead, emit an =html code block with a proper <a><img></a> structure.
	altAttr := ""
	if imgAlt != "" {
		altAttr = fmt.Sprintf(" alt=%q", imgAlt)
	}
	captionAttr := ""
	if caption != "" {
		captionAttr = fmt.Sprintf(" title=%q", caption)
	}
	return fmt.Sprintf("```=html\n<a href=%q%s><img src=%q%s></a>\n```", linkURL, captionAttr, imgURL, altAttr)
}

func convertImage(match string) string {
	parts := imageRe.FindStringSubmatch(match)
	if parts == nil {
		return match
	}
	caption := parts[1]
	url := parts[2]
	altText := parts[3]
	directive := classifyImageURL(url)
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
	text := parts[1]
	url := parts[2]
	if strings.HasPrefix(url, "$") {
		return match
	}
	directive := classifyLinkURL(url)
	return fmt.Sprintf("[%s](%s)", text, directive)
}

func convertSmdLinkedImage(match string) string {
	parts := smdLinkedImageRe.FindStringSubmatch(match)
	if parts == nil {
		return match
	}
	caption := parts[1]
	imgExpr := parts[2]
	linkExpr := parts[3]
	// Parse image URL from $image.* calls
	_, imgCalls := parseDirectiveCalls(imgExpr)
	imgURL := ""
	imgAlt := ""
	for _, c := range imgCalls {
		switch c.function {
		case "url", "asset", "siteAsset", "buildAsset":
			if len(c.args) > 0 {
				imgURL = unquote(c.args[0])
			}
		case "alt":
			if len(c.args) > 0 {
				imgAlt = unquote(c.args[0])
			}
		}
	}
	_, linkCalls := parseDirectiveCalls(linkExpr)
	linkURL := ""
	for _, c := range linkCalls {
		switch c.function {
		case "url":
			if len(c.args) > 0 {
				linkURL = unquote(c.args[0])
			}
		case "ref":
			if len(c.args) > 0 {
				linkURL = "#" + unquote(c.args[0])
			}
		case "page":
			if len(c.args) > 0 {
				linkURL = "/" + unquote(c.args[0])
			}
		case "sub":
			if len(c.args) > 0 {
				linkURL = "./" + unquote(c.args[0])
			}
		case "sibling":
			if len(c.args) > 0 {
				linkURL = unquote(c.args[0])
			}
		case "site":
			linkURL = "/"
		}
	}
	if imgURL == "" || linkURL == "" {
		return match
	}
	if imgAlt != "" {
		return fmt.Sprintf("[![%s](%s %q)](%s)", caption, imgURL, imgAlt, linkURL)
	}
	return fmt.Sprintf("[![%s](%s)](%s)", caption, imgURL, linkURL)
}

func fixLegacyLinkedImage(match string) string {
	parts := smdLinkedImageLegacyRe.FindStringSubmatch(match)
	if parts == nil {
		return match
	}
	caption := parts[1]
	imgExpr := parts[2]
	rawURL := parts[3]
	linkDirective := classifyLinkURL(rawURL)
	return fmt.Sprintf("[[%s](%s)](%s)", caption, imgExpr, linkDirective)
}

func convertHtmlLinkedImage(match string) string {
	parts := htmlLinkedImageRe.FindStringSubmatch(match)
	if parts == nil {
		return match
	}
	linkURL := parts[1]
	// parts[2] = extra attributes on <a> (e.g., title)
	imgURL := parts[3]
	// parts[4] = extra attributes on <img> (e.g., alt)
	// Extract title from <a> tag
	title := ""
	aAttrs := parts[2]
	if titleMatch := regexp.MustCompile(`title="([^"]*?)"`).FindStringSubmatch(aAttrs); titleMatch != nil {
		title = titleMatch[1]
	}
	// Extract alt from <img> tag
	alt := ""
	imgAttrs := parts[4]
	if altMatch := regexp.MustCompile(`alt="([^"]*?)"`).FindStringSubmatch(imgAttrs); altMatch != nil {
		alt = altMatch[1]
	}
	if alt != "" {
		return fmt.Sprintf("[![%s](%s %q)](%s)", title, imgURL, alt, linkURL)
	}
	if title != "" {
		return fmt.Sprintf("[![%s](%s)](%s)", title, imgURL, linkURL)
	}
	return fmt.Sprintf("![](%s)", imgURL)
}

func convertSmdLegacyLinkedImage(match string) string {
	parts := smdLinkedImageLegacyRe.FindStringSubmatch(match)
	if parts == nil {
		return match
	}
	caption := parts[1]
	imgExpr := parts[2]
	rawURL := parts[3]
	_, imgCalls := parseDirectiveCalls(imgExpr)
	imgURL := ""
	imgAlt := ""
	for _, c := range imgCalls {
		switch c.function {
		case "url", "asset", "siteAsset", "buildAsset":
			if len(c.args) > 0 {
				imgURL = unquote(c.args[0])
			}
		case "alt":
			if len(c.args) > 0 {
				imgAlt = unquote(c.args[0])
			}
		}
	}
	if imgURL == "" {
		return match
	}
	if imgAlt != "" {
		return fmt.Sprintf("[![%s](%s %q)](%s)", caption, imgURL, imgAlt, rawURL)
	}
	return fmt.Sprintf("[![%s](%s)](%s)", caption, imgURL, rawURL)
}
func findMatchingParen(s string, start int) int {
	if start >= len(s) || s[start] != '(' {
		return -1
	}
	depth := 0
	inString := false
	strChar := byte(0)
	for i := start; i < len(s); i++ {
		ch := s[i]
		if inString {
			if ch == strChar {
				inString = false
			}
			continue
		}
		switch ch {
		case '"', '\'':
			inString = true
			strChar = ch
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}
type directiveCall struct {
	function string
	args     []string
}
func parseDirectiveCalls(expr string) (string, []directiveCall) {
	expr = strings.TrimSpace(expr)
	if !strings.HasPrefix(expr, "$") {
		return "", nil
	}
	dotIdx := strings.Index(expr, ".")
	if dotIdx < 0 {
		return expr[1:], nil
	}
	dirName := expr[1:dotIdx]
	rest := expr[dotIdx:]
	var calls []directiveCall
	i := 0
	for i < len(rest) {
		if rest[i] != '.' {
			break
		}
		i++
		funcStart := i
		for i < len(rest) && rest[i] != '(' {
			i++
		}
		funcName := rest[funcStart:i]
		if i < len(rest) && rest[i] == '(' {
			closeParen := findMatchingParen(rest, i)
			if closeParen < 0 {
				break
			}
			argsStr := rest[i+1 : closeParen]
			var args []string
			if strings.TrimSpace(argsStr) != "" {
				args = splitScriptyArgs(argsStr)
			}
			calls = append(calls, directiveCall{function: funcName, args: args})
			i = closeParen + 1
		}
	}
	return dirName, calls
}
func splitScriptyArgs(argsStr string) []string {
	var args []string
	var current strings.Builder
	inString := false
	strChar := byte(0)
	parenDepth := 0
	bracketDepth := 0
	braceDepth := 0
	for i := 0; i < len(argsStr); i++ {
		ch := argsStr[i]
		if inString {
			current.WriteByte(ch)
			if ch == strChar {
				inString = false
			}
			continue
		}
		switch ch {
		case '"', '\'':
			inString = true
			strChar = ch
			current.WriteByte(ch)
		case '(':
			parenDepth++
			current.WriteByte(ch)
		case ')':
			parenDepth--
			current.WriteByte(ch)
		case '[':
			bracketDepth++
			current.WriteByte(ch)
		case ']':
			bracketDepth--
			current.WriteByte(ch)
		case '{':
			braceDepth++
			current.WriteByte(ch)
		case '}':
			braceDepth--
			current.WriteByte(ch)
		case ',':
			if parenDepth == 0 && bracketDepth == 0 && braceDepth == 0 {
				args = append(args, strings.TrimSpace(current.String()))
				current.Reset()
			} else {
				current.WriteByte(ch)
			}
		default:
			current.WriteByte(ch)
		}
	}
	if current.Len() > 0 {
		args = append(args, strings.TrimSpace(current.String()))
	}
	return args
}
func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && ((s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'')) {
		return s[1 : len(s)-1]
	}
	return s
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
	url := ""
	altText := ""
	for _, c := range calls {
		switch c.function {
		case "url":
			if len(c.args) > 0 {
				url = unquote(c.args[0])
			}
		case "asset":
			if len(c.args) > 0 {
				url = unquote(c.args[0])
			}
		case "siteAsset":
			if len(c.args) > 0 {
				url = unquote(c.args[0])
			}
		case "buildAsset":
			if len(c.args) > 0 {
				url = unquote(c.args[0])
			}
		case "alt":
			if len(c.args) > 0 {
				altText = unquote(c.args[0])
			}
		}
	}
	if url == "" {
		return ""
	}
	if altText != "" {
		return fmt.Sprintf("![%s](%s %q)", caption, url, altText)
	}
	if caption != "" {
		return fmt.Sprintf("![%s](%s)", caption, url)
	}
	return fmt.Sprintf("![](%s)", url)
}
func convertSmdLinkExpr(text string, calls []directiveCall) string {
	url := ""
	for _, c := range calls {
		switch c.function {
		case "url":
			if len(c.args) > 0 {
				url = unquote(c.args[0])
			}
		case "ref":
			if len(c.args) > 0 {
				url = "#" + unquote(c.args[0])
			}
		case "page":
			if len(c.args) > 0 {
				url = "/" + unquote(c.args[0])
			}
		case "sub":
			if len(c.args) > 0 {
				url = "./" + unquote(c.args[0])
			}
		case "sibling":
			if len(c.args) > 0 {
				url = unquote(c.args[0])
			}
		case "site":
			url = "/"
		}
	}
	if url == "" {
		return ""
	}
	return fmt.Sprintf("[%s](%s)", text, url)
}
func smdToMdConvert(input string) string {
	// Handle legacy broken linked images even if they contain $ only in image part
	// Legacy: [[cap]($image...)](https://...) -> [![cap](img)](https://...)
	input = smdLinkedImageLegacyRe.ReplaceAllStringFunc(input, convertSmdLegacyLinkedImage)
	if !strings.Contains(input, "(") || !strings.Contains(input, "$") {
		return input
	}
	// Handle linked images first: [[caption]($image...)]($link...) -> [![caption](url)](url)
	// This must run before the generic single-directive conversion.
	input = smdLinkedImageRe.ReplaceAllStringFunc(input, convertSmdLinkedImage)
	if !strings.Contains(input, "(") || !strings.Contains(input, "$") {
		return input
	}
	var result strings.Builder
	i := 0
	for i < len(input) {
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
		expr := input[parenStart+1 : closeParen]
		dirName, calls := parseDirectiveCalls(expr)
		if dirName == "" || len(calls) == 0 {
			result.WriteString(input[i+loc[0] : i+loc[1]])
			i += loc[1]
			continue
		}
		converted := convertSmdExpression(text, dirName, calls)
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
func inferTitleFromBody(body string) string {
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Match any heading level (#..######) as title fallback
		loc := headingRe.FindStringSubmatchIndex(line)
		if loc != nil {
			title := strings.TrimSpace(line[loc[5]:])
			title = strings.Trim(title, "*_` ")
			if title != "" {
				return title
			}
		}
		// Also handle setext? not needed
	}
	return ""
}

func sanitizeMatter(matter map[string]interface{}, body string) map[string]interface{} {
	if matter == nil {
		matter = make(map[string]interface{})
	}
	// Whitelist of allowed top-level Page fields in Zine's .smd.ziggy-schema
	allowed := map[string]bool{
		"title":               true,
		"description":         true,
		"date":                true,
		"authors":             true,
		"author":              true, // alias, will be normalized to authors
		"tags":                true,
		"layout":              true,
		"aliases":             true,
		"alternatives":        true,
		"translation_key":     true,
		"draft":               true,
		"forbid_subsections":  true,
		"custom":              true,
	}
	// Normalize author -> authors
	if v, ok := matter["author"]; ok {
		if _, has := matter["authors"]; !has {
			matter["authors"] = v
		}
		delete(matter, "author")
	}
	// For super compatibility, drop unknown top-level fields that are not in
	// Zine's Page schema (e.g., cover, summary, image, content_meta).
	// Previously we tried to move them into `custom`, but nested custom
	// structures produce invalid Ziggy (` .custom.content_meta.trending`)
	// resulting in "missing token" errors. Dropping is safest for builds.
	for k := range matter {
		if !allowed[k] {
			delete(matter, k)
		}
	}
	// Ensure custom, if present, is a simple map[string]interface{} or drop it
	if c, ok := matter["custom"]; ok {
		if _, ok := c.(map[string]interface{}); !ok {
			if _, ok2 := c.(map[interface{}]interface{}); !ok2 {
				delete(matter, "custom")
			}
		}
	}
	// Ensure required fields with sensible defaults
	if _, ok := matter["title"]; !ok {
		title := inferTitleFromBody(body)
		if title == "" {
			title = "Untitled"
		}
		matter["title"] = title
	}
	if _, ok := matter["date"]; !ok {
		matter["date"] = time.Now().Format("2006-01-02")
	}
	if _, ok := matter["layout"]; !ok {
		// Heuristic: files named index.* are section pages
		matter["layout"] = "post.shtml"
	}
	if _, ok := matter["draft"]; !ok {
		matter["draft"] = false
	}
	return matter
}

func tryParseLongDashFrontmatter(body string, matter map[string]interface{}) (map[string]interface{}, string, bool) {
	bodyTrim := strings.TrimLeft(body, "\n\r\t ")
	if !strings.HasPrefix(bodyTrim, "----") {
		return matter, body, false
	}
	// Try to match long dash frontmatter at start
	loc := longDashFMRe.FindStringSubmatchIndex(bodyTrim)
	if loc == nil {
		return matter, body, false
	}
	inner := bodyTrim[loc[2]:loc[3]]
	rest := bodyTrim[loc[1]:]
	// Try YAML parse of inner
	var parsed map[string]interface{}
	if err := yaml.Unmarshal([]byte(inner), &parsed); err == nil && len(parsed) > 0 {
		// Merge into matter
		if matter == nil {
			matter = make(map[string]interface{})
		}
		for k, v := range parsed {
			matter[k] = v
		}
		return matter, rest, true
	}
	// Fallback: simple key: value parsing
	parsed = make(map[string]interface{})
	for _, line := range strings.Split(inner, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		sep := strings.Index(line, ":")
		if sep < 0 {
			continue
		}
		k := strings.TrimSpace(line[:sep])
		v := strings.TrimSpace(line[sep+1:])
		if k == "" {
			continue
		}
		// Handle nested menu etc. - keep as string for now
		parsed[k] = v
	}
	if len(parsed) > 0 {
		if matter == nil {
			matter = make(map[string]interface{})
		}
		for k, v := range parsed {
			if _, exists := matter[k]; !exists {
				matter[k] = v
			}
		}
		return matter, rest, true
	}
	return matter, body, false
}

func MdToSmd(input string) (string, error) {
	r := strings.NewReader(input)
	var matter map[string]interface{}
	body, err := frontmatter.Parse(r, &matter)
	if err != nil {
		// Lenient fallback: treat body after frontmatter as content if YAML is malformed
		// (e.g., unquoted colon in title like "How to Use ...: A Guide").
		fm, _, rest := extractFrontmatter(input)
		if fm != "" {
			var parsed map[string]interface{}
			if yamlErr := yaml.Unmarshal([]byte(fm), &parsed); yamlErr == nil && len(parsed) > 0 {
				matter = parsed
				body = []byte(rest)
				err = nil
			} else {
				// Manual lenient parse: split each line on first ':' to salvage title/date
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
					v := strings.TrimSpace(line[sep+1:])
					v = strings.Trim(v, `"'`)
					if k != "" && v != "" {
						parsed[k] = v
					}
				}
				if len(parsed) > 0 {
					matter = parsed
				} else {
					matter = make(map[string]interface{})
				}
				body = []byte(rest)
				err = nil
			}
		} else {
			matter = make(map[string]interface{})
			body = []byte(input)
			err = nil
		}
		if err != nil {
			return "", fmt.Errorf("parsing frontmatter: %w", err)
		}
	}
	// Handle custom long-dash frontmatter (casual-markdown style) if YAML frontmatter not found
	if len(matter) == 0 {
		if newMatter, rest, ok := tryParseLongDashFrontmatter(string(body), matter); ok {
			matter = newMatter
			body = []byte(rest)
		}
	}
	// Super compatible: ensure frontmatter always has required fields
	matter = sanitizeMatter(matter, string(body))
	var smdFM string
	smdFM = mapToZiggy(matter, "")
	processed, blocks := extractFencedBlocks(string(body))
	processed = stripMdxImports(processed)
	processed = stripHtmlComments(processed)
	processed = handleInlineHtml(processed)
	processed = normalizeThematicBreaks(processed)
	processed = normalizeHeadings(processed)
	// Linked images must be handled before standalone images/links to avoid
	// partial conversion: [![alt](img)](link) -> [[alt]($image...)]($link...)
	processed = linkedImageRe.ReplaceAllStringFunc(processed, convertLinkedImage)
	processed = imageRe.ReplaceAllStringFunc(processed, convertImage)
	processed = linkRe.ReplaceAllStringFunc(processed, convertLink)
	// Second pass: catch HTML comments that survived due to link conversion inside them
	// or comments that were wrapped in =html but should be stripped entirely.
	processed = stripHtmlComments(processed)
	// Remove any remaining =html blocks that only contained a comment (now empty)
	processed = regexp.MustCompile("(?m)^```=html\\n\\s*\\n```\\n?").ReplaceAllString(processed, "")
	processed = restoreBlocks(processed, blocks)
	// Collapse excessive blank lines left by stripping (e.g., removed imports/comments)
	processed = regexp.MustCompile(`\n{3,}`).ReplaceAllString(processed, "\n\n")
	// Ensure body starts with a heading level 1 if no heading present? Not required
	smdFM = "---\n" + smdFM + "---\n"
	return smdFM + processed, nil
}
// RepairSmdContent fixes already-generated .smd files in-place: strips HTML comments
// and normalizes heading levels so the document starts at #1 and never skips.
func RepairSmdContent(input string) (string, error) {
	fm, _, rest := extractFrontmatter(input)
	processed, blocks := extractFencedBlocks(rest)
	// Strip HTML comments (the "inline html forbidden" error)
	processed = stripHtmlComments(processed)
	processed = handleInlineHtml(processed)
	processed = normalizeThematicBreaks(processed)
	processed = normalizeHeadings(processed)
	// Fix legacy broken linked images: [[cap]($image...)](https://...) -> [[cap]($image...)]($link...)
	processed = smdLinkedImageLegacyRe.ReplaceAllStringFunc(processed, fixLegacyLinkedImage)
	// Second pass strip after handling
	processed = stripHtmlComments(processed)
	processed = regexp.MustCompile("(?m)^```=html\\n\\s*\\n```\\n?").ReplaceAllString(processed, "")
	processed = restoreBlocks(processed, blocks)
	processed = regexp.MustCompile(`\n{3,}`).ReplaceAllString(processed, "\n\n")
	if fm != "" {
		// Preserve original ziggy frontmatter exactly (trimmed)
		fmBlock := "---\n" + strings.Trim(fm, "\n") + "\n---\n"
		// Ensure frontmatter ends with newline
		if !strings.HasSuffix(fm, "\n") {
			fmBlock = "---\n" + fm + "\n---\n"
		}
		// Re-trim leading newlines from body
		processed = strings.TrimLeft(processed, "\n")
		return fmBlock + processed, nil
	}
	return processed, nil
}

func SmdToMd(input string) (string, error) {
	fm, _, rest := extractFrontmatter(input)
	var mdFM string
	if fm != "" {
		var err error
		mdFM, err = ziggyToYaml(fm)
		if err != nil {
			return "", err
		}
	}
	processed, blocks := extractFencedBlocks(rest)
	processed = smdToMdConvert(processed)
	processed = restoreBlocks(processed, blocks)
	// Convert =html linked images back to markdown syntax
	processed = htmlLinkedImageRe.ReplaceAllStringFunc(processed, convertHtmlLinkedImage)
	if mdFM != "" {
		mdFM = "---\n" + mdFM + "---\n"
	}
	return mdFM + processed, nil
}
