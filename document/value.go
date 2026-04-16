package document

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
	"unsafe"

	"github.com/njreid/gokdl2/internal/tokenizer"
)

// ValueFlag represents flags for a Value
type ValueFlag uint16

const (
	// FlagNone indicates no flag is set
	FlagNone ValueFlag = 0
	// FlagRaw specifies that this Value should be output in RawString notation (r"foo\n")
	FlagRaw ValueFlag = 1 << iota
	// FlagQuoted specifies that this Value should be output in FormattedString notation ("foo\\n")
	FlagQuoted
	// FlagBinary specifies that this Value should be output in binary notation (0b10101010)
	FlagBinary
	// FlagOctal specifies that this Value should be output in octal notation (0o751)
	FlagOctal
	// FlagHexadecimal specifies that this Value should be output in hexadecimal notation (0xdeadbeef)
	FlagHexadecimal
	// FlagSuffixed specifies that this value is a suffixed number
	FlagBareSuffixed
	// FlagMultiLine specifies that this Value was parsed from a KDL v2 multi-line string ("""...""")
	FlagMultiLine
	// FlagBare specifies that this Value was parsed as a KDL v2 bare identifier
	FlagBare
	// FlagExpression specifies that this Value should be output as a backtick-delimited expression.
	FlagExpression
)

func (f ValueFlag) Has(flag ValueFlag) bool {
	return f&flag != 0
}

// Value represents a value in a KDL document
type Value struct {
	// Type is the value type, if an annotation was provided
	Type TypeAnnotation
	// Value is the actual value
	Value interface{}
	// Flag is any flag assigned for use in output
	Flag ValueFlag
	// RawHashes indicates the number of '#' characters used in the raw string representation
	RawHashes int
}

// valueOpts specify options for rendering Values as strings
type valueOpts int

const (
	// if a string was originally quoted or raw, output it quoted; if bare, output bare if possible, otherwise quoted (default)
	voTranslateStringFlags valueOpts = 0
	// if a numeric value was originally in octal, binary, or hex representation, output it the same way
	voUseNumericFlags valueOpts = 1 << iota
	// if a string was originally in raw, quoted, or bare representation, try to output it the same way with fallback to quoted
	voStrictStringFlags
	// strings are output bare if possible, otherwise quoted
	voSimpleString
	// force unquoted, bare output regardless of the string's original representation
	voNoQuotes
	// force quoted or raw representation of strings
	voNoBare
	// output KDL v2 syntax
	voVersionV2
)

// AppendTo appends the simple string representation of this Value to b using decimal numbers, and returns the expanded
// buffer.
func (v *Value) AppendTo(b []byte) []byte {
	return v.value(b, voSimpleString)
}

// value appends the string representation of this Value to b using the specified opts, and returns the expanded buffer
func (v *Value) value(b []byte, opts valueOpts) []byte {
	haveOpt := func(opt valueOpts) bool {
		return (opts & opt) != 0
	}
	if v.Value == nil {
		if haveOpt(voVersionV2) {
			return append(b, "#null"...)
		}
		return append(b, "null"...)
	}

	base := 10
	prefix := ""
	if haveOpt(voUseNumericFlags) {
		switch {
		case v.Flag.Has(FlagBinary):
			base = 2
			prefix = "0b"
			if b == nil {
				b = make([]byte, 0, 10)
			}
		case v.Flag.Has(FlagOctal):
			base = 8
			prefix = "0o"
			if b == nil {
				b = make([]byte, 0, 10)
			}
		case v.Flag.Has(FlagHexadecimal):
			base = 16
			prefix = "0x"
			b = make([]byte, 0, 18)
		}
	}

	switch x := v.Value.(type) {
	case uint:
		if base != 10 {
			b = append(b, prefix...)
		}
		b = strconv.AppendUint(b, uint64(x), base)
	case uint8:
		if base != 10 {
			b = append(b, prefix...)
		}
		b = strconv.AppendUint(b, uint64(x), base)
	case uint16:
		if base != 10 {
			b = append(b, prefix...)
		}
		b = strconv.AppendUint(b, uint64(x), base)
	case uint32:
		if base != 10 {
			b = append(b, prefix...)
		}
		b = strconv.AppendUint(b, uint64(x), base)
	case uint64:
		if base != 10 {
			b = append(b, prefix...)
		}
		b = strconv.AppendUint(b, x, base)
	case uintptr:
		if base != 10 {
			b = append(b, prefix...)
		}
		b = strconv.AppendUint(b, uint64(x), base)
	case int:
		if base != 10 {
			b = append(b, prefix...)
		}
		b = strconv.AppendInt(b, int64(x), base)
	case int8:
		if base != 10 {
			b = append(b, prefix...)
		}
		b = strconv.AppendInt(b, int64(x), base)
	case int16:
		if base != 10 {
			b = append(b, prefix...)
		}
		b = strconv.AppendInt(b, int64(x), base)
	case int32:
		if base != 10 {
			b = append(b, prefix...)
		}
		b = strconv.AppendInt(b, int64(x), base)
	case int64:
		if base != 10 {
			b = append(b, prefix...)
		}
		b = strconv.AppendInt(b, x, base)
	case float32:
		if haveOpt(voVersionV2) {
			if math.IsInf(float64(x), 1) {
				b = append(b, "#inf"...)
				return b
			} else if math.IsInf(float64(x), -1) {
				b = append(b, "#-inf"...)
				return b
			} else if math.IsNaN(float64(x)) {
				b = append(b, "#nan"...)
				return b
			}
		}
		l10 := math.Log10(math.Abs(float64(x)))
		if !math.IsInf(l10, 0) && (l10 > 9 || l10 < -9) {
			b = strconv.AppendFloat(b, float64(x), 'E', -1, 32)
		} else {
			// make sure floats in decimal notation always include a decimal point
			b = strconv.AppendFloat(b, float64(x), 'f', -1, 32)
			if _, frac := math.Modf(float64(x)); frac == 0.0 {
				b = append(b, '.', '0')
			}
		}
	case float64:
		if haveOpt(voVersionV2) {
			if math.IsInf(x, 1) {
				b = append(b, "#inf"...)
				return b
			} else if math.IsInf(x, -1) {
				b = append(b, "#-inf"...)
				return b
			} else if math.IsNaN(x) {
				b = append(b, "#nan"...)
				return b
			}
		}
		l10 := math.Log10(math.Abs(x))
		if !math.IsInf(l10, 0) && (l10 > 9 || l10 < -9) {
			b = strconv.AppendFloat(b, x, 'E', -1, 64)
		} else {
			// make sure floats in decimal notation always include a decimal point
			b = strconv.AppendFloat(b, x, 'f', -1, 64)
			if _, frac := math.Modf(float64(x)); frac == 0.0 {
				b = append(b, '.', '0')
			}
		}
	case bool:
		if haveOpt(voVersionV2) {
			if x {
				b = append(b, "#true"...)
			} else {
				b = append(b, "#false"...)
			}
		} else {
			b = strconv.AppendBool(b, x)
		}
	case string:

		version := tokenizer.VersionV1
		if haveOpt(voVersionV2) {
			version = tokenizer.VersionV2
		}
		isBare := tokenizer.IsBareIdentifierVersion(x, 0, version)

		if b == nil {
			size := len(x)
			if !isBare {
				size += 16
			}
			b = make([]byte, 0, size)
		}

		if v.Flag.Has(FlagBareSuffixed) || (v.Flag.Has(FlagBare) && haveOpt(voVersionV2)) || (!haveOpt(voNoBare) && (haveOpt(voNoQuotes) || (isBare && haveOpt(voSimpleString)))) {
			b = append(b, x...)
		} else {
			hashes := v.RawHashes
			if hashes == 0 {
				hashes = -1
			}

			if v.Flag.Has(FlagMultiLine) && haveOpt(voStrictStringFlags) {
				if v.Flag.Has(FlagRaw) && haveOpt(voVersionV2) {
					b = AppendMultiLineStringV2(b, x, hashes)
				} else {
					b = AppendMultiLineString(b, x)
				}
			} else if v.Flag.Has(FlagRaw) && haveOpt(voStrictStringFlags) {
				if haveOpt(voVersionV2) {
					b = AppendRawStringV2(b, x, hashes)
				} else {
					b = AppendRawString(b, x, hashes)
				}
			} else if v.Flag.Has(FlagQuoted) || (v.Flag.Has(FlagRaw) && !haveOpt(voStrictStringFlags)) || (v.Flag.Has(FlagMultiLine) && !haveOpt(voStrictStringFlags)) {
				b = AppendQuotedString(b, x, '"')
			} else if isBare && !haveOpt(voNoBare) {
				b = append(b, x...)
			} else {
				b = AppendQuotedString(b, x, '"')
			}
		}

	case Expression:
		if b == nil {
			size := len(x)
			if !haveOpt(voNoQuotes) {
				size += 16
			}
			b = make([]byte, 0, size)
		}

		if haveOpt(voNoQuotes) {
			b = append(b, string(x)...)
		} else if v.Flag.Has(FlagMultiLine) || strings.ContainsRune(string(x), '\n') {
			b = AppendMultiLineExpressionString(b, string(x))
		} else {
			b = AppendQuotedString(b, string(x), '`')
		}

	case *big.Int:
		b = x.Append(b, base)
	case *big.Float:
		exp := x.MantExp(nil)
		if exp > 9 || exp < -9 {
			b = x.Append(b, 'E', -1)
		} else {
			b = x.Append(b, 'f', 6)
		}

	case SuffixedDecimal:
		b = append(b, x.Number...)
		b = append(b, x.Suffix...)

	default:
		formatted := fmt.Sprintf("%v", x)
		if haveOpt(voNoQuotes) {
			b = append(b, formatted...)
		} else {
			b = strconv.AppendQuote(b, formatted)
		}
	}
	return b
}

// string returns the KDL representation of this value with the specified opts, including type annotation if available,
// eg: (u8)1234
func (v *Value) string(opts valueOpts, version tokenizer.Version) string {
	if version == tokenizer.VersionV2 {
		opts |= voVersionV2
	}
	var b []byte
	if len(v.Type) > 0 {
		b = make([]byte, 0, 32)
		b = append(b, '(')
		b = append(b, v.Type...)
		b = append(b, ')')
	}

	b = v.value(b, opts)
	return string(b)
}

// String returns the KDL representation of this Value, including type annotation, formatting numbers and strings per
// their Flags.
//
// This returns the exact input KDL (if any) that was used to generate this Value.
func (v *Value) String() string {
	return v.string(voStrictStringFlags|voUseNumericFlags, tokenizer.VersionV1)
}

// FormattedString is similar to String, but bare strings are converted to quoted strings.
//
// This is suitable for returning arguments and property values while preserving their original formatting.
func (v *Value) FormattedString() string {
	return v.string(voNoBare|voUseNumericFlags, tokenizer.VersionV1)
}

// UnformattedString is similar to String, but bare strings are converted to quoted strings and numbers are formatted
// in decimal notation.
//
// This is suitable for returning arguments and property values while ignoring their original formatting.
func (v *Value) UnformattedString() string {
	return v.string(voNoBare, tokenizer.VersionV1)
}

// NodeNameString returns the simplest possible KDL representation of this Value, including type annotation, formatting
// numbers in decimal notation and strings as bare strings if possible, otherwise quoted.
//
// This is suitable for returning a valid node name.
func (v *Value) NodeNameString() string {
	return v.string(voSimpleString, tokenizer.VersionV1)
}

// StringV2 is similar to String, but outputs KDL v2 syntax.
func (v *Value) StringV2() string {
	return v.string(voStrictStringFlags|voUseNumericFlags, tokenizer.VersionV2)
}

// FormattedStringV2 is similar to FormattedString, but outputs KDL v2 syntax.
func (v *Value) FormattedStringV2() string {
	return v.string(voNoBare|voUseNumericFlags|voStrictStringFlags, tokenizer.VersionV2)
}

// UnformattedStringV2 is similar to UnformattedString, but outputs KDL v2 syntax.
func (v *Value) UnformattedStringV2() string {
	return v.string(voSimpleString, tokenizer.VersionV2)
}

// NodeNameStringV2 is similar to NodeNameString, but outputs KDL v2 syntax.
func (v *Value) NodeNameStringV2() string {
	return v.string(voSimpleString, tokenizer.VersionV2)
}

// ValueString returns the unquoted, unescaped, un-type-hinted representation of this Value; numbers are formatted per
// their Flags, strings are always unquoted.
//
// This is suitable for passing as a []byte value to UnmarshalText.
func (v *Value) ValueString() string {
	b := make([]byte, 0, 32)
	return string(v.value(b, voNoQuotes|voUseNumericFlags))
}

// ResolvedValue returns the unquoted, unescaped, un-type-hinted Go representation of this value via an interface{}:
// - numbers are returned as the appropriate numeric type (int64, float64, *big.Int, *big.Float, etc),
// - bools are returned as a bool
// - nulls are returned as nil
// - strings are returned as strings containing the unquoted representation of the string
func (v *Value) ResolvedValue() interface{} {
	if _, ok := v.Value.(Expression); ok {
		return v.Value
	}
	if _, ok := v.Value.(string); ok {
		return v.string(voNoQuotes, tokenizer.VersionV1)
	} else {
		return v.Value
	}
}

// ResolvedValueV2 is similar to ResolvedValue, but outputs KDL v2 syntax for strings.
func (v *Value) ResolvedValueV2() interface{} {
	if _, ok := v.Value.(Expression); ok {
		return v.Value
	}
	if _, ok := v.Value.(string); ok {
		return v.string(voNoQuotes, tokenizer.VersionV2)
	} else {
		return v.Value
	}
}

// isNonzeroSciNot returns true if b contains a string representation of a number in scientific notation with a nonzero
// coefficient.
func isNonzeroSciNot(b []byte) bool {
	coe, _, ok := bytes.Cut(b, []byte{'e'})
	if !ok {
		coe, _, ok = bytes.Cut(b, []byte{'E'})
	}
	if ok {
		coe = bytes.Trim(coe, "0")
		return len(coe) > 0 && !(len(coe) == 1 && coe[0] == '.')
	}
	return false
}

func bytesToString(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	return *(*string)(unsafe.Pointer(&b))
}

func classifyNumber(b []byte, base int) (underscoreCount int, isFloat bool) {
	for _, c := range b {
		switch c {
		case '_':
			underscoreCount++
		case '.':
			isFloat = true
		case 'e', 'E':
			if base == 10 {
				isFloat = true
			}
		}
	}
	return underscoreCount, isFloat
}

func stripUnderscores(b []byte, underscoreCount int) []byte {
	if underscoreCount == 0 {
		return b
	}

	clean := make([]byte, 0, len(b)-underscoreCount)
	for _, c := range b {
		if c != '_' {
			clean = append(clean, c)
		}
	}
	return clean
}

// parseNumber parses a number from b in the specified base, and returns an interface{} containing either a float64,
// an int64, a *big.Float, or a *big.Int, depending on the size and type of the number in b
func parseNumber(b []byte, base int) (interface{}, error) {
	if base != 10 {
		b = b[2:] // strip 0x, 0o, 0b
	}
	underscoreCount, float := classifyNumber(b, base)
	b = stripUnderscores(b, underscoreCount)
	bstr := bytesToString(b)

	var (
		v   interface{}
		err error
	)
	if float {
		if base != 10 {
			return nil, fmt.Errorf("parsing number %s: floating point numbers must be base 10 only", bstr)
		}

		var f float64
		f, err = strconv.ParseFloat(bstr, 64)

		// ParseFloat doesn't seem to generate ErrRange for tiny numbers in scientific notation (eg: 1.23E-1000); it
		// just returns 0, which is wrong. So if ParseFloat returns 0.0 and b contains a nonzero coefficient, we reparse
		// as a big.Float.
		if errors.Is(err, strconv.ErrRange) || (err == nil && f == 0.0 && isNonzeroSciNot(b)) {
			err = nil
			n := big.NewFloat(0)
			n.SetString(bstr)
			v = n
		} else {
			v = f
		}

	} else {
		v, err = strconv.ParseInt(bstr, base, 64)
		if errors.Is(err, strconv.ErrRange) {
			err = nil
			n := big.NewInt(0)
			n.SetString(bstr, base)
			v = n
		}
	}

	if err != nil {
		err = fmt.Errorf("parsing number %s: %w", bstr, err)
	}
	return v, err

}

// unquoteQuotedTokenString parses a quoted KDL string token and returns the unquoted string.
func unquoteQuotedTokenString(b []byte) (string, error) {
	v, err := UnquoteString(bytesToString(b))
	if err != nil {
		err = fmt.Errorf("parsing quoted string %s: %w", bytesToString(b), err)
	}
	return v, err
}

// unquoteRawTokenString parses a raw KDL string token and returns the unquoted string plus its hash count.
func unquoteRawTokenString(b []byte) (string, int, error) {
	// the tokenizer has already validated the string format, so we can safely just use byte offsets
	p := bytes.IndexByte(b, '"')
	if p == -1 {
		return "", 0, fmt.Errorf("invalid raw string: missing opening quote")
	}
	hashCount := p
	if p > 0 && b[0] == 'r' {
		hashCount--
	}

	content := b[p+1:]
	content = content[:len(content)-1-hashCount]
	return string(content), hashCount, nil
}

func isMultilineStringToken(s string) bool {
	if strings.HasPrefix(s, `"""`) {
		return true
	}
	if !strings.HasPrefix(s, "#") {
		return false
	}
	hashCount := 0
	for hashCount < len(s) && s[hashCount] == '#' {
		hashCount++
	}
	return strings.HasPrefix(s[hashCount:], `"""`)
}

func isMultilineExpressionToken(s string) bool {
	return strings.HasPrefix(s, "```")
}

func keywordValue(data []byte) (interface{}, error) {
	switch bytesToString(data) {
	case "#inf":
		return math.Inf(1), nil
	case "#-inf":
		return math.Inf(-1), nil
	case "#nan":
		return math.NaN(), nil
	default:
		return nil, fmt.Errorf("unknown keyword: %s", string(data))
	}
}

// ValueFromToken creates and returns a Value representing the content of t, or a non-nil error on failure
func ValueFromToken(t tokenizer.Token) (*Value, error) {
	v := &Value{}
	if err := ValueFromTokenInto(v, t); err != nil {
		return nil, err
	}
	return v, nil
}

// ValueFromTokenInto decodes t into v.
func ValueFromTokenInto(v *Value, t tokenizer.Token) error {
	*v = Value{}
	var err error
	switch t.ID {
	case tokenizer.QuotedString:
		s := bytesToString(t.Data)
		if isMultilineStringToken(s) {
			v.Value, v.RawHashes, err = UnquoteMultiLineString(s)
			v.Flag = FlagMultiLine
			// if it had hashes, it's also a raw string
			if strings.HasPrefix(s, "#") {
				v.Flag |= FlagRaw
			}
		} else {
			v.Value, err = unquoteQuotedTokenString(t.Data)
			v.Flag = FlagQuoted
		}
	case tokenizer.ExpressionString:
		s := bytesToString(t.Data)
		if isMultilineExpressionToken(s) {
			var expr string
			expr, err = UnquoteMultiLineExpressionString(s)
			v.Value = Expression(expr)
			v.Flag = FlagExpression | FlagMultiLine
		} else {
			var expr string
			expr, err = UnquoteString(s)
			v.Value = Expression(expr)
			v.Flag = FlagExpression
		}
	case tokenizer.BareIdentifier:
		v.Value = bytesToString(t.Data)
		v.Flag = FlagBare
	case tokenizer.Binary:
		v.Value, err = parseNumber(t.Data, 2)
		v.Flag = FlagBinary
	case tokenizer.RawString:
		v.Value, v.RawHashes, err = unquoteRawTokenString(t.Data)
		v.Flag = FlagRaw
	case tokenizer.Decimal:
		v.Value, err = parseNumber(t.Data, 10)
	case tokenizer.SuffixedDecimal:
		v.Value, err = ParseSuffixedDecimal(t.Data)
	case tokenizer.Octal:
		v.Value, err = parseNumber(t.Data, 8)
		v.Flag = FlagOctal
	case tokenizer.Hexadecimal:
		v.Value, err = parseNumber(t.Data, 16)
		v.Flag = FlagHexadecimal
	case tokenizer.Boolean:
		s := string(t.Data)
		v.Value = s == "true" || s == "#true"
	case tokenizer.Null:
		v.Value = nil
	case tokenizer.Keyword:
		v.Value, err = keywordValue(t.Data)
	}
	if err != nil {
		err = fmt.Errorf("value from token: %w", err)
	}

	return err
}
