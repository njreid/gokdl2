package tokenizer

import (
	"errors"
	"fmt"
	"unicode/utf8"
)

// This file holds the ASCII byte-loop fast paths. They only run when the whole input is already in s.input (no reader
// attached, so no refill can happen). Cached lookahead in s.peeked only mirrors the front of s.input, so a fast path
// that consumes input discards it. Line and column accounting, token data and error positions are identical to the
// rune-at-a-time paths; internal/tokenizer/differential_test.go and the kdl test suites guard that.

// identTableKey identifies the (version, flags) pair an identChars table was built for.
type identTableKey struct {
	version Version
	flags   int
}

// canFastScan reports whether the byte-loop fast paths may run.
func (s *Scanner) canFastScan() bool {
	return s.r == nil
}

// consumeFast consumes n bytes that the caller has verified are ASCII and not newlines, advancing the column by n.
func (s *Scanner) consumeFast(n int) {
	s.peeked = s.peeked[:0]
	s.input = s.input[n:]
	s.len -= n
	s.column += n
}

// identChars returns a table of the ASCII bytes that may continue a bare identifier under the scanner's current
// version and relaxed flags. Bytes >= 0x80 are always false.
func (s *Scanner) identChars() *[256]bool {
	key := identTableKey{s.Version, int(s.RelaxedNonCompliant)}
	if !s.identTableSet || s.identTableKey != key {
		for b := 0; b < utf8.RuneSelf; b++ {
			s.identTable[b] = isBareIdentifierCharVersion(rune(b), s.RelaxedNonCompliant, false, s.Version)
		}
		s.identTableKey = key
		s.identTableSet = true
	}
	return &s.identTable
}

// readWhitespaceFast consumes a run of spaces and tabs at the start of s.input. It reports false, consuming nothing,
// if the run is followed by a non-ASCII byte (which may be more unicode whitespace) so the rune path must handle it.
func (s *Scanner) readWhitespaceFast() ([]byte, bool) {
	in := s.input
	n := 1
	for n < len(in) && (in[n] == ' ' || in[n] == '\t') {
		n++
	}
	if n < len(in) && in[n] >= utf8.RuneSelf {
		return nil, false
	}
	off := len(s.raw) - len(in)
	s.consumeFast(n)
	return s.raw[off : off+n], true
}

// readBareIdentifierChars reads a run of bare identifier characters (at least one) from the current position.
func (s *Scanner) readBareIdentifierChars() ([]byte, error) {
	isChar := func(c rune) bool {
		return isBareIdentifierCharVersion(c, s.RelaxedNonCompliant, false, s.Version)
	}
	if !s.canFastScan() {
		return s.readWhile(isChar, 1)
	}

	s.pushMark()
	defer s.popMark()

	tbl := s.identChars()
	in := s.input
	n := 0
	for n < len(in) && tbl[in[n]] {
		n++
	}
	if n > 0 {
		s.consumeFast(n)
	}
	if n == 0 || (n < len(in) && in[n] >= utf8.RuneSelf) {
		// continue with the rune path; the mark still covers everything from the start of the token
		return s.readWhileMarked(isChar, 1, n > 0)
	}
	return s.copyFromMark(), nil
}

// quoteStop marks the bytes at which readQuotedStringFast must stop skipping and run its per-rune state machine:
// backslash, every newline (including \v and \f) and every non-ASCII byte.
var quoteStop = func() (t [256]bool) {
	for _, b := range []byte{'\\', '\n', '\r', '\v', '\f'} {
		t[b] = true
	}
	for b := utf8.RuneSelf; b < 256; b++ {
		t[b] = true
	}
	return
}()

var errUnexpectedNewlineInString = errors.New("unexpected newline in string")

// readQuotedStringFast is readQuotedStringQ for fully buffered input. It runs the same state machine without per-rune
// peek/get calls or a heap-allocated predicate closure, and leaves the scanner in the same position (including on
// errors) as the rune path would. The scanner must be positioned at the opening quote q.
func (s *Scanner) readQuotedStringFast(q byte) ([]byte, error) {
	in := s.input
	v2 := s.Version == VersionV2
	line, col := s.line, s.column

	i := 1 // the opening quote
	col++
	var (
		escaped, folding, done bool
		err                    error
	)
	for i < len(in) && !done {
		if !escaped && !folding {
			// nothing special can happen until the next stop byte or the closing quote
			j := i
			for j < len(in) && in[j] != q && !quoteStop[in[j]] {
				j++
			}
			col += j - i
			i = j
			if i == len(in) {
				break
			}
		}

		c, size := rune(in[i]), 1
		if c >= utf8.RuneSelf {
			if c, size = utf8.DecodeRune(in[i:]); c == utf8.RuneError {
				err = ErrInvalidRune
				break
			}
		}

		if folding && (isWhiteSpace(c) || isNewline(c)) {
			// still folding whitespace after an escaped line break or space
		} else {
			folding = false
			switch c {
			case '\\':
				if v2 && !escaped && i+size < len(in) && in[i+size] == '/' {
					err = fmt.Errorf("unexpected character %c", '/')
				}
				escaped = !escaped
			case rune(q):
				if escaped {
					escaped = false
				} else {
					done = true
				}
			case ' ', '\t':
				if v2 && escaped {
					escaped = false
					folding = true
				}
			case '\n', '\r':
				if v2 {
					if !escaped {
						err = errUnexpectedNewlineInString
					} else {
						escaped = false
						folding = true
					}
				}
			default:
				escaped = false
			}
			if err != nil {
				break
			}
		}

		i += size
		if isNewline(c) {
			line++
			col = 1
		} else {
			col++
		}
	}

	if done && i < len(in) && in[i] >= utf8.RuneSelf {
		// the rune path peeks the rune after the closing quote and fails on invalid UTF-8
		if c, _ := utf8.DecodeRune(in[i:]); c == utf8.RuneError {
			err = ErrInvalidRune
		}
	}

	start := len(s.raw) - len(in)
	s.peeked = s.peeked[:0]
	s.input = in[i:]
	s.len -= i
	s.line, s.column = line, col
	switch {
	case err != nil:
		return nil, err
	case !done && i < 2:
		return nil, ErrEndOfToken
	case !done:
		return nil, errUnterminatedString
	}
	return s.raw[start : start+i], nil
}
