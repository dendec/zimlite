//go:build !cgo

package zim

import "github.com/kiwix-sdl/zimlite/internal/document"

type Reader struct {
	rootPrefix   string
	mainPagePath string
}

type ArticleEntry struct {
	Title string
	Path  string
}

func Open(path string) (*Reader, error) {
	return nil, nil
}

func (r *Reader) Close() {}

func (r *Reader) ArticleCount() int {
	return 0
}

func (r *Reader) ListArticles() []ArticleEntry {
	return nil
}

func (r *Reader) MainPage() (*document.Document, error) {
	return nil, nil
}

func (r *Reader) MainPagePath() string {
	return ""
}

func (r *Reader) GetArticle(path string) (*document.Document, error) {
	return nil, nil
}

func (r *Reader) ResolveArticle(rawURL string, referrer string) (*document.Document, error) {
	return nil, nil
}

func (r *Reader) GetResource(path string) ([]byte, string, error) {
	return nil, "", nil
}

func (r *Reader) ResolveResource(rawURL string) ([]byte, string, error) {
	return nil, "", nil
}
