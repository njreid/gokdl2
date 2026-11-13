package kdl

import (
	"fmt"
	"strings"
	"testing"
)

// TestParseManyNodesAcrossChunks parses enough nodes to span several allocator
// chunks and verifies every node and argument is distinct and intact.
func TestParseManyNodesAcrossChunks(t *testing.T) {
	const n = 1000
	var sb strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&sb, "node%d %d \"s%d\" k=%d\n", i, i, i, i)
	}
	doc, err := Parse(strings.NewReader(sb.String()))
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Nodes) != n {
		t.Fatalf("got %d nodes, want %d", len(doc.Nodes), n)
	}
	for i, node := range doc.Nodes {
		if got := fmt.Sprint(node.Name.Value); got != fmt.Sprintf("node%d", i) {
			t.Fatalf("node %d name %q", i, got)
		}
		if len(node.Arguments) != 2 ||
			fmt.Sprint(node.Arguments[0].Value) != fmt.Sprint(i) ||
			fmt.Sprint(node.Arguments[1].Value) != fmt.Sprintf("s%d", i) {
			t.Fatalf("node %d arguments corrupted", i)
		}
	}
}
