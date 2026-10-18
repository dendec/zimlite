package article

import (
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestServerSessionAndRelativeResources(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<link href='style.css' rel='stylesheet'>"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "style.css"), []byte("body{color:red}"), 0600); err != nil {
		t.Fatal(err)
	}
	source, entry, err := OpenFileSource(filepath.Join(dir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = source.Close() }()
	server, err := StartServer(source)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Jar: jar}
	get := func(client *http.Client, address string, status int) http.Header {
		t.Helper()
		resp, err := client.Get(address)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = resp.Body.Close() }()
		_, _ = io.Copy(io.Discard, resp.Body)
		if resp.StatusCode != status {
			t.Fatalf("%s: status %d, want %d", address, resp.StatusCode, status)
		}
		return resp.Header
	}
	page, err := url.Parse(server.URL(entry))
	if err != nil {
		t.Fatal(err)
	}
	css := page.ResolveReference(&url.URL{Path: "style.css", RawQuery: "version=1"}).String()
	get(http.DefaultClient, css, http.StatusForbidden)
	headers := get(client, page.String(), http.StatusOK)
	if !strings.Contains(headers.Get("Content-Security-Policy"), "default-src 'none'") {
		t.Fatal("missing content script policy")
	}
	get(client, css, http.StatusOK)
	get(client, page.ResolveReference(&url.URL{Path: "missing.css"}).String(), http.StatusNotFound)
	if server.Failures() != 1 {
		t.Fatalf("failed resource count %d", server.Failures())
	}
}
