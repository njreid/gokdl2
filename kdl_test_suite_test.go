package kdl

import (
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/njreid/gokdl2/document"
)

type testSuiteNode struct {
	Type     *string                        `json:"type"`
	Name     string                         `json:"name"`
	Args     []testSuiteTypedValue          `json:"args"`
	Props    map[string]testSuiteTypedValue `json:"props"`
	Children []testSuiteNode                `json:"children"`
}

type testSuiteTypedValue struct {
	Type  *string        `json:"type"`
	Value testSuiteValue `json:"value"`
}

type testSuiteValue struct {
	Type  string  `json:"type"`
	Value *string `json:"value,omitempty"`
}

func TestKDLTestSuite(t *testing.T) {
	root := os.Getenv("KDL_TEST_SUITE_DIR")
	if root == "" {
		cwd, err := os.Getwd()
		if err != nil {
			t.Fatalf("Getwd() error = %v", err)
		}
		root = filepath.Join(filepath.Dir(cwd), "kdl-test", "test_cases")
	}

	if _, err := os.Stat(root); err != nil {
		t.Skipf("kdl-test suite not available at %s", root)
	}

	runValidKDLTestCases(t, filepath.Join(root, "valid"))
	runInvalidKDLTestCases(t, filepath.Join(root, "invalid"))
}

func runValidKDLTestCases(t *testing.T, dir string) {
	cases, err := filepath.Glob(filepath.Join(dir, "*.kdl"))
	if err != nil {
		t.Fatalf("Glob(valid) error = %v", err)
	}
	sort.Strings(cases)

	for _, path := range cases {
		path := path
		t.Run("valid/"+filepath.Base(path), func(t *testing.T) {
			input, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("ReadFile(%q) error = %v", path, err)
			}

			doc, err := ParseWithOptions(strings.NewReader(string(input)), ParseOptions{Version: ParseVersionV2})
			if err != nil {
				t.Fatalf("ParseWithOptions(%q) error = %v", path, err)
			}

			got := encodeTestSuiteDocument(doc)

			expectedPath := strings.TrimSuffix(path, ".kdl") + ".json"
			expectedBytes, err := os.ReadFile(expectedPath)
			if err != nil {
				t.Fatalf("ReadFile(%q) error = %v", expectedPath, err)
			}

			var want any
			if err := json.Unmarshal(expectedBytes, &want); err != nil {
				t.Fatalf("json.Unmarshal(%q) error = %v", expectedPath, err)
			}

			var have any
			encoded, err := json.Marshal(got)
			if err != nil {
				t.Fatalf("json.Marshal(%q) error = %v", path, err)
			}
			if err := json.Unmarshal(encoded, &have); err != nil {
				t.Fatalf("json.Unmarshal(got) error = %v", err)
			}

			if !reflect.DeepEqual(have, want) {
				t.Fatalf("kdl-test mismatch for %s\nwant: %s\n got: %s", filepath.Base(path), string(expectedBytes), string(encoded))
			}
		})
	}
}

func runInvalidKDLTestCases(t *testing.T, dir string) {
	cases, err := filepath.Glob(filepath.Join(dir, "*.kdl"))
	if err != nil {
		t.Fatalf("Glob(invalid) error = %v", err)
	}
	sort.Strings(cases)

	for _, path := range cases {
		path := path
		t.Run("invalid/"+filepath.Base(path), func(t *testing.T) {
			input, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("ReadFile(%q) error = %v", path, err)
			}

			if _, err := ParseWithOptions(strings.NewReader(string(input)), ParseOptions{Version: ParseVersionV2}); err == nil {
				t.Fatalf("ParseWithOptions(%q) error = nil, want invalid input rejection", path)
			}
		})
	}
}

func encodeTestSuiteDocument(doc *document.Document) []testSuiteNode {
	nodes := make([]testSuiteNode, 0, len(doc.Nodes))
	for _, node := range doc.Nodes {
		nodes = append(nodes, encodeTestSuiteNode(node))
	}
	return nodes
}

func encodeTestSuiteNode(node *document.Node) testSuiteNode {
	args := make([]testSuiteTypedValue, 0, len(node.Arguments))
	for _, arg := range node.Arguments {
		args = append(args, encodeTestSuiteTypedValue(arg))
	}

	props := make(map[string]testSuiteTypedValue, node.Properties.Len())
	for name, value := range node.Properties.Unordered() {
		props[name] = encodeTestSuiteTypedValue(value)
	}

	children := make([]testSuiteNode, 0, len(node.Children))
	for _, child := range node.Children {
		children = append(children, encodeTestSuiteNode(child))
	}

	return testSuiteNode{
		Type:     normalizeTypeAnnotation(string(node.Type)),
		Name:     node.Name.ResolvedValue().(string),
		Args:     args,
		Props:    props,
		Children: children,
	}
}

func encodeTestSuiteTypedValue(v *document.Value) testSuiteTypedValue {
	return testSuiteTypedValue{
		Type:  normalizeTypeAnnotation(string(v.Type)),
		Value: encodeTestSuiteValue(v),
	}
}

func encodeTestSuiteValue(v *document.Value) testSuiteValue {
	rv := v.Value
	switch x := rv.(type) {
	case nil:
		return testSuiteValue{Type: "null"}
	case bool:
		s := strconv.FormatBool(x)
		return testSuiteValue{Type: "boolean", Value: &s}
	case string:
		return testSuiteValue{Type: "string", Value: &x}
	case document.Expression:
		s := string(x)
		return testSuiteValue{Type: "string", Value: &s}
	case int:
		return numberValue(strconv.FormatInt(int64(x), 10) + ".0")
	case int8:
		return numberValue(strconv.FormatInt(int64(x), 10) + ".0")
	case int16:
		return numberValue(strconv.FormatInt(int64(x), 10) + ".0")
	case int32:
		return numberValue(strconv.FormatInt(int64(x), 10) + ".0")
	case int64:
		return numberValue(strconv.FormatInt(x, 10) + ".0")
	case uint:
		return numberValue(strconv.FormatUint(uint64(x), 10) + ".0")
	case uint8:
		return numberValue(strconv.FormatUint(uint64(x), 10) + ".0")
	case uint16:
		return numberValue(strconv.FormatUint(uint64(x), 10) + ".0")
	case uint32:
		return numberValue(strconv.FormatUint(uint64(x), 10) + ".0")
	case uint64:
		return numberValue(strconv.FormatUint(x, 10) + ".0")
	case float32:
		return numberValue(normalizeNumberString(formatFloat(float64(x))))
	case float64:
		return numberValue(normalizeNumberString(formatFloat(x)))
	case *big.Int:
		return numberValue(x.String() + ".0")
	case *big.Float:
		return numberValue(normalizeNumberString(x.Text('f', -1)))
	default:
		panic(fmt.Sprintf("unsupported kdl-test value type %T", rv))
	}
}

func formatFloat(f float64) string {
	switch {
	case math.IsInf(f, 1):
		return "inf"
	case math.IsInf(f, -1):
		return "-inf"
	case math.IsNaN(f):
		return "nan"
	default:
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
}

func numberValue(s string) testSuiteValue {
	return testSuiteValue{Type: "number", Value: &s}
}

func optionalString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func normalizeTypeAnnotation(s string) *string {
	if s == "" {
		return nil
	}
	if unquoted, err := document.UnquoteString(s); err == nil {
		return &unquoted
	}
	return &s
}

func normalizeNumberString(s string) string {
	if s == "inf" || s == "-inf" || s == "nan" {
		return s
	}
	if strings.ContainsAny(s, ".eE") {
		return s
	}
	return s + ".0"
}
