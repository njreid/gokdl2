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
	// Comment is comment text between the previous entry (or the node name and type) and this value, kept when
	// parsing with ParseComments. It is written before the value with one space after it; for a property, before
	// "key=". It may hold block comments and slash-dashed arguments or properties, and, for a // comment on a line
	// that is continued with a backslash, the backslash and the newline it needs. Comments after the last entry of a
	// node are Comment.Trailing of the node instead. Only the generator reads it, and only when writing a document.
	Comment []byte
	// RawHashes indicates the number of '#' characters used in the raw string representation
	RawHashes int
	// Flag is any flag assigned for use in output
	Flag ValueFlag
	// EmptyType is true when an explicitly empty type annotation, eg `("")1`, was present
	EmptyType bool
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
			if b == nil {
				b = make([]byte, 0, 18)
			}
		}
	}

	switch x := v.Value.(type) {
	case uint:
		b = appendUnsigned(b, uint64(x), base, prefix)
	case uint8:
		b = appendUnsigned(b, uint64(x), base, prefix)
	case uint16:
		b = appendUnsigned(b, uint64(x), base, prefix)
	case uint32:
		b = appendUnsigned(b, uint64(x), base, prefix)
	case uint64:
		b = appendUnsigned(b, x, base, prefix)
	case uintptr:
		b = appendUnsigned(b, uint64(x), base, prefix)
	case int:
		b = appendSigned(b, int64(x), base, prefix)
	case int8:
		b = appendSigned(b, int64(x), base, prefix)
	case int16:
		b = appendSigned(b, int64(x), base, prefix)
	case int32:
		b = appendSigned(b, int64(x), base, prefix)
	case int64:
		b = appendSigned(b, x, base, prefix)
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
		// the tokenizer accepts some strings as identifiers that contain characters that may not appear literally
		isBare := tokenizer.IsBareIdentifierVersion(x, 0, version) && (version != tokenizer.VersionV2 || canLiteralString(x))

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

			if v.Flag.Has(FlagMultiLine) && haveOpt(voStrictStringFlags) && haveOpt(voVersionV2) {
				// a raw multi-line string cannot represent every value, so fall back to an escaped multi-line string
				if v.Flag.Has(FlagRaw) && canRawMultiLine(x) {
					b = AppendMultiLineStringV2(b, x, hashes)
				} else {
					b = AppendMultiLineString(b, x)
				}
			} else if v.Flag.Has(FlagRaw) && haveOpt(voStrictStringFlags) && (!haveOpt(voVersionV2) || canRawString(x)) {
				if haveOpt(voVersionV2) {
					b = AppendRawStringV2(b, x, hashes)
				} else {
					b = AppendRawString(b, x, hashes)
				}
			} else if v.Flag.Has(FlagQuoted) || v.Flag.Has(FlagRaw) || v.Flag.Has(FlagMultiLine) {
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
		if base != 10 {
			if x.Sign() < 0 {
				b = append(b, '-')
			}
			b = append(b, prefix...)
			b = new(big.Int).Abs(x).Append(b, base)
		} else {
			b = x.Append(b, base)
		}
	case *big.Float:
		exp := x.MantExp(nil)
		switch {
		case x.IsInf():
			// an overflowed big.Float is written like a float64 infinity
			return (&Value{Value: math.Inf(x.Sign())}).value(b, opts)
		case x.Sign() == 0:
			// an underflowed big.Float is written like a float64 zero
			if x.Signbit() {
				b = append(b, '-')
			}
			b = append(b, "0.0"...)
		case exp > bigFloatDirectBinExp || exp < -bigFloatDirectBinExp:
			b = appendHugeBigFloat(b, x)
		case exp > 9 || exp < -9:
			b = x.Append(b, 'E', -1)
		default:
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

// appendUnsigned appends x in the given base, with prefix when base is not 10.
func appendUnsigned(b []byte, x uint64, base int, prefix string) []byte {
	if base != 10 {
		b = append(b, prefix...)
	}
	return strconv.AppendUint(b, x, base)
}

// appendSigned appends x in the given base. For bases other than 10 the sign precedes the prefix (-0xff).
func appendSigned(b []byte, x int64, base int, prefix string) []byte {
	if base == 10 {
		return strconv.AppendInt(b, x, base)
	}
	var u uint64
	if x < 0 {
		b = append(b, '-')
		u = uint64(-x) // correct for MinInt64 as well
	} else {
		u = uint64(x)
	}
	b = append(b, prefix...)
	return strconv.AppendUint(b, u, base)
}

// string returns the KDL representation of this value with the specified opts, including type annotation if available,
// eg: (u8)1234
func (v *Value) string(opts valueOpts, version tokenizer.Version) string {
	var b []byte
	if len(v.Type) > 0 || v.EmptyType {
		b = make([]byte, 0, 32)
	}
	return string(v.appendString(b, opts, version))
}

// appendString appends the KDL representation of this value with the specified opts, including type annotation if
// available, to b and returns the expanded buffer.
func (v *Value) appendString(b []byte, opts valueOpts, version tokenizer.Version) []byte {
	if version == tokenizer.VersionV2 {
		opts |= voVersionV2
	}
	if len(v.Type) > 0 || v.EmptyType {
		b = append(b, '(')
		b = appendTypeAnnotationName(b, v.Type, version)
		b = append(b, ')')
	}
	return v.value(b, opts)
}

// argValueOpts returns the valueOpts used to render arguments and property values.
func argValueOpts(unformatted bool, version tokenizer.Version) valueOpts {
	if version == tokenizer.VersionV2 {
		if unformatted {
			return voSimpleString
		}
		return voNoBare | voUseNumericFlags | voStrictStringFlags
	}
	if unformatted {
		return voNoBare
	}
	return voNoBare | voUseNumericFlags
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

// bigFloatDirectExp is the largest decimal exponent magnitude parsed directly with big.Float. math/big's decimal
// conversion slows down enormously for huge exponents (1e-1000000 takes minutes), so beyond this the value is built
// from the mantissa and a power of ten computed by repeated squaring.
const bigFloatDirectExp = 1000

// bigFloatMaxDecimalExp bounds decimal exponents that a big.Float can represent (its binary exponent is an int32);
// anything larger overflows to ±Inf or underflows to ±0.
const bigFloatMaxDecimalExp = 646_000_000

// bigFloatDirectBinExp is the largest binary exponent formatted directly by big.Float, whose exact decimal
// conversion takes time proportional to the exponent. It corresponds roughly to bigFloatDirectExp.
const bigFloatDirectBinExp = 3400

// pow10BigFloat returns 10^n (n >= 0) at the given precision, by repeated squaring.
func pow10BigFloat(n int64, prec uint) *big.Float {
	pow := new(big.Float).SetPrec(prec).SetInt64(1)
	base := new(big.Float).SetPrec(prec).SetInt64(10)
	for ; n > 0; n >>= 1 {
		if n&1 == 1 {
			pow.Mul(pow, base)
		}
		base.Mul(base, base)
	}
	return pow
}

// appendHugeBigFloat appends x, a finite non-zero big.Float with a very large exponent, in E notation, in time that
// does not depend on the exponent: it scales x into [1, 10) by a power of ten and formats that mantissa as a float64.
func appendHugeBigFloat(b []byte, x *big.Float) []byte {
	const prec = 128
	abs := new(big.Float).SetPrec(prec).Abs(x)
	// estimate the decimal exponent from the binary one, then correct it after scaling
	e10 := int64(math.Floor(float64(abs.MantExp(nil)-1) * math.Log10(2)))
	scaled := new(big.Float).SetPrec(prec)
	for i := 0; i < 3; i++ {
		if e10 >= 0 {
			scaled.Quo(abs, pow10BigFloat(e10, prec))
		} else {
			scaled.Mul(abs, pow10BigFloat(-e10, prec))
		}
		m, _ := scaled.Float64()
		switch {
		case m >= 10:
			e10++
		case m < 1:
			e10--
		default:
			i = 3
		}
	}
	m, _ := scaled.Float64()
	if m >= 10 {
		// rounding pushed the mantissa to 10
		m /= 10
		e10++
	}
	digits := strconv.FormatFloat(m, 'f', -1, 64)
	if x.Sign() < 0 {
		b = append(b, '-')
	}
	b = append(b, digits...)
	b = append(b, 'E')
	if e10 >= 0 {
		b = append(b, '+')
	}
	return strconv.AppendInt(b, e10, 10)
}

// parseBigFloat parses a base-10 floating point string that is outside the float64 range, at 53 bits of precision,
// in time that does not depend on the size of the exponent.
func parseBigFloat(s string) (*big.Float, error) {
	mant, exp := s, int64(0)
	if i := strings.IndexAny(s, "eE"); i >= 0 {
		mant = s[:i]
		e, err := strconv.ParseInt(s[i+1:], 10, 64)
		if err != nil {
			// the exponent itself does not fit in an int64: certainly out of range
			e = math.MaxInt64
			if strings.HasPrefix(s[i+1:], "-") {
				e = math.MinInt64
			}
		}
		exp = e
	}
	if exp >= -bigFloatDirectExp && exp <= bigFloatDirectExp {
		n := big.NewFloat(0)
		if _, ok := n.SetString(s); !ok {
			return nil, fmt.Errorf("invalid number %s", s)
		}
		return n, nil
	}

	m, ok := new(big.Float).SetPrec(128).SetString(mant)
	if !ok {
		return nil, fmt.Errorf("invalid number %s", s)
	}
	result := new(big.Float).SetPrec(53)
	switch {
	case m.Sign() == 0:
		return result.Set(m), nil
	case exp > bigFloatMaxDecimalExp:
		return result.SetInf(m.Sign() < 0), nil
	case exp < -bigFloatMaxDecimalExp:
		if m.Sign() < 0 {
			return result.Neg(result), nil
		}
		return result, nil
	}

	// 10^|exp| by repeated squaring, at 128 bits so the final rounding to 53 bits is exact in practice
	abs := exp
	if abs < 0 {
		abs = -abs
	}
	pow := pow10BigFloat(abs, 128)
	if exp > 0 {
		m.Mul(m, pow)
	} else {
		m.Quo(m, pow)
	}
	return result.Set(m), nil
}

// parseNumber parses a number from b in the specified base, and returns an interface{} containing either a float64,
// an int64, a *big.Float, or a *big.Int, depending on the size and type of the number in b
func parseNumber(b []byte, base int) (interface{}, error) {
	var sign []byte
	if base != 10 {
		if len(b) > 0 && (b[0] == '+' || b[0] == '-') {
			if b[0] == '-' {
				sign = b[:1]
			}
			b = b[1:]
		}
		b = b[2:] // strip 0x, 0o, 0b
	}
	underscoreCount, float := classifyNumber(b, base)
	b = stripUnderscores(b, underscoreCount)
	bstr := bytesToString(b)
	if sign != nil {
		bstr = "-" + bstr
	}

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
			v, err = parseBigFloat(bstr)
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
