package document

// Expression is a backtick-delimited expression literal kept distinct from an ordinary string.
type Expression string

func (e Expression) String() string {
	return string(e)
}
