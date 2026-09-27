package smd

import (
	"fmt"
	"strings"
)

// directiveCall is a single Scripty method call inside a directive expression,
// e.g. `alt("pic")` within `$image.asset("a.png")`.
type directiveCall struct {
	function string
	args     []string
}

// findMatchingParen returns the index of the ")" closing the "(" at start,
// ignoring parentheses that appear inside string literals.
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

// splitScriptyArgs splits an argument list on top-level commas so that commas
// inside quoted strings, arrays, objects or nested calls are preserved.
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

// parseDirectiveCalls splits an expression such as
// `$image.asset("a.png").alt("hi")` into the directive name and its calls.
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

func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && ((s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'')) {
		return s[1 : len(s)-1]
	}
	return s
}

// classifyImageURL maps a Markdown image path onto the right $image function.
func classifyImageURL(url string) string {
	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
		return fmt.Sprintf("$image.url(%q)", url)
	}
	if strings.HasPrefix(url, "/") {
		return fmt.Sprintf("$image.siteAsset(%q)", url)
	}
	return fmt.Sprintf("$image.asset(%q)", url)
}

// classifyLinkURL maps a Markdown link target onto the right $link function.
func classifyLinkURL(url string) string {
	switch {
	case strings.HasPrefix(url, "http://"), strings.HasPrefix(url, "https://"):
		return fmt.Sprintf("$link.url(%q).new(true)", url)
	case strings.HasPrefix(url, "#"):
		return fmt.Sprintf("$link.ref(%q)", url[1:])
	case strings.HasPrefix(url, "/"):
		return fmt.Sprintf("$link.page(%q)", strings.TrimPrefix(url, "/"))
	case strings.HasPrefix(url, "./"):
		return fmt.Sprintf("$link.sub(%q)", url[2:])
	default:
		return fmt.Sprintf("$link.sibling(%q)", url)
	}
}
