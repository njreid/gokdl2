package document

import "testing"

func TestAllocatorChunkRollover(t *testing.T) {
	for _, est := range []int{0, 1, 5, allocNodeChunk, 1 << 20} {
		a := NewAllocator(est)
		const total = allocNodeChunk*3 + 7
		nodes := make([]*Node, total)
		vals := make([]*Value, total*3)
		for i := range nodes {
			n := a.NewNode()
			if n.Name != nil || n.Children != nil || len(n.Arguments) != 0 {
				t.Fatalf("est %d: node %d not zero", est, i)
			}
			n.Name = &Value{Value: i}
			nodes[i] = n
		}
		for i := range vals {
			v := a.NewValue()
			if v.Value != nil || v.Type != "" {
				t.Fatalf("est %d: value %d not zero", est, i)
			}
			v.Value = i
			vals[i] = v
		}
		seen := map[*Node]bool{}
		for i, n := range nodes {
			if seen[n] {
				t.Fatalf("est %d: duplicate node pointer at %d", est, i)
			}
			seen[n] = true
			if n.Name.Value != i {
				t.Fatalf("est %d: node %d corrupted: %v", est, i, n.Name.Value)
			}
		}
		vseen := map[*Value]bool{}
		for i, v := range vals {
			if vseen[v] {
				t.Fatalf("est %d: duplicate value pointer at %d", est, i)
			}
			vseen[v] = true
			if v.Value != i {
				t.Fatalf("est %d: value %d corrupted", est, i)
			}
		}
	}
}

func TestAllocatorNil(t *testing.T) {
	var a *Allocator
	if a.NewNode() == nil || a.NewValue() == nil {
		t.Fatal("nil allocator returned nil")
	}
}
