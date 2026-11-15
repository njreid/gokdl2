package document

// Expression is the value of a backtick-delimited expression string, a gokdl2 extension to KDL for embedding code
// such as CEL, JMESPath or SQL fragments:
//
//	rule `request.auth.claims.role == "admin"`
//	deny ```
//	    request.path.startsWith("/admin")
//	      && !request.auth.mfa
//	    ```
//
// It is a distinct type from string so that callers can tell an expression from plain text. Expressions may appear
// wherever a value may (arguments and property values, with an optional type annotation), but not as node names or
// property keys. Single-line expressions use the same escapes as KDL quoted strings, with \` for a backtick; double
// quotes need no escaping. Triple-backtick expressions span lines and are dedented by the indentation of the closing
// delimiter, like KDL v2 multi-line strings.
//
// Parsed values have FlagExpression set, plus FlagMultiLine for the triple-backtick form. When generating, an
// Expression is written with backticks, using the triple-backtick form if it contains a newline.
//
// Expression strings are not part of the KDL specification, so other KDL implementations will reject documents that
// contain them.
type Expression string

func (e Expression) String() string {
	return string(e)
}
