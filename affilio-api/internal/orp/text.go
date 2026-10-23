package orp

import "regexp"

// dirtyPattern matches the characters and markup a host must refuse.
// Newline and tab are allowed. The markup test is the same as the reference
// host: '<' followed by a letter, '!' or '/'.
var dirtyPattern = regexp.MustCompile(`[\x{202A}-\x{202E}\x{2066}-\x{2069}\x{200B}-\x{200D}\x{FEFF}\x{0000}-\x{0008}\x{000B}\x{000C}\x{000E}-\x{001F}\x{007F}-\x{009F}]|<[A-Za-z!/]`)

// DirtyPointer returns a JSON Pointer to the first disallowed string, or "".
func DirtyPointer(value any) string {
	return dirtyAt(value, "")
}

func dirtyAt(value any, path string) string {
	switch v := value.(type) {
	case string:
		if dirtyPattern.MatchString(v) {
			if path == "" {
				return "/"
			}
			return path
		}
		return ""
	case []any:
		for i, item := range v {
			if found := dirtyAt(item, path+"/"+itoa(i)); found != "" {
				return found
			}
		}
		return ""
	case map[string]any:
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		// Stable pointer: first key in sorted order that is dirty.
		sortStrings(keys)
		for _, key := range keys {
			if found := dirtyAt(v[key], path+"/"+escapePointer(key)); found != "" {
				return found
			}
		}
		return ""
	default:
		return ""
	}
}

func escapePointer(token string) string {
	out := make([]byte, 0, len(token))
	for i := 0; i < len(token); i++ {
		switch token[i] {
		case '~':
			out = append(out, '~', '0')
		case '/':
			out = append(out, '~', '1')
		default:
			out = append(out, token[i])
		}
	}
	return string(out)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

func sortStrings(keys []string) {
	for i := 1; i < len(keys); i++ {
		j := i
		for j > 0 && keys[j] < keys[j-1] {
			keys[j], keys[j-1] = keys[j-1], keys[j]
			j--
		}
	}
}
