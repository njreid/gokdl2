package document

const (
	// allocNodeChunk is the number of Nodes allocated per chunk.
	allocNodeChunk = 128
	// allocValueChunk is the number of Values allocated per chunk.
	allocValueChunk = allocNodeChunk * 3
)

// Allocator hands out Nodes and Values from fixed-size chunks that are
// allocated on demand. Fresh chunks are already zeroed, so handed-out items
// need no further clearing. Interior pointers keep their chunk alive, so the
// allocator itself need not be retained by the documents it builds.
type Allocator struct {
	nodes  []Node
	values []Value
}

// NewAllocator creates an allocator. estimatedNodes is a hint used to size the
// first chunk; it is capped at one chunk, and later chunks are fixed-size.
func NewAllocator(estimatedNodes int) *Allocator {
	nn := allocNodeChunk
	if estimatedNodes < 1 {
		estimatedNodes = 1
	}
	if estimatedNodes < nn {
		nn = estimatedNodes
	}
	return &Allocator{
		nodes:  make([]Node, nn),
		values: make([]Value, nn*3),
	}
}

// NewNode returns a zeroed Node.
func (a *Allocator) NewNode() *Node {
	if a == nil {
		return &Node{}
	}
	if len(a.nodes) == 0 {
		a.nodes = make([]Node, allocNodeChunk)
	}
	n := &a.nodes[0]
	a.nodes = a.nodes[1:]
	return n
}

// NewValue returns a zeroed Value.
func (a *Allocator) NewValue() *Value {
	if a == nil {
		return &Value{}
	}
	if len(a.values) == 0 {
		a.values = make([]Value, allocValueChunk)
	}
	v := &a.values[0]
	a.values = a.values[1:]
	return v
}
