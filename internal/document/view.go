package document

// ResourceLoader resolves an image URL relative to the current document.
type ResourceLoader func(url string) ([]byte, error)

// TreeItem is the presentation data for one archive navigation row.
// It contains no renderer or input implementation details.
type TreeItem struct {
	Text       string
	Path       string
	IsLeaf     bool
	IsCursor   bool
	LabelStart int // rune offset where label begins in Text
	LabelEnd   int // exclusive rune offset where label ends in Text
}
