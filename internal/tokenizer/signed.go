package tokenizer

// signedPrefix returns the radix prefix character ('x', 'o' or 'b') following a sign and a '0' at the current
// position (eg: the 'x' in -0xff), or 0 if the input is not a signed prefixed number.
func (s *Scanner) signedPrefix() rune {
	c, err := s.peekAt(2)
	if err != nil {
		return 0
	}
	switch c {
	case 'x', 'o', 'b':
		return c
	}
	return 0
}

// isSignedPrefixedNumber reports whether the input at the current position looks like a signed hexadecimal, octal or
// binary number, assuming the first character is a sign and the second is '0'.
func (s *Scanner) isSignedPrefixedNumber() bool {
	return s.signedPrefix() != 0
}

// readSignedPrefixedNumber reads a signed hexadecimal, octal or binary number into token.
func (s *Scanner) readSignedPrefixedNumber(token *Token) (err error) {
	switch s.signedPrefix() {
	case 'x':
		token.ID = Hexadecimal
		token.Data, err = s.readHexadecimal(3)
	case 'o':
		token.ID = Octal
		token.Data, err = s.readOctal(3)
	case 'b':
		token.ID = Binary
		token.Data, err = s.readBinary(3)
	}
	return err
}
