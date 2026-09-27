package smd

import (
	"regexp"
	"strings"
	"time"
)

// knownTopLevel is the exact set of frontmatter fields Zine accepts on a
// Page. Anything else is rejected with "unknown field", so unknown keys are
// stashed under `.custom` instead of being dropped.
var knownTopLevel = map[string]bool{
	"title":           true,
	"description":     true,
	"date":            true,
	"authors":         true,
	"tags":            true,
	"layout":          true,
	"aliases":         true,
	"alternatives":    true,
	"translation_key": true,
	"skip_subdirs":    true,
	"draft":           true,
	"custom":          true,
}

// defaultLayout is used when the Markdown source has no layout. Zine requires
// `.layout` on every page, so emitting a page without one never builds.
const defaultLayout = "base.shtml"

// normalizeFrontmatter maps arbitrary Markdown (Hugo-style) frontmatter onto
// the Zine Page schema:
//
//   - known Zine fields stay top-level,
//   - common aliases are translated (author->authors, categories->tags, ...),
//   - everything else moves under `.custom` so no data is lost and Zine never
//     sees an "unknown field",
//   - mandatory title/date/layout get sensible defaults when absent.
func normalizeFrontmatter(data map[string]interface{}) map[string]interface{} {
	if data == nil {
		data = make(map[string]interface{})
	}
	// Work on lower-cased keys so `Title`, `Date`, `Draft`, ... all match.
	lowered := make(map[string]interface{}, len(data))
	for k, v := range data {
		lk := strings.ToLower(strings.TrimSpace(k))
		// First occurrence wins on collision after lowering.
		if _, exists := lowered[lk]; !exists {
			lowered[lk] = v
		}
	}

	out := make(map[string]interface{})

	// Pass through known fields as-is (coerced where needed below).
	for k := range knownTopLevel {
		if v, ok := lowered[k]; ok {
			out[k] = v
		}
	}

	// --- aliases ---------------------------------------------------------

	// author (singular) -> authors (list).
	if _, ok := out["authors"]; !ok {
		if v, ok := lowered["author"]; ok {
			out["authors"] = toStringSlice(v)
		}
	} else {
		out["authors"] = toStringSlice(out["authors"])
	}

	// categories -> tags (merged).
	var tags []interface{}
	if v, ok := out["tags"]; ok {
		tags = toStringSlice(v)
	}
	for _, alias := range []string{"categories", "category"} {
		if v, ok := lowered[alias]; ok {
			tags = append(tags, toStringSlice(v)...)
		}
	}
	if tags != nil {
		out["tags"] = tags
	}

	// description aliases.
	if _, ok := out["description"]; !ok {
		for _, alias := range []string{"summary", "excerpt", "subtitle"} {
			if v, ok := lowered[alias]; ok {
				out["description"] = toPlainString(v)
				break
			}
		}
	}

	// layout aliases (Hugo `type`).
	if _, ok := out["layout"]; !ok {
		if v, ok := lowered["type"]; ok {
			if s := toPlainString(v); s != "" {
				out["layout"] = s
			}
		}
	}

	// date aliases.
	if _, ok := out["date"]; !ok {
		for _, alias := range []string{"publishdate", "publish_date", "lastmod", "last_modified", "updated"} {
			if v, ok := lowered[alias]; ok {
				out["date"] = v
				break
			}
		}
	}

	// published (Hugo) is the inverse of draft.
	if _, ok := out["draft"]; !ok {
		if v, ok := lowered["published"]; ok {
			if b, ok := toBool(v); ok {
				out["draft"] = !b
			}
		}
	}
	if v, ok := out["draft"]; ok {
		if b, ok := toBool(v); ok {
			out["draft"] = b
		}
	}
	if v, ok := out["skip_subdirs"]; ok {
		if b, ok := toBool(v); ok {
			out["skip_subdirs"] = b
		}
	}
	if v, ok := out["aliases"]; ok {
		out["aliases"] = toStringSlice(v)
	}

	// headless (Hugo bundle flag) has no Zine equivalent. A headless bundle
	// is never rendered on its own, so the closest mapping is draft=true.
	// The original value is still preserved under custom.
	if v, ok := lowered["headless"]; ok {
		if b, ok := toBool(v); ok && b {
			if _, ok := out["draft"]; !ok {
				out["draft"] = true
			}
		}
	}

	// --- overflow into custom --------------------------------------------
	custom := make(map[string]interface{})
	if v, ok := out["custom"]; ok {
		switch m := v.(type) {
		case map[string]interface{}:
			for k, val := range m {
				custom[k] = val
			}
		case map[interface{}]interface{}:
			for k, val := range m {
				custom[toPlainString(k)] = val
			}
		}
	}
	for k, v := range lowered {
		if knownTopLevel[k] {
			continue
		}
		// Already consumed aliases must still be preserved, not dropped.
		switch k {
		case "author", "categories", "category", "summary", "excerpt",
			"subtitle", "type", "publishdate", "publish_date", "lastmod",
			"last_modified", "updated", "published", "headless", "weight":
			// fall through to custom below
		}
		// Skip keys already mapped to a canonical field under their alias
		// name only if they ARE the canonical name; alias originals are kept.
		custom[k] = v
	}
	if len(custom) > 0 {
		out["custom"] = custom
	} else {
		delete(out, "custom")
	}

	// --- mandatory defaults ----------------------------------------------
	if _, ok := out["date"]; !ok {
		out["date"] = time.Now().UTC().Format(dateLayout)
	}
	if _, ok := out["layout"]; !ok {
		out["layout"] = defaultLayout
	}
	// title default is filled by ensureTitle (needs the body); keep a blank
	// fallback here for callers that only normalize the map.
	if _, ok := out["title"]; !ok {
		out["title"] = "Untitled"
	}

	return out
}

// ensureTitle derives a title from the first ATX heading when the frontmatter
// has none (or the placeholder default), so pages without `title` still build
// with something meaningful.
func ensureTitle(data map[string]interface{}, body string) {
	if t, ok := data["title"]; ok && toPlainString(t) != "" && toPlainString(t) != "Untitled" {
		return
	}
	if h := firstHeading(body); h != "" {
		data["title"] = h
		return
	}
	if _, ok := data["title"]; !ok {
		data["title"] = "Untitled"
	}
}

var headingLineRe = regexp.MustCompile(`^(\s{0,3})(#{1,6})\s+(.*?)\s*#*\s*$`)

// normalizeHeadings rewrites ATX headings so Zine's "documents start at
// heading level 1" rule and its "skipped heading level" check both pass:
//
//  1. shift every heading up so the FIRST heading becomes `#`, preserving
//     relative hierarchy (###,#### -> #,##),
//  2. clamp any remaining jumps so a heading is never more than one level
//     deeper than its predecessor (#,### -> #,##).
//
// Lines inside fenced code blocks are left untouched. Callers should run this
// on text with code blocks already extracted (placeholders), or on plain
// text — the function also tracks fences itself for direct use.
func normalizeHeadings(input string) string {
	lines := strings.Split(input, "\n")
	// First pass: find the first heading level outside fences. Shifting by
	// the minimum is not enough: a doc opening with `###` and containing a
	// later `#` would still start at level 3 and fail to build.
	firstLevel := 0
	inFence := false
	var fence string
	for _, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			marker := trimmed[:3]
			if !inFence {
				inFence = true
				fence = marker
			} else if marker == fence {
				inFence = false
			}
			continue
		}
		if inFence {
			continue
		}
		m := headingLineRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		firstLevel = len(m[2])
		break
	}
	if firstLevel == 0 {
		return input
	}
	shift := firstLevel - 1
	// Second pass: apply shift + clamp jumps.
	var out []string
	inFence = false
	fence = ""
	prev := 0
	for _, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			marker := trimmed[:3]
			if !inFence {
				inFence = true
				fence = marker
			} else if marker == fence {
				inFence = false
			}
			out = append(out, line)
			continue
		}
		if inFence {
			out = append(out, line)
			continue
		}
		m := headingLineRe.FindStringSubmatch(line)
		if m == nil {
			out = append(out, line)
			continue
		}
		level := len(m[2]) - shift
		if level < 1 {
			level = 1
		}
		if level > 6 {
			level = 6
		}
		if prev != 0 && level > prev+1 {
			level = prev + 1
		}
		prev = level
		indent := m[1]
		text := m[3]
		out = append(out, indent+strings.Repeat("#", level)+" "+text)
	}
	return strings.Join(out, "\n")
}

func firstHeading(body string) string {
	inFence := false
	var fence string
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimLeft(line, " \t")
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			marker := trimmed[:3]
			if !inFence {
				inFence = true
				fence = marker
			} else if marker == fence {
				inFence = false
			}
			continue
		}
		if inFence {
			continue
		}
		if m := headingLineRe.FindStringSubmatch(line); m != nil {
			return strings.TrimSpace(m[3])
		}
	}
	return ""
}

func toPlainString(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case time.Time:
		return t.Format(dateLayout)
	default:
		_ = t
		return strings.Trim(strings.TrimSpace(formatZiggyValueInline(v)), `"`)
	}
}

func toStringSlice(v interface{}) []interface{} {
	switch t := v.(type) {
	case nil:
		return nil
	case []interface{}:
		out := make([]interface{}, 0, len(t))
		for _, e := range t {
			out = append(out, toPlainString(e))
		}
		return out
	case []string:
		out := make([]interface{}, 0, len(t))
		for _, e := range t {
			out = append(out, e)
		}
		return out
	case string:
		return []interface{}{t}
	default:
		return []interface{}{toPlainString(v)}
	}
}

func toBool(v interface{}) (bool, bool) {
	switch t := v.(type) {
	case bool:
		return t, true
	case string:
		switch strings.ToLower(strings.TrimSpace(t)) {
		case "true", "yes", "1":
			return true, true
		case "false", "no", "0":
			return false, true
		}
	}
	return false, false
}
