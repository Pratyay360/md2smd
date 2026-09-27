package smd

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// dateLayout is the timestamp layout Ziggy .date(...) values are rendered in.
const dateLayout = "2006-01-02T15:04:05"

// mapToZiggy renders decoded YAML frontmatter back into a Ziggy assignment
// block, with keys sorted for a stable, diff-friendly output.
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
		return fmt.Sprintf("%s.%s = .date(%q),\n", indent, key, val.Format(dateLayout))
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
		return fmt.Sprintf(".date(%q)", val.Format(dateLayout))
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
		type kv struct {
			k   interface{}
			str string
		}
		var kvs []kv
		for k := range val {
			kvs = append(kvs, kv{k, fmt.Sprintf("%v", k)})
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
	out, err := yaml.Marshal(ziggyToMap(ziggyStr))
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
		key, value, ok := splitZiggyKeyValue(line)
		if !ok {
			continue
		}
		setNestedValue(data, strings.Split(key, "."), parseZiggyValue(value))
	}
	return data
}

func splitZiggyKeyValue(line string) (string, string, bool) {
	idx := strings.Index(line, "=")
	if idx < 0 {
		return "", "", false
	}
	return strings.TrimSpace(line[:idx]), strings.TrimSpace(line[idx+1:]), true
}

func parseZiggyValue(value string) interface{} {
	value = strings.TrimSpace(value)
	if len(value) == 0 {
		return ""
	}
	if isQuoted(value) {
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
		// splitScriptyArgs respects commas inside quoted strings.
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
			fk := strings.TrimPrefix(strings.TrimSpace(field[:eqIdx]), ".")
			result[fk] = parseZiggyValue(strings.TrimSpace(field[eqIdx+1:]))
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

func isQuoted(value string) bool {
	return len(value) >= 2 &&
		((strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`)) ||
			(strings.HasPrefix(value, `'`) && strings.HasSuffix(value, `'`)))
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

// isDateString reports whether a string looks like an ISO-8601 date, and so
// should be emitted as a Ziggy .date(...) value rather than a plain string.
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
	return rest[0] == 'T' || rest[0] == ' '
}
