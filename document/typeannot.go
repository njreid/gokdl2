package document

import "github.com/njreid/gokdl2/internal/tokenizer"

// TypeAnnotationFromToken decodes a type annotation token (bare identifier, quoted string or raw string) into a
// TypeAnnotation, removing any quotes and decoding escapes.
func TypeAnnotationFromToken(t tokenizer.Token) (TypeAnnotation, error) {
	if t.ID == tokenizer.BareIdentifier {
		return TypeAnnotation(t.Data), nil
	}
	var v Value
	if err := ValueFromTokenInto(&v, t); err != nil {
		return "", err
	}
	s, _ := v.Value.(string)
	return TypeAnnotation(s), nil
}

// appendTypeAnnotationName appends the type annotation's name to b, quoted if it is not a valid bare identifier for
// the given version.
func appendTypeAnnotationName(b []byte, ta TypeAnnotation, version tokenizer.Version) []byte {
	if tokenizer.IsBareIdentifierVersion(string(ta), 0, version) {
		return append(b, ta...)
	}
	return AppendQuotedString(b, string(ta), '"')
}
