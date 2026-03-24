package tokenizer

import (
	"github.com/sblinch/kdl-go/relaxed"
)

func isDisallowedV2Rune(c rune) bool {
	switch c {
	case 0x7F, 0x200E, 0x200F, 0x202A, 0x202B, 0x202D, 0x202E, 0x2066, 0x2067, 0x2068, 0x2069, 0x202C:
		return true
	default:
		return false
	}
}

// isWhiteSpace returns true if c is a whitespace character
func isWhiteSpace(c rune) bool {
	switch c {
	case // unicode-space
		'\t', ' ',
		'\u00A0',
		'\u1680',
		'\u2000',
		'\u2001',
		'\u2002',
		'\u2003',
		'\u2004',
		'\u2005',
		'\u2006',
		'\u2007',
		'\u2008',
		'\u2009',
		'\u200A',
		'\u202F',
		'\u205F',
		'\u3000':
		return true
	default:
		return false
	}
}

// isNewline returns true if c is a newline character
func isNewline(c rune) bool {
	switch c {
	case '\r', '\n', '\u000b', '\u000c', '\u0085', '\u2028', '\u2029':
		return true
	default:
		return false
	}
}

// isLineSpace returns true if c is a whitespace or newline character
func isLineSpace(c rune) bool {
	return isWhiteSpace(c) || isNewline(c)
}

// isDigit returns true if c is a digit
func isDigit(c rune) bool {
	return c >= '0' && c <= '9'
}

// isSign returns true if c is + or -
func isSign(c rune) bool {
	return c == '-' || c == '+'
}

// isSeparator returns true if c whitespace, a newline, or a semicolon
func isSeparator(c rune) bool {
	return isWhiteSpace(c) || isNewline(c) || c == ';'
}

// isBareIdentifierStartChar indicates whether c is a valid first character for a bare identifier. Note that this
// returns true if c is + or -, in which case the second character must not be a digit.
func isBareIdentifierStartChar(c rune, r relaxed.Flags) bool {
	return isBareIdentifierStartCharVersion(c, r, VersionAuto)
}

// isBareIdentifierStartCharVersion indicates whether c is a valid first character for a bare identifier in the given KDL version.
func isBareIdentifierStartCharVersion(c rune, r relaxed.Flags, v Version) bool {
	return isBareIdentifierCharVersion(c, r, true, v)
}

// isBareIdentifierChar indicates whether c is a valid character for a bare identifier
func isBareIdentifierChar(c rune, r relaxed.Flags) bool {
	return isBareIdentifierCharVersion(c, r, false, VersionAuto)
}

// isBareIdentifierCharVersion indicates whether c is a valid character for a bare identifier in the given KDL version
func isBareIdentifierCharVersion(c rune, r relaxed.Flags, first bool, v Version) bool {
	if isNewline(c) || isWhiteSpace(c) {
		return false
	}
	if c < 0x20 || c > 0x10FFFF {
		return false
	}
	if isDisallowedV2Rune(c) {
		return false
	}

	if v == VersionV2 {
		switch c {
		case '(', ')', '{', '}', '[', ']', '/', '\\', '"', '#', ';', '=':
			return false
		}
		if first && isDigit(c) {
			return false
		}
		return true
	}

	// KDL v1 (including VersionAuto)
	if first && isDigit(c) {
		return false
	}

	switch c {
	case '{', '}', '<', '>', ';', '[', ']', '=', ',':
		return false
	case '(', ')', '/', '\\', '"':
		return r.Permit(relaxed.NGINXSyntax)
	case ':':
		return !r.Permit(relaxed.YAMLTOMLAssignments) || r.Permit(relaxed.NGINXSyntax)
	}

	return true
}

// IsBareIdentifier returns true if s contains a valid BareIdentifier (a string that requires no quoting in KDL)
func IsBareIdentifier(s string, rf relaxed.Flags) bool {
	return IsBareIdentifierVersion(s, rf, VersionV1)
}

// IsBareIdentifierVersion returns true if s contains a valid BareIdentifier for the specified KDL version
func IsBareIdentifierVersion(s string, rf relaxed.Flags, v Version) bool {
	if len(s) == 0 {
		return false
	}

	first := true
	for _, r := range s {
		if !isBareIdentifierCharVersion(r, rf, first, v) {
			return false
		}
		first = false
	}
	return true
}
