package tokenizer

import (
	"fmt"
	"io"

	"github.com/sblinch/kdl-go/relaxed"
)

// readWhitespace reads all whitespace starting from the current position. It does not return an error as in practice it
// is only called after r.peek() has already been invoked and returned a whitespace character, and thus at least one
// whitespace character will always be available.
func (s *Scanner) readWhitespace() []byte {
	ws, _ := s.readWhile(isWhiteSpace, 1)
	return ws
}

// skipWhitespace skips zero or more whitespace characters from the current position, and returns a non-nil error on
// failure
func (s *Scanner) skipWhitespace() error {
	_, err := s.readWhile(isWhiteSpace, 0)
	return err
}

// readMultiLineComment reads and returns a multiline comment from the current position, supporting nested /* and */
// sequences. It returns a non-nil error on failure.
func (s *Scanner) readMultiLineComment() ([]byte, error) {
	s.pushMark()
	defer s.popMark()

	depth := 0
	for {
		c, err := s.get()
		if err != nil {
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			return nil, err
		}

		switch c {
		case '*':
			if next, err := s.peek(); err == nil && next == '/' {
				depth--
				s.skip()

				if depth == 0 {
					return s.copyFromMark(), nil
				}
			}

		case '/':
			if next, err := s.peek(); err == nil && next == '*' {
				depth++
				s.skip()
			}
		}
	}
}

// skipUntilNewline skips all characters from the current position until the next newline. It returns a non-nil error on
// failure.
func (s *Scanner) skipUntilNewline() error {
	escaped := false
	for {
		c, err := s.get()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		switch c {
		case '\\':
			escaped = true
			if err := s.skipWhitespace(); err != nil {
				return err
			}
		case '\r':
			// swallow error on peek, as it's still a valid newline if \r is not followed by \n
			if c, err := s.peek(); err == nil && c == '\n' {
				s.skip()
			}
			if escaped {
				escaped = false
			} else {
				return nil
			}

		case '\n', '\u0085', '\u000c', '\u2028', '\u2029':
			if escaped {
				escaped = false
			} else {
				return nil
			}
		default:
			escaped = false
		}
	}
}

// readSingleLineComment reads and returns a single-line comment from the current position, or a non-nil error on
// failure.
func (s *Scanner) readSingleLineComment() ([]byte, error) {
	literal, err := s.readUntil(isNewline, false)
	if err == io.ErrUnexpectedEOF {
		err = nil
	}
	return literal, err
}

// readRawString reads and returns a raw string from the input, or returns a non-nil error on failure
func (s *Scanner) readRawString() ([]byte, error) {
	s.pushMark()
	defer s.popMark()

	var (
		c   rune
		err error
	)

	startHashes := 0

	if c, err = s.get(); err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return nil, err
	}
	if c != 'r' {
		return nil, fmt.Errorf("unexpected character %c", c)
	}

hashLoop:
	for {
		if c, err = s.get(); err != nil {
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			return nil, err
		}
		switch c {
		case '"':
			break hashLoop
		case '#':
			startHashes++
		default:
			return nil, fmt.Errorf("unexpected character %c", c)
		}
	}

	if err := s.readRawStringContent(startHashes); err != nil {
		return nil, err
	}
	return s.copyFromMark(), nil
}

// readRawStringContent reads the content of a raw string until the closing quote and hashes.
func (s *Scanner) readRawStringContent(hashCount int) error {
	for {
		c, err := s.get()
		if err != nil {
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			return err
		}

		if c == '"' {
			// potentially the end; check for hashCount hashes
			matchedHashes := 0
			for matchedHashes < hashCount {
				next, err := s.peek()
				if err != nil {
					// EOF or other error while looking for hashes
					break
				}
				if next != '#' {
					break
				}
				s.skip()
				matchedHashes++
			}

			if matchedHashes == hashCount {
				return nil
			}
			// not the end; some hashes matched but then we found something else
			// (or no hashes were needed and we're done, but the loop handles that)
		}

		if s.Version == VersionV2 && isNewline(c) {
			return fmt.Errorf("unexpected character %c", c)
		}
	}
}

func (s *Scanner) readQuotedString() ([]byte, error) {
	return s.readQuotedStringQ('"')
}

func (s *Scanner) readSingleQuotedString() ([]byte, error) {
	return s.readQuotedStringQ('\'')
}

// readQuotedString reads and returns a quoted string from the current position, or returns a non-nil error on failure.
func (s *Scanner) readQuotedStringQ(q rune) ([]byte, error) {
	var (
		c   rune
		err error
	)
	if c, err = s.peek(); err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return nil, err
	}
	if c != q {
		return nil, fmt.Errorf("unexpected character %c", c)
	}

	escaped := false
	foldingWhitespace := false
	done := false
	first := true
	return s.readWhile(func(c rune) bool {
		if first {
			// skip "
			first = false
			return true
		}
		if done {
			return false
		}
		if foldingWhitespace {
			if isWhiteSpace(c) || isNewline(c) {
				return true
			}
			foldingWhitespace = false
		}
		switch c {
		case '\\':
			if s.Version == VersionV2 {
				next, err := s.peekAt(1)
				if err == nil && next == '/' {
					err = fmt.Errorf("unexpected character %c", next)
					return false
				}
			}
			escaped = !escaped
		case q:
			if escaped {
				escaped = false
			} else {
				done = true
			}
		case ' ', '\t':
			if s.Version == VersionV2 && escaped {
				escaped = false
				foldingWhitespace = true
			}
		case '\n', '\r':
			if s.Version != VersionV2 {
				break
			}
			if !escaped {
				err = fmt.Errorf("unexpected character %c", c)
				return false
			}
			escaped = false
			foldingWhitespace = true
		default:
			if escaped {
				escaped = false
			}
		}
		return true
	}, 2)

}

// readBareIdentifier reads a bare identifier from the current position and returns a TokenID representing its type
// (either BareIdentifier, Boolean, or Null), the byte sequence for the identifier, and a non-nil error on failure
func (s *Scanner) readBareIdentifier() (TokenID, []byte, error) {
	var (
		c   rune
		err error
	)

	if c, err = s.peek(); err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}

		return Unknown, nil, err
	}

	switch c {
	case '+', '-':
		if _, c, err = s.peekTwo(); err != nil {
			if err == io.EOF {
				literal, err := s.readWhile(func(r rune) bool {
					return r == '+' || r == '-'
				}, 1)
				return BareIdentifier, literal, err
			}
			return Unknown, nil, err
		}
		if isDigit(c) {
			return Unknown, nil, fmt.Errorf("unexpected character %c", c)
		}
	default:
		if !isBareIdentifierStartCharVersion(c, s.RelaxedNonCompliant, s.Version) {
			return Unknown, nil, fmt.Errorf("unexpected character %c", c)
		}
	}

	var literal []byte

	isBareIdentifierCharClosure := func(c rune) bool {
		return isBareIdentifierCharVersion(c, s.RelaxedNonCompliant, false, s.Version)
	}

	if literal, err = s.readWhile(isBareIdentifierCharClosure, 1); err != nil {
		return Unknown, nil, err
	}
	tokenType := BareIdentifier

	sLit := string(literal)
	if s.Version == VersionV2 {
		if sLit == "true" || sLit == "false" || sLit == "null" || sLit == "inf" || sLit == "-inf" || sLit == "nan" {
			return Unknown, nil, fmt.Errorf("bare %q is not valid in KDL v2; use #%s", sLit, sLit)
		}
	}

	if sLit == "true" || sLit == "false" {
		tokenType = Boolean
	} else if sLit == "null" {
		tokenType = Null
	}

	return tokenType, literal, nil
}

// readHashToken reads a KDL v2 token beginning with '#': a keyword (#true, #false, #null, #inf, #-inf, #nan)
// or a raw string (#"..."#).
func (s *Scanner) readHashToken() (TokenID, []byte, error) {
	s.pushMark()
	defer s.popMark()

	// consume '#'
	if _, err := s.get(); err != nil {
		return Unknown, nil, err
	}

	c, err := s.peek()
	if err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return Unknown, nil, err
	}

	switch {
	case c == '"' || c == '#':
		// v2 raw string: #"..."# or #"""..."""# etc.; first '#' already consumed
		hashCount := 1
		for {
			next, err := s.peek()
			if err != nil {
				return Unknown, nil, io.ErrUnexpectedEOF
			}
			if next != '#' {
				break
			}
			hashCount++
			s.skip()
		}

		// check if it's a triple-quoted raw string
		c1, c2, c3, err := s.peekThree()
		if err == nil && c1 == '"' && c2 == '"' && c3 == '"' {
			// leading hashes were already marked by readHashToken's caller or earlier
			literal, err := s.readMultiLineStringRawNoMark(hashCount)
			return QuotedString, literal, err
		}

		// next char must be '"' for regular raw string
		next, err := s.peek()
		if err != nil {
			return Unknown, nil, io.ErrUnexpectedEOF
		}
		if next != '"' {
			return Unknown, nil, fmt.Errorf("expected '\"' after '#' in raw string")
		}
		// consume '"'
		s.skip()

		if err := s.readRawStringContent(hashCount); err != nil {
			return Unknown, nil, err
		}
		return RawString, s.copyFromMark(), nil

	case c == 't':

		// #true
		for _, expected := range []rune("true") {
			ch, err := s.get()
			if err != nil {
				return Unknown, nil, err
			}
			if ch != expected {
				return Unknown, nil, fmt.Errorf("unexpected character %c", ch)
			}
		}
		return Boolean, s.copyFromMark(), nil

	case c == 'f':
		// #false
		for _, expected := range []rune("false") {
			ch, err := s.get()
			if err != nil {
				return Unknown, nil, err
			}
			if ch != expected {
				return Unknown, nil, fmt.Errorf("unexpected character %c", ch)
			}
		}
		return Boolean, s.copyFromMark(), nil

	case c == 'n':
		// #null or #nan — peek 2nd char to distinguish
		_, c2, err := s.peekTwo()
		if err != nil {
			return Unknown, nil, err
		}
		if c2 == 'u' {
			for _, expected := range []rune("null") {
				ch, err := s.get()
				if err != nil {
					return Unknown, nil, err
				}
				if ch != expected {
					return Unknown, nil, fmt.Errorf("unexpected character %c", ch)
				}
			}
			return Null, s.copyFromMark(), nil
		}
		for _, expected := range []rune("nan") {
			ch, err := s.get()
			if err != nil {
				return Unknown, nil, err
			}
			if ch != expected {
				return Unknown, nil, fmt.Errorf("unexpected character %c", ch)
			}
		}
		return Keyword, s.copyFromMark(), nil

	case c == 'i':
		// #inf
		for _, expected := range []rune("inf") {
			ch, err := s.get()
			if err != nil {
				return Unknown, nil, err
			}
			if ch != expected {
				return Unknown, nil, fmt.Errorf("unexpected character %c", ch)
			}
		}
		return Keyword, s.copyFromMark(), nil

	case c == '-':
		// #-inf
		for _, expected := range []rune("-inf") {
			ch, err := s.get()
			if err != nil {
				return Unknown, nil, err
			}
			if ch != expected {
				return Unknown, nil, fmt.Errorf("unexpected character %c", ch)
			}
		}
		return Keyword, s.copyFromMark(), nil

	default:
		return Unknown, nil, fmt.Errorf("unexpected character %c after '#'", c)
	}
}

// readMultiLineString reads a KDL v2 triple-quoted multi-line string ("""...""") from the current position.
func (s *Scanner) readMultiLineString() ([]byte, error) {
	return s.readMultiLineStringRaw(0)
}

// readMultiLineStringRaw reads a KDL v2 triple-quoted multi-line string with hashCount hashes ("""...""" or #"""..."""#).
func (s *Scanner) readMultiLineStringRaw(hashCount int) ([]byte, error) {
	s.pushMark()
	defer s.popMark()
	return s.readMultiLineStringRawNoMark(hashCount)
}

// readMultiLineStringRawNoMark is like readMultiLineStringRaw but does not push its own mark.
func (s *Scanner) readMultiLineStringRawNoMark(hashCount int) ([]byte, error) {
	// opening hashes were already consumed by readHashToken if hashCount > 0

	// consume the opening """
	for i := 0; i < 3; i++ {
		if _, err := s.get(); err != nil {
			return nil, io.ErrUnexpectedEOF
		}
	}

	// must be followed by optional whitespace then a newline
	for {
		c, err := s.peek()
		if err != nil {
			return nil, io.ErrUnexpectedEOF
		}
		if c == ' ' || c == '\t' {
			s.skip()
			continue
		}
		if c == '\n' || c == '\r' {
			// found the newline, now we can read the content
			break
		}
		return nil, fmt.Errorf("non-whitespace content after opening triple-quote")
	}

	// read until we find the closing """ followed by hashCount hashes
	for {
		c, err := s.get()
		if err != nil {
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			return nil, err
		}

		if c == '\\' && hashCount == 0 {
			// skip next char in standard multi-line strings
			if _, err := s.get(); err != nil {
				return nil, io.ErrUnexpectedEOF
			}
			continue
		}

		if c == '"' {
			// potentially the end; check if we have two more quotes
			c2, c3, err := s.peekTwo()
			if err == nil && c2 == '"' && c3 == '"' {
				s.skip() // 2nd "
				s.skip() // 3rd "

				// check for hashCount hashes
				matchedHashes := 0
				for matchedHashes < hashCount {
					next, err := s.peek()
					if err != nil {
						break
					}
					if next != '#' {
						break
					}
					s.skip()
					matchedHashes++
				}

				if matchedHashes == hashCount {
					return s.copyFromMark(), nil
				}
			}
		}
	}
}

// readIdentifier reads an identifier from the current position and returns a TokenID representing the identifier's
// type, a byte sequence representeing the identifier, and a non-nil error on failure
func (s *Scanner) readIdentifier() (TokenID, []byte, error) {
	c, err := s.peek()
	if err != nil {
		return Unknown, nil, err
	}

	if c <= 0x20 || c > 0x10FFFF {
		return Unknown, nil, fmt.Errorf("unexpected character %c", c)
	}

	switch c {
	case 'r':
		if s.Version != VersionV2 {
			_, c2, err := s.peekTwo()
			if err == nil && (c2 == '#' || c2 == '"') {
				literal, err := s.readRawString()
				return RawString, literal, err
			}
		}
	case '"':
		if s.Version == VersionV2 {
			// check for triple-quote multi-line string
			_, c2, c3, err2 := s.peekThree()
			if err2 == nil && c2 == '"' && c3 == '"' {
				s.log("multi-line string, reading")
				literal, err := s.readMultiLineString()
				return QuotedString, literal, err
			}
		}
		s.log("quoted string, reading")
		literal, err := s.readQuotedString()
		return QuotedString, literal, err

	case '\'':
		if s.Version != VersionV2 && s.RelaxedNonCompliant.Permit(relaxed.NGINXSyntax) {
			s.log("single quoted string, reading")
			literal, err := s.readSingleQuotedString()
			return QuotedString, literal, err
		}
	}

	_, c2, err := s.peekTwo()
	s.log("checking if bare identifier start", "c", c, "c2", c2)
	if err == nil && !isBareIdentifierStartCharVersion(c, s.RelaxedNonCompliant, s.Version) && !(c == '-' && !isDigit(c2)) {
		s.log("not a valid bare identifier")
		return Unknown, nil, fmt.Errorf("unexpected character %c", c)
	}

	s.log("bare identifier, reading")
	tokenType, literal, err := s.readBareIdentifier()
	return tokenType, literal, err
}

// readInteger reads and returns an integer from the current position, or a non-nil error on failure
func (s *Scanner) readInteger() (TokenID, []byte, error) {
	tokenID := Decimal

	first := true
	validRune := func(c rune) bool {
		if first {
			first = false
			return isDigit(c) // cannot start with _
		}
		return isDigit(c) || c == '_'
	}

	hasMultiplier := false
	if s.RelaxedNonCompliant.Permit(relaxed.MultiplierSuffixes) {
		multiplierOK := true
		validRune = func(c rune) bool {
			if first {
				first = false
				if c == '+' || c == '-' {
					multiplierOK = false
					return true
				}
			}

			if multiplierOK {
				switch c {
				case 'h', 'm', 's', 'u', 'µ', 'k', 'K', 'M', 'g', 'G', 't', 'T', 'b':
					hasMultiplier = true
					return true
				}
			}

			return isDigit(c) || c == '_'
		}
	}
	data, err := s.readWhile(validRune, 1)

	if hasMultiplier {
		tokenID = SuffixedDecimal
	}

	return tokenID, data, err
}

// readSignedInteger reads and returns a signed integer from the current position, or a non-nil error on failure
func (s *Scanner) readSignedInteger() (TokenID, []byte, error) {
	s.pushMark()
	defer s.popMark()

	c, err := s.peek()
	if err != nil {
		return Unknown, nil, err
	}

	if c == '+' || c == '-' {
		s.skip()
	}

	tokenID, _, err := s.readInteger()
	return tokenID, s.copyFromMark(), err
}

// readDecimal reads and returns a a decimal value (either an integer or a floating point number) from the current
// position, or a non-nil error on failure
func (s *Scanner) readDecimal() (TokenID, []byte, error) {
	s.pushMark()
	defer s.popMark()

	tokenID, _, err := s.readSignedInteger()
	if err != nil {
		s.log("reading decimal: failed", "error", err)
		return tokenID, nil, err
	}

	// ignore any error at this point because we've already successfully read the initial signed integer
	// r.log("reading decimal: peeky")
	if c, err := s.peek(); err == nil {
		if c == '.' {
			s.skip()

			// r.log("reading decimal: unsigned integer")
			if tokenID, _, err = s.readInteger(); err != nil {
				s.log("reading decimal: failed", "error", err)
				return tokenID, nil, err
			}
		}

		// again, ignore any error
		if c, err := s.peek(); err == nil {
			if c == 'e' || c == 'E' {
				s.skip()
				// r.log("reading decimal: signed integer")
				if tokenID, _, err := s.readSignedInteger(); err != nil {
					s.log("reading decimal: failed", "error", err)
					return tokenID, nil, err
				}
			}
		}
	}

	if c, err := s.peek(); err == nil && !isSeparator(c) {
		if s.RelaxedNonCompliant.Permit(relaxed.NGINXSyntax) && isBareIdentifierChar(c, s.RelaxedNonCompliant) {
			// it's not actually a numeric identifier; parse as a bare string

			isBareIdentifierCharClosure := func(c rune) bool {
				return isBareIdentifierChar(c, s.RelaxedNonCompliant)
			}

			if _, err = s.readWhile(isBareIdentifierCharClosure, 1); err != nil {
				return Unknown, nil, err
			}

			tokenID = BareIdentifier
		} else {
			return tokenID, nil, fmt.Errorf("unexpected character %c", c)
		}
	}

	return tokenID, s.copyFromMark(), nil
}

// readNumericBase reads and returns a binary, octal, or hexadecimal number from the current position, ensuring that it
// is at least 3 characters in length (eg: 0xN), followed by whitespace or a newline, and that all characters are valid;
// returns a non-nil error on failure
func (s *Scanner) readNumericBase(valid func(c rune) bool) ([]byte, error) {
	lit, err := s.readWhile(valid, 3)
	if err == nil && lit[2] == '_' {
		// disallow 0x_
		return nil, fmt.Errorf("unexpected character _")
	}
	if err == nil {
		if c, err := s.peek(); err == nil && !isWhiteSpace(c) && !isNewline(c) {
			return nil, fmt.Errorf("unexpected character %c", c)
		}
	}
	return lit, err
}

// readHexadecimal reads and returns a hexadecimal number from the current position, or a non-nil error on failure
func (s *Scanner) readHexadecimal() ([]byte, error) {
	n := 0
	return s.readNumericBase(func(c rune) bool {
		if n < 2 {
			// skip 0x
			n++
			return true
		}
		return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') || c == '_'
	})
}

// readOctal reads and returns an octal number from the current position, or a non-nil error on failure
func (s *Scanner) readOctal() ([]byte, error) {
	n := 0
	return s.readNumericBase(func(c rune) bool {
		if n < 2 {
			// skip 0o
			n++
			return true
		}
		return (c >= '0' && c <= '7') || c == '_'
	})
}

// readBinary reads and returns a binary number from the current position, or a non-nil error on failure
func (s *Scanner) readBinary() ([]byte, error) {
	n := 0
	return s.readNumericBase(func(c rune) bool {
		if n < 2 {
			// skip 0b
			n++
			return true
		}
		return c == '0' || c == '1' || c == '_'
	})
}
