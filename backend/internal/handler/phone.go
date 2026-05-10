package handler

import (
	"errors"
	"strings"
)

var errInvalidPhone = errors.New("invalid phone format")

// normalizePhone converts an Indonesian phone number to E.164 (+62...).
// Accepted inputs: "+62...", "62...", "0..." with optional spaces/dashes.
// Anything else (including valid E.164 from other countries) is rejected.
// MVP single-region; multi-region is a separate change.
func normalizePhone(phone string) (string, error) {
	s := strings.TrimSpace(phone)
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "-", "")

	var digits string
	switch {
	case strings.HasPrefix(s, "+62"):
		digits = s[3:]
	case strings.HasPrefix(s, "62"):
		digits = s[2:]
	case strings.HasPrefix(s, "0"):
		digits = s[1:]
	default:
		return "", errInvalidPhone
	}

	if len(digits) < 8 {
		return "", errInvalidPhone
	}
	for _, c := range digits {
		if c < '0' || c > '9' {
			return "", errInvalidPhone
		}
	}
	return "+62" + digits, nil
}
