package handlers

import (
	"net/url"
	"strings"
)

// publicReturnFrom reads return_to from a raw query string.
// A same-origin relative path is returned. An absolute URL, a host,
// a protocol-relative path, a fragment, a backslash, or a broken
// percent-escape is ignored. The caller does not redirect to the result.
// This does not call Request.ParseForm: a broken escape makes ParseForm
// fail, and treating that failure as HTTP 400 hides the page.
func publicReturnFrom(rawQuery string) string {
	raw, ok := rawQueryFirst(rawQuery, "return_to")
	if !ok {
		return ""
	}
	value, err := url.QueryUnescape(raw)
	if err != nil {
		return ""
	}
	return sameOriginRelativePath(value)
}

func rawQueryFirst(rawQuery, key string) (string, bool) {
	for rawQuery != "" {
		var pair string
		pair, rawQuery, _ = strings.Cut(rawQuery, "&")
		if pair == "" || strings.Contains(pair, ";") {
			continue
		}
		name, value, _ := strings.Cut(pair, "=")
		decoded, err := url.QueryUnescape(name)
		if err != nil || decoded != key {
			continue
		}
		return value, true
	}
	return "", false
}

func sameOriginRelativePath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || strings.ContainsAny(value, "\\\r\n") {
		return ""
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed == nil || parsed.IsAbs() || parsed.Host != "" || parsed.Fragment != "" || parsed.Opaque != "" {
		return ""
	}
	if !strings.HasPrefix(parsed.Path, "/") || strings.HasPrefix(parsed.Path, "//") || strings.Contains(parsed.Path, "..") {
		return ""
	}
	return parsed.RequestURI()
}
