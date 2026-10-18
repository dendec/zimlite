//go:build linux || darwin || freebsd

package servo

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dendec/zimlite/internal/article"
)

type lifecycleSource struct{ closes atomic.Int32 }

func (*lifecycleSource) ReadResource(string) (article.Resource, error) {
	return article.Resource{Data: []byte("<p>article</p>"), MIME: "text/html"}, nil
}

type blockedSource struct {
	lifecycleSource
	entered, unblock chan struct{}
}

func (s *blockedSource) ReadResource(string) (article.Resource, error) {
	close(s.entered)
	<-s.unblock
	return article.Resource{Data: []byte("<p>article</p>"), MIME: "text/html"}, nil
}

func TestCancelledSessionDoesNotWaitForBlockedArchiveRead(t *testing.T) {
	helper := filepath.Join(t.TempDir(), "helper")
	if err := os.WriteFile(helper, []byte("#!/bin/sh\nsleep 30\n"), 0700); err != nil {
		t.Fatal(err)
	}
	source := &blockedSource{entered: make(chan struct{}), unblock: make(chan struct{})}
	session, err := NewSession(helper, source, "index.html", 640, 480)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		close(source.unblock)
		session.Close()
		select {
		case <-session.Done:
		case <-time.After(3 * time.Second):
			t.Error("resource cleanup did not finish")
		}
	}()
	u, err := url.Parse(session.BaseURL())
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/index.html"
	go func() {
		if response, e := http.Get(u.String()); e == nil {
			_ = response.Body.Close()
		}
	}()
	select {
	case <-source.entered:
	case <-time.After(3 * time.Second):
		t.Fatal("source read did not start")
	}
	start := time.Now()
	session.Close()
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("cancel blocked UI for %s", elapsed)
	}
	if source.closes.Load() != 0 {
		t.Fatal("source was closed during an active native read")
	}
	newSource := &lifecycleSource{}
	if _, err := NewSession(helper, newSource, "index.html", 640, 480); err == nil {
		t.Fatal("started another helper during deferred cleanup")
	}
	if newSource.closes.Load() != 1 {
		t.Fatal("declined session did not close its source")
	}
}
func (s *lifecycleSource) Close() error { s.closes.Add(1); return nil }

func TestFailedSessionStartupClosesSource(t *testing.T) {
	source := &lifecycleSource{}
	if _, err := NewSession(filepath.Join(t.TempDir(), "missing-helper"), source, "index.html", 640, 480); err == nil {
		t.Fatal("missing helper should fail startup")
	}
	if got := source.closes.Load(); got != 1 {
		t.Fatalf("source closed %d times, want once", got)
	}
}

func TestMalformedHelperOutputTerminatesSessionAndClosesSource(t *testing.T) {
	helper := filepath.Join(t.TempDir(), "helper")
	if err := os.WriteFile(helper, []byte("#!/bin/sh\nprintf 'bad!'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	source := &lifecycleSource{}
	session, err := NewSession(helper, source, "index.html", 640, 480)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(session.Close)
	select {
	case err := <-session.Errors:
		if err == nil {
			t.Fatal("malformed helper output must report an error")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("helper protocol failure was not reported")
	}
	select {
	case <-session.Done:
	case <-time.After(3 * time.Second):
		t.Fatal("failed helper session did not stop")
	}
	session.Close()
	session.Close()
	if got := source.closes.Load(); got != 1 {
		t.Fatalf("source closed %d times, want once", got)
	}
	if err := session.Send(Command{Op: "state"}); err == nil {
		t.Fatal("closed session accepted input")
	}
}
