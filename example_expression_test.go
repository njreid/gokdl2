package kdl_test

import (
	"fmt"
	"strings"

	kdl "github.com/njreid/gokdl2"
	"github.com/njreid/gokdl2/document"
)

// Backtick expression strings parse to document.Expression, a string type distinct from ordinary KDL strings.
func Example_expressionStrings() {
	src := "rule (cel)`request.auth.claims.role == \"admin\"` deny=```\n" +
		"    request.path.startsWith(\"/admin\")\n" +
		"      && !request.auth.mfa\n" +
		"    ```\n"

	doc, err := kdl.Parse(strings.NewReader(src))
	if err != nil {
		panic(err)
	}

	arg := doc.Nodes[0].Arguments[0]
	expr := arg.Value.(document.Expression)
	fmt.Printf("%s %q\n", arg.Type, expr)

	deny, _ := doc.Nodes[0].Properties.Get("deny")
	fmt.Printf("%q\n", deny.Value.(document.Expression))

	// Output:
	// cel "request.auth.claims.role == \"admin\""
	// "request.path.startsWith(\"/admin\")\n  && !request.auth.mfa"
}

// Expression values are written back with backticks; values containing a newline use the triple-backtick form.
func Example_generateExpressions() {
	node := document.NewNode()
	node.SetName("rule")
	node.AddArgument(document.Expression("size(items) > 0"), "")
	node.AddProperty("when", document.Expression("a\n&& b"), "")

	fmt.Println(node.String())

	// Output:
	// rule `size(items) > 0` when=```
	// a
	// && b
	// ```
}

// Struct fields of type document.Expression marshal as expressions; expressions unmarshal into document.Expression,
// string, or interface{} fields.
func Example_marshalExpressions() {
	type Policy struct {
		Allow document.Expression `kdl:"allow"`
		Note  string              `kdl:"note"`
		Any   interface{}         `kdl:"any,omitempty"`
	}

	out, err := kdl.Marshal(Policy{Allow: "user.admin", Note: "plain text"})
	if err != nil {
		panic(err)
	}
	fmt.Print(string(out))

	var p Policy
	if err := kdl.Unmarshal([]byte("allow `user.admin`\nnote `as text`\nany `kept`\n"), &p); err != nil {
		panic(err)
	}
	fmt.Printf("%q %q %T\n", p.Allow, p.Note, p.Any)

	// Output:
	// allow `user.admin`
	// note "plain text"
	// "user.admin" "as text" document.Expression
}
