package document

// Document is the top-level container for a KDL document
type Document struct {
	Nodes   []*Node
	Version int // 0=unknown, 1=KDLv1, 2=KDLv2; set by parser when detected
	retain  any
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

// Retain keeps parser-owned allocation state alive for the lifetime of the document.
func (d *Document) Retain(v any) {
	d.retain = v
}
