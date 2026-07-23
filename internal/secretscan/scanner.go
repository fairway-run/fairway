// Package secretscan provides one content-free secret detector for Fairway
// packets and project-owned derived content.
package secretscan

import (
	"regexp"
	"strings"
)

var (
	assignment = regexp.MustCompile("(?i)\\b(access[_-]?token|refresh[_-]?token|id[_-]?token|client[_-]?secret|ssh[_-]?private[_-]?key|aws[_-]?secret[_-]?access[_-]?key|api[_-]?key|password|passwd|authorization|cookie|set-cookie|secret)\\s*[:=]\\s*[\\\"']?([^\\s\\\"'`]+)")
	bearer     = regexp.MustCompile(`(?i)\bauthorization\s*:\s*(?:bearer|basic)\s+\S+|\bbearer\s+[A-Za-z0-9._~+/=-]{16,}`)
	jwt        = regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\b`)
	known      = regexp.MustCompile(`\b(?:sk_live_|rk_live_|gh[pousr]_|github_pat_|AKIA)[A-Za-z0-9_-]{8,}`)
	pem        = regexp.MustCompile(`(?i)-----BEGIN [A-Z0-9][A-Z0-9 _-]*-----`)
	envVar     = regexp.MustCompile(`^\$\{[A-Za-z_][A-Za-z0-9_]*\}$`)
	tag        = regexp.MustCompile(`^<(?:redacted|masked|unset|none|example|changeme|secret|token|password)>$`)
)

// Contains reports secret-like material without returning the matched value.
func Contains(data []byte) bool {
	text := string(data)
	if pem.MatchString(text) || bearer.MatchString(text) || jwt.MatchString(text) || known.MatchString(text) {
		return true
	}
	for _, match := range assignment.FindAllStringSubmatch(text, -1) {
		if len(match) >= 3 && !placeholder(match[2]) {
			return true
		}
	}
	return false
}

func placeholder(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	if envVar.MatchString(value) || tag.MatchString(value) {
		return true
	}
	value = strings.Trim(value, "[]{}()*,.;")
	switch value {
	case "", "redacted", "none", "unset", "example", "changeme", "masked", "xxx":
		return true
	default:
		return false
	}
}
