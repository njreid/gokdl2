package document

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	// noEscapeTable maps each ASCII value to a boolean value indicating whether it does NOT require escapement
	noEscapeTable = [256]bool{}
	// hexTable maps each hexadecimal digit (0-9, a-f, and A-F) to its decimal value
	hexTable = [256]rune{}
)

func init() {
	// initialize the maps
	for i := 0; i <= 0x7e; i++ {
		noEscapeTable[i] = i >= 0x20 && i != '\\' && i != '"'
	}

	for r := '0'; r <= '9'; r++ {
		hexTable[r] = r - '0'
	}
	for r := 'a'; r <= 'f'; r++ {
		hexTable[r] = r - 'a' + 10
	}
	for r := 'A'; r <= 'F'; r++ {
		hexTable[r] = r - 'A' + 10
	}
}

// QuoteString returns s quoted for use as a KDL FormattedString
func QuoteString(s string) string {
	b := make([]byte, 0, len(s)*5/4)
	return string(AppendQuotedString(b, s, '"'))
}

// AppendQuotedString appends s, quoted for use as a KDL FormattedString, to b, and returns the expanded buffer.
//
// AppendQuotedString is based on the JSON string quoting function from the MIT-Licensed ZeroLog, Copyright (c) 2017
// Olivier Poitrey, but has been heavily modified to improve performance and use KDL string escapes instead of JSON.
func AppendQuotedString(b []byte, s string, quote byte) []byte {
	b = append(b, quote)

	// use uints for bounds-check elimination
	lenS := uint(len(s))
	// Loop through each character in the string.
	for i := uint(0); i < lenS; i++ {
		// Check if the character needs encoding. Control characters, slashes,
		// and the double quote need json encoding. Bytes above the ascii
		// boundary needs utf8 encoding.
		if !noEscapeTable[s[i]] {
			// We encountered a character that needs to be encoded. Switch
			// to complex version of the algorithm.

			start := uint(0)
			for i < lenS {
				c := s[i]
				if noEscapeTable[c] {
					i++
					continue
				}

				if c >= utf8.RuneSelf {
					r, size := utf8.DecodeRuneInString(s[i:])
					if r == utf8.RuneError && size == 1 {
						// In case of error, first append previous simple characters to
						// the byte slice if any and append a replacement character code
						// in place of the invalid sequence.
						if start < i {
							b = append(b, s[start:i]...)
						}
						b = append(b, `\ufffd`...)
						i += uint(size)
						start = i
						continue
					}
					i += uint(size)
					continue
				}

				// We encountered a character that needs to be encoded.
				// Let's append the previous simple characters to the byte slice
				// and switch our operation to read and encode the remainder
				// characters byte-by-byte.
				if start < i {
					b = append(b, s[start:i]...)
				}

				switch c {
				case quote, '\\', '/':
					b = append(b, '\\', c)
				case '\n':
					b = append(b, '\\', 'n')
				case '\r':
					b = append(b, '\\', 'r')
				case '\t':
					b = append(b, '\\', 't')
				case '\b':
					b = append(b, '\\', 'b')
				case '\f':
					b = append(b, '\\', 'f')
				default:
					b = append(b, '\\', 'u')
					b = strconv.AppendUint(b, uint64(c), 16)
				}
				i++
				start = i
			}
			if start < lenS {
				b = append(b, s[start:]...)
			}

			b = append(b, quote)
			return b
		}
	}
	// The string has no need for encoding an therefore is directly
	// appended to the byte slice.
	b = append(b, s...)
	b = append(b, quote)

	return b
}

const empty = ""

// UnquoteString returns s unquoted from KDL FormattedString notation
func UnquoteString(s string) (string, error) {
	if len(s) == 0 {
		return empty, nil
	}
	q := s[0]
	switch q {
	case '"', '\'', '`':
	default:
		return "", ErrInvalid
	}

	b := make([]byte, 0, len(s))
	b, err := AppendUnquotedString(b, s, q)
	return string(b), err
}

var ErrInvalid = errors.New("invalid quoted string")

// AppendUnquotedString appends s, unquoted from KDL FormattedString notation, to b and returns the expanded buffer.
//
// AppendUnquotedString was originally based on the JSON string quoting function from the MIT-Licensed ZeroLog,
// Copyright (c) 2017 Olivier Poitrey, but has been heavily modified to unquote KDL quoted strings.
func AppendUnquotedString(b []byte, s string, quote byte) ([]byte, error) {
	if len(s) < 2 || s[0] != quote || s[len(s)-1] != quote {
		return nil, ErrInvalid
	}
	// remove quotes
	s = s[1 : len(s)-1]

	// use uints for bounds-check elimination
	lenS := uint(len(s))
	// Loop through each character in the string.
	for i := uint(0); i < lenS; i++ {
		c := s[i]
		// Check if the character needs decoding.
		if c == '\\' || c >= utf8.RuneSelf {
			// We encountered a character that needs to be decoded. Switch
			// to complex version of the algorithm.

			start := uint(0)
			for i < lenS {
				c := s[i]
				if !(c == '\\' || c >= utf8.RuneSelf) {
					i++
					continue
				}

				if c >= utf8.RuneSelf {
					r, size := utf8.DecodeRuneInString(s[i:])
					if r == utf8.RuneError && size == 1 {
						// In case of error, first append previous simple characters to
						// the byte slice if any and append a replacement character code
						// in place of the invalid sequence.
						if start < i {
							b = append(b, s[start:i]...)
						}
						b = append(b, `\ufffd`...)
						i += uint(size)
						start = i
						continue
					}
					i += uint(size)
					continue
				}

				// We encountered a character that needs to be decoded.
				// Let's append the previous simple characters to the byte slice
				// and switch our operation to read and encode the remainder
				// characters byte-by-byte.
				if start < i {
					b = append(b, s[start:i]...)
				}

				i++
				if i == lenS {
					return b, ErrInvalid
				}
				c = s[i]

				if isWhitespaceEscapeChar(c) {
					var consumed bool
					for i < lenS {
						r, size := utf8.DecodeRuneInString(s[i:])
						if !isWhitespaceEscapeRune(r) {
							break
						}
						consumed = true
						i += uint(size)
					}
					if !consumed {
						return b, ErrInvalid
					}
					start = i
					continue
				}

				switch c {
				case quote, '\\':
					b = append(b, c)
				case 'n':
					b = append(b, '\n')
				case 'r':
					b = append(b, '\r')
				case 't':
					b = append(b, '\t')
				case 'b':
					b = append(b, '\b')
				case 'f':
					b = append(b, '\f')
				case 's':
					// KDL v2: \s is a space character
					b = append(b, ' ')
				case 'u':
					// make sure we have enough room for `{n}`
					if i+3 >= lenS || s[i+1] != '{' {
						return b, ErrInvalid
					}
					i += 2

					// find the closing `}`
					rstart := i
					for i < lenS && s[i] != '}' {
						i++
					}
					if i >= lenS {
						return b, ErrInvalid
					}
					if i-rstart > 6 {
						return b, ErrInvalid
					}

					// convert the hex digits, working backwards
					r := rune(0)
					factor := rune(1)
					for j := i - 1; j >= rstart; j-- {
						r += hexTable[s[j]] * factor
						factor *= 16
					}
					if r > 0x10FFFF {
						return b, ErrInvalid
					}
					if r >= 0xD800 && r <= 0xDFFF {
						return b, ErrInvalid
					}
					b = utf8.AppendRune(b, r)
				case '/':
					b = append(b, c)
				default:
					return b, ErrInvalid
				}
				i++
				start = i
			}
			if start < lenS {
				b = append(b, s[start:]...)
			}

			return b, nil
		}
	}

	// The string has no need for decoding an therefore is directly
	// appended to the byte slice.
	b = append(b, s...)

	return b, nil
}

func isWhitespaceEscapeChar(c byte) bool {
	return c == ' ' || c == '\t' || c == '\r' || c == '\n'
}

func isWhitespaceEscapeRune(r rune) bool {
	return unicode.IsSpace(r)
}

func rawString(s string) string {
	b := make([]byte, 0, 1+8*2+len(s))
	return string(AppendRawString(b, s, -1))
}

// AppendRawString appends s, quoted for use as a KDL v1 RawString (r#"..."#), to b and returns the expanded buffer.
// If hashCount is -1, the minimum number of hashes required to quote s will be used.
func AppendRawString(b []byte, s string, hashCount int) []byte {
	if hashCount < 0 {
		hashes := append(make([]byte, 0, 64), '#')
		ok := false
		for i := 0; i < cap(hashes); i++ {
			terminator := `"` + string(hashes)
			if !strings.Contains(s, terminator) {
				ok = true
				break
			}
			hashes = append(hashes, '#')
		}
		if !ok {
			return append(b, "r\"invalid\""...)
		}
		hashCount = len(hashes) - 1
	}

	minSpace := 1 + hashCount + 1 + len(s) + 1 + hashCount
	if cap(b)-len(b) < minSpace {
		n := make([]byte, 0, len(b)+minSpace)
		n = append(n, b...)
		b = n
	}
	b = append(b, 'r')
	for i := 0; i < hashCount; i++ {
		b = append(b, '#')
	}
	b = append(b, '"')
	b = append(b, s...)
	b = append(b, '"')
	for i := 0; i < hashCount; i++ {
		b = append(b, '#')
	}
	return b
}

// AppendRawStringV2 appends s, quoted for use as a KDL v2 RawString (#"..."#), to b and returns the expanded buffer.
// If hashCount is -1, the minimum number of hashes required to quote s will be used.
func AppendRawStringV2(b []byte, s string, hashCount int) []byte {
	if hashCount < 0 {
		hashes := append(make([]byte, 0, 64), '#')
		ok := false
		for i := 0; i < cap(hashes); i++ {
			terminator := `"` + string(hashes)
			if !strings.Contains(s, terminator) {
				ok = true
				break
			}
			hashes = append(hashes, '#')
		}
		if !ok {
			return append(b, "\"invalid\""...)
		}
		hashCount = len(hashes) - 1
	}

	minSpace := hashCount + 1 + len(s) + 1 + hashCount
	if cap(b)-len(b) < minSpace {
		n := make([]byte, 0, len(b)+minSpace)
		n = append(n, b...)
		b = n
	}
	for i := 0; i < hashCount; i++ {
		b = append(b, '#')
	}
	b = append(b, '"')
	b = append(b, s...)
	b = append(b, '"')
	for i := 0; i < hashCount; i++ {
		b = append(b, '#')
	}
	return b
}

// UnquoteMultiLineString parses a KDL v2 triple-quoted multi-line string ("""...""") and returns the
// unescaped, dedented string content and the number of hashes used, or a non-nil error on failure.
//
// Processing rules per the KDL v2 spec:
//  1. The opening """ must be followed by optional whitespace then a newline.
//  2. The first newline (and preceding whitespace on the opening line) is stripped.
//  3. The indentation of the closing """ line defines the base indentation stripped from all content lines.
//  4. Escape sequences are processed as in regular quoted strings.
func UnquoteMultiLineString(s string) (string, int, error) {
	return unquoteMultiLineToken(s, `"""`, '"', true)
}

// UnquoteMultiLineExpressionString parses a triple-backtick multi-line expression string and returns its content.
func UnquoteMultiLineExpressionString(s string) (string, error) {
	v, _, err := unquoteMultiLineToken(s, "```", '`', false)
	return v, err
}

func unquoteMultiLineToken(s string, delim string, quote byte, allowRaw bool) (string, int, error) {
	hashCount := 0
	if allowRaw {
		for hashCount < len(s) && s[hashCount] == '#' {
			hashCount++
		}

		if hashCount > 0 {
			suffix := delim + strings.Repeat("#", hashCount)
			if !strings.HasPrefix(s[hashCount:], delim) || !strings.HasSuffix(s, suffix) {
				return "", 0, ErrInvalid
			}
			s = s[hashCount : len(s)-hashCount]
		}
	}

	if len(s) < len(delim)*2 || !strings.HasPrefix(s, delim) || !strings.HasSuffix(s, delim) {
		return "", 0, ErrInvalid
	}

	inner := s[len(delim) : len(s)-len(delim)]

	openingLineEnd, openingNewlineLen := firstNewline(inner)
	if openingLineEnd == -1 {
		return "", 0, ErrInvalid
	}
	if !isInlineWhitespaceOnly(inner[:openingLineEnd]) {
		return "", 0, fmt.Errorf("non-whitespace content after opening triple-quote")
	}
	inner = inner[openingLineEnd+openingNewlineLen:]

	content, indent, closingEscaped, err := parseMultilineTail(inner)
	if err != nil {
		return "", 0, err
	}
	if !isInlineWhitespaceOnly(indent) {
		return "", 0, fmt.Errorf("non-whitespace content on closing line of triple-quoted string")
	}

	dedented, err := dedentMultilineContent(content, indent)
	if err != nil {
		return "", 0, err
	}
	if hashCount > 0 {
		return dedented, hashCount, nil
	}
	if closingEscaped {
		dedented += `\`
	}

	b := make([]byte, 0, len(dedented))
	b, err = appendUnquotedMultilineContent(b, dedented, quote, closingEscaped)
	return string(b), hashCount, err
}

func parseMultilineClosingEscape(s string) (int, bool) {
	for i, r := range s {
		if r == '\\' {
			if !isInlineWhitespaceOnly(s[:i]) || !isInlineWhitespaceOnly(s[i+1:]) {
				return 0, false
			}
			return i, true
		}
	}
	return 0, false
}

func parseMultilineTail(inner string) (content string, indent string, closingEscaped bool, err error) {
	closingLineStart, closingNewlineLen := lastNewline(inner)
	if closingLineStart == -1 {
		if idx, ok := parseMultilineClosingEscape(inner); ok {
			return "", inner[:idx], true, nil
		}
		return "", inner, false, nil
	}

	closingLine := inner[closingLineStart+closingNewlineLen:]
	prefix := inner[:closingLineStart]
	if idx, ok := parseMultilineClosingEscape(closingLine); ok {
		return prefix, closingLine[:idx], true, nil
	}
	if !isInlineWhitespaceOnly(closingLine) {
		return "", "", false, fmt.Errorf("non-whitespace content on closing line of triple-quoted string")
	}

	prevLineStart, prevNewlineLen := lastNewline(prefix)
	prevLine := prefix
	prevPrefix := ""
	if prevLineStart != -1 {
		prevLine = prefix[prevLineStart+prevNewlineLen:]
		prevPrefix = prefix[:prevLineStart]
	}
	if idx, ok := parseMultilineClosingEscape(prevLine); ok {
		return prevPrefix, prevLine[:idx], true, nil
	}
	return prefix, closingLine, false, nil
}

func firstNewline(s string) (int, int) {
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\n':
			return i, 1
		case '\r':
			if i+1 < len(s) && s[i+1] == '\n' {
				return i, 2
			}
			return i, 1
		}
	}
	return -1, 0
}

func lastNewline(s string) (int, int) {
	for i := len(s) - 1; i >= 0; i-- {
		switch s[i] {
		case '\n':
			if i > 0 && s[i-1] == '\r' {
				return i - 1, 2
			}
			return i, 1
		case '\r':
			return i, 1
		}
	}
	return -1, 0
}

func isInlineWhitespaceOnly(s string) bool {
	for _, r := range s {
		if r == '\n' || r == '\r' {
			return false
		}
		if !unicode.IsSpace(r) {
			return false
		}
	}
	return true
}

func dedentMultilineContent(content string, indent string) (string, error) {
	if content == "" {
		return "", nil
	}

	var result strings.Builder
	continued := false
	for len(content) > 0 {
		lineEnd, newlineLen := firstNewline(content)
		line := content
		newline := ""
		if lineEnd != -1 {
			line = content[:lineEnd]
			newline = content[lineEnd : lineEnd+newlineLen]
			content = content[lineEnd+newlineLen:]
		} else {
			content = ""
		}

		if strings.TrimSpace(line) == "" {
			result.WriteString(newline)
			continued = hasLineContinuation(line)
			continue
		}
		if !continued && !strings.HasPrefix(line, indent) {
			return "", fmt.Errorf("line in triple-quoted string has insufficient indentation")
		}
		if continued {
			result.WriteString(line)
		} else {
			result.WriteString(line[len(indent):])
		}
		result.WriteString(newline)
		continued = hasLineContinuation(line)
	}
	return result.String(), nil
}

func hasLineContinuation(line string) bool {
	end := len(line)
	for end > 0 {
		r, size := utf8.DecodeLastRuneInString(line[:end])
		if !unicode.IsSpace(r) {
			return r == '\\'
		}
		end -= size
	}
	return false
}

// appendUnquotedContent processes escape sequences in raw string content (no surrounding quotes).
func appendUnquotedContent(b []byte, s string) ([]byte, error) {
	return appendUnquotedContentQuoted(b, s, '"')
}

func appendUnquotedContentQuoted(b []byte, s string, quote byte) ([]byte, error) {
	// wrap in quotes so we can reuse AppendUnquotedString
	quoted := string([]byte{quote}) + s + string([]byte{quote})
	return AppendUnquotedString(b, quoted, quote)
}

func appendUnquotedMultilineContent(b []byte, s string, quote byte, allowTrailingEscape bool) ([]byte, error) {
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r != '\\' {
			b = utf8.AppendRune(b, r)
			i += size
			continue
		}

		i += size
		if i >= len(s) {
			if allowTrailingEscape {
				return b, nil
			}
			return b, ErrInvalid
		}

		r, size = utf8.DecodeRuneInString(s[i:])
		if unicode.IsSpace(r) {
			for i < len(s) {
				r, size = utf8.DecodeRuneInString(s[i:])
				if !unicode.IsSpace(r) {
					break
				}
				i += size
			}
			continue
		}

		switch r {
		case 'n':
			b = append(b, '\n')
		case 'r':
			b = append(b, '\r')
		case 't':
			b = append(b, '\t')
		case 'b':
			b = append(b, '\b')
		case 'f':
			b = append(b, '\f')
		case 's':
			b = append(b, ' ')
		case rune(quote), '\\':
			b = append(b, byte(r))
		case 'u':
			if i+size >= len(s) || s[i+size] != '{' {
				return b, ErrInvalid
			}
			j := i + size + 1
			start := j
			for j < len(s) && s[j] != '}' {
				j++
			}
			if j >= len(s) || j-start > 6 {
				return b, ErrInvalid
			}
			rn := rune(0)
			factor := rune(1)
			for k := j - 1; k >= start; k-- {
				rn += hexTable[s[k]] * factor
				factor *= 16
			}
			if rn > 0x10FFFF || (rn >= 0xD800 && rn <= 0xDFFF) {
				return b, ErrInvalid
			}
			b = utf8.AppendRune(b, rn)
			i = j + 1
			continue
		default:
			return b, ErrInvalid
		}
		i += size
	}
	return b, nil
}

// AppendMultiLineString appends s, quoted for use as a KDL v2 multi-line string ("""..."""), to b and returns the expanded buffer.
func AppendMultiLineString(b []byte, s string) []byte {
	return appendMultiLineQuotedString(b, s, '"', `"""`)
}

// AppendMultiLineExpressionString appends s, quoted for use as a triple-backtick multi-line expression string.
func AppendMultiLineExpressionString(b []byte, s string) []byte {
	b = appendMultiLineQuotedString(b, s, '`', "```")
	if len(s) > 0 && s[len(s)-1] != '\n' {
		b = append(b[:len(b)-3], '\n')
		b = append(b, '`', '`', '`')
	}
	return b
}

func appendMultiLineQuotedString(b []byte, s string, quote byte, delim string) []byte {
	b = append(b, delim...)
	b = append(b, '\n')

	lines := strings.Split(s, "\n")
	for i, line := range lines {
		var lineBuf []byte
		lineBuf = AppendQuotedString(lineBuf, line, quote)
		if len(lineBuf) >= 2 {
			b = append(b, lineBuf[1:len(lineBuf)-1]...)
		}
		if i < len(lines)-1 {
			b = append(b, '\n')
		}
	}

	b = append(b, delim...)
	return b
}

// AppendMultiLineStringV2 appends s, quoted for use as a KDL v2 raw triple-quoted string (#"""..."""#), to b and returns the expanded buffer.
// If hashCount is -1, it will use the minimum hashes required (at least 1).
func AppendMultiLineStringV2(b []byte, s string, hashCount int) []byte {
	if hashCount < 0 {
		hashes := append(make([]byte, 0, 64), '#')
		ok := false
		for i := 0; i < cap(hashes); i++ {
			terminator := `"""` + string(hashes)
			if !strings.Contains(s, terminator) {
				ok = true
				break
			}
			hashes = append(hashes, '#')
		}
		if !ok {
			return append(b, "\"\"\"invalid\"\"\""...)
		}
		hashCount = len(hashes)
	}
	if hashCount == 0 {
		hashCount = 1
	}
	minSpace := hashCount + 3 + 1 + len(s) + 3 + hashCount
	if cap(b)-len(b) < minSpace {
		n := make([]byte, 0, len(b)+minSpace)
		n = append(n, b...)
		b = n
	}
	for i := 0; i < hashCount; i++ {
		b = append(b, '#')
	}
	b = append(b, '"', '"', '"', '\n')
	b = append(b, s...)
	if len(s) > 0 && s[len(s)-1] != '\n' {
		b = append(b, '\n')
	}
	b = append(b, '"', '"', '"')
	for i := 0; i < hashCount; i++ {
		b = append(b, '#')
	}
	return b
}
