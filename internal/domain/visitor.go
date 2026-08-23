package domain

import (
	"errors"
	"strings"
	"unicode"
)

var ErrBadVisitor = errors.New("Could not start that session.")

func NormalizeVisitorID(id string) (string, error) {
	id = strings.TrimSpace(id)
	if len(id) < 8 || len(id) > 64 {
		return "", ErrBadVisitor
	}

	for _, r := range id {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' {
			continue
		}
		return "", ErrBadVisitor
	}

	return id, nil
}
