package types

import (
	"fmt"
)

// MaxIdentifierLength is the longest identifier ValidateIdentifier accepts. It
// fits the VARCHAR(64) columns identifiers are stored in.
const MaxIdentifierLength = 64

// ValidateIdentifier returns an error unless s is a well-formed identifier: 1
// to MaxIdentifierLength characters of [a-z0-9_].
//
// An identifier is a config value that names something and may be stored as a
// key, for example a session name in a database row. The rule keeps such a
// value the same wherever it lands:
//
//   - lowercase only, so two identifiers cannot differ by case alone, which a
//     case-insensitive database collation would merge
//   - no spaces, separators or other special characters, so it never needs
//     quoting or escaping in a query, a path or a metric label
//   - a bounded length, so it fits the column it is stored in
//
// It is meant for every config value of that kind. There is no identifier type
// yet: validate the string where the config is loaded.
func ValidateIdentifier(s string) error {
	if s == "" {
		return fmt.Errorf("identifier is empty, expected 1 to %d characters of [a-z0-9_]", MaxIdentifierLength)
	}

	for i, c := range s {
		if !isIdentifierChar(c) {
			return fmt.Errorf("identifier %q has the character %q at position %d, expected only [a-z0-9_]", s, c, i)
		}
	}

	// every character is ASCII here, so bytes are characters
	if len(s) > MaxIdentifierLength {
		return fmt.Errorf("identifier %q has %d characters, expected at most %d", s, len(s), MaxIdentifierLength)
	}

	return nil
}

func isIdentifierChar(c rune) bool {
	return (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_'
}
