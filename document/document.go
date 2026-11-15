package document

// Document is the top-level container for a KDL document
type Document struct {
	Nodes   []*Node
	Version int // 0=unknown, 1=KDLv1, 2=KDLv2; set by parser when detected
}

// AddNode adds a Node to this document
func (d *Document) AddNode(child *Node) {
	d.Nodes = append(d.Nodes, child)
}

// New cerates a new Document
func New() *Document {
	return &Document{
		Nodes: make([]*Node, 0, 32),
	}
}

// Retain is kept for API compatibility and is a no-op: nodes and values
// allocated by an Allocator keep their backing chunks alive through ordinary
// interior pointers.
func (d *Document) Retain(v any) {}
