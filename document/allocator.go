package document

// Allocator provides pooled Node and Value allocation for parser-driven document construction.
type Allocator struct {
	nodes  []Node
	values []Value
	ni     int
	vi     int
}

// NewAllocator creates an allocator sized for approximately estimatedNodes parsed nodes.
func NewAllocator(estimatedNodes int) *Allocator {
	if estimatedNodes < 1 {
		estimatedNodes = 1
	}
	return &Allocator{
		nodes:  make([]Node, estimatedNodes),
		values: make([]Value, estimatedNodes*2),
	}
}

func (a *Allocator) NewNode() *Node {
	if a == nil || a.ni >= len(a.nodes) {
		return &Node{}
	}
	n := &a.nodes[a.ni]
	a.ni++
	*n = Node{}
	return n
}

func (a *Allocator) NewValue() *Value {
	if a == nil || a.vi >= len(a.values) {
		return &Value{}
	}
	v := &a.values[a.vi]
	a.vi++
	*v = Value{}
	return v
}
