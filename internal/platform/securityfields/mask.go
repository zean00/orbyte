package securityfields

import (
	"fmt"
	"strings"
)

func ApplyMask(mask string, value any) any {
	switch strings.TrimSpace(mask) {
	case "", "none":
		return value
	case "hide":
		return "[redacted]"
	case "last4":
		text := strings.TrimSpace(fmt.Sprintf("%v", value))
		if len(text) <= 4 {
			return text
		}
		return strings.Repeat("*", len(text)-4) + text[len(text)-4:]
	case "partial_email":
		text := strings.TrimSpace(fmt.Sprintf("%v", value))
		parts := strings.Split(text, "@")
		if len(parts) != 2 || parts[0] == "" {
			return "[redacted]"
		}
		prefix := parts[0]
		if len(prefix) > 2 {
			prefix = prefix[:2] + strings.Repeat("*", len(prefix)-2)
		}
		return prefix + "@" + parts[1]
	default:
		return "[redacted]"
	}
}
