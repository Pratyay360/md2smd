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
	imageRe        = regexp.MustCompile(`!\[([^\]]*)\]\(([^\s)]+)(?:\s+"([^"]*)")?\)`)
	linkRe         = regexp.MustCompile(`\[([^\]]+)\]\(([^\s)]+)(?:\s+"([^"]*)")?\)`)
	smdDirectiveRe = regexp.MustCompile(`\[([^\]]*)\]\(`)
	htmlCommentRe  = regexp.MustCompile(`(?s)<!--.*?-->`)
	headingRe      = regexp.MustCompile(`(?m)^(\s{0,3})(#{1,6})(\s)`)
	mdxImportRe    = regexp.MustCompile(`(?m)^\s*(import|export)\s+.*$`)
	htmlTagRe      = regexp.MustCompile(`<[^>]+>`)
	htmlLineRe     = regexp.MustCompile(`(?m)^\s*<[^>]+>\s*$`)
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
		var nestedKeys []string
		for sk := range val {
			nestedKeys = append(nestedKeys, fmt.Sprintf("%v", sk))
		}
		sort.Strings(nestedKeys)
		var lines []string
		for _, sk := range nestedKeys {
			lines = append(lines, ziggyValue(key+"."+sk, val[sk], depth))
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
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, fmt.Sprintf("%v", k))
		}
		sort.Strings(keys)
		var fields []string
		for _, k := range keys {
			fields = append(fields, fmt.Sprintf(".%s = %s", k, formatZiggyValueInline(val[k])))
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
	for {
		start := strings.Index(input[i:], "```")
		if start < 0 {
			result.WriteString(input[i:])
			break
		}
		result.WriteString(input[i : i+start])
		end := strings.Index(input[i+start+3:], "```")
		if end < 0 {
			result.WriteString(input[i+start:])
			break
		}
		end += 3
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
func MdToSmd(input string) (string, error) {
	r := strings.NewReader(input)
	var matter map[string]interface{}
	body, err := frontmatter.Parse(r, &matter)
	if err != nil {
		return "", fmt.Errorf("parsing frontmatter: %w", err)
	}
	var smdFM string
	if len(matter) > 0 {
		smdFM = mapToZiggy(matter, "")
	}
	processed, blocks := extractFencedBlocks(string(body))
	processed = stripMdxImports(processed)
	processed = stripHtmlComments(processed)
	processed = handleInlineHtml(processed)
	processed = normalizeHeadings(processed)
	processed = imageRe.ReplaceAllStringFunc(processed, convertImage)
	processed = linkRe.ReplaceAllStringFunc(processed, convertLink)
	processed = restoreBlocks(processed, blocks)
	// Collapse excessive blank lines left by stripping (e.g., removed imports/comments)
	processed = regexp.MustCompile(`\n{3,}`).ReplaceAllString(processed, "\n\n")
	if smdFM != "" {
		smdFM = "---\n" + smdFM + "---\n"
	}
	return smdFM + processed, nil
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
	if mdFM != "" {
		mdFM = "---\n" + mdFM + "---\n"
	}
	return mdFM + processed, nil
}
