// Package redact removes common credential material from user-visible output.
package redact

import (
	"encoding/json"
	"regexp"
	"strings"
)

var patterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(authorization:\s*(?:bearer|basic)\s+)[^\s]+`),
	regexp.MustCompile(`(?i)((?:password|passwd|secret|token|api[_-]?key)\s*[=:]\s*)[^\s,;]+`),
	regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`),
	regexp.MustCompile(`\beyJ[a-zA-Z0-9_-]{10,}\.[a-zA-Z0-9_-]{10,}\.[a-zA-Z0-9_-]{10,}\b`),
}

// Text redacts likely credentials and reports how many replacements were made.
func Text(content string) (string, int) {
	redactions := 0
	for _, pattern := range patterns {
		content = pattern.ReplaceAllStringFunc(content, func(match string) string {
			redactions++
			if idx := strings.IndexAny(match, "=:"); idx >= 0 {
				return match[:idx+1] + "[REDACTED]"
			}
			parts := strings.Fields(match)
			if len(parts) > 1 {
				return strings.Join(parts[:len(parts)-1], " ") + " [REDACTED]"
			}
			return "[REDACTED]"
		})
	}
	return content, redactions
}

// JSON redacts sensitive keys and credential-shaped strings in a JSON object.
func JSON(raw []byte) []byte {
	if len(raw) == 0 {
		return raw
	}
	var value interface{}
	if err := json.Unmarshal(raw, &value); err != nil {
		text, _ := Text(string(raw))
		return []byte(text)
	}
	redactValue(value, "")
	out, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return out
}

func redactValue(value interface{}, parentKey string) {
	switch typed := value.(type) {
	case map[string]interface{}:
		for key, child := range typed {
			if sensitiveKey(key) || (parentKey == "env" && key == "value") {
				typed[key] = "[REDACTED]"
				continue
			}
			if text, ok := child.(string); ok {
				typed[key], _ = Text(text)
				continue
			}
			redactValue(child, key)
		}
	case []interface{}:
		for i, child := range typed {
			if text, ok := child.(string); ok {
				typed[i], _ = Text(text)
				continue
			}
			redactValue(child, parentKey)
		}
	case string:
		// Strings are handled by their parent map assignment; free-standing
		// values are not addressable through an interface copy.
	}
}

func sensitiveKey(key string) bool {
	switch strings.ToLower(key) {
	case "data", "stringdata", "token", "password", "passwd", "authorization",
		"apikey", "api_key", "clientsecret", "client_secret":
		return true
	default:
		return false
	}
}
