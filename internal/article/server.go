package article

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dendec/zimlite/internal/profile"

	_ "golang.org/x/image/webp"
)

// CSP disables scripts from the archive while preserving static HTML/CSS.
const contentPolicy = "default-src 'none'; style-src 'self' 'unsafe-inline'; img-src 'self'; font-src 'self'; base-uri 'self'; form-action 'none'; frame-src 'none'"

const (
	cookieName      = "zimlite_article"
	maxSessionBytes = 32 << 20
	maxImagePixels  = 4 * 1024 * 1024
	// Formula-heavy articles issue hundreds of small SVG requests together.
	// Waiting handlers hold no resource data; reads remain serialized below.
	maxPendingResources = 512
)

// Server serves one article source on loopback. It owns neither Source nor SDL.
// Close waits for resource reads before the caller may close Source.
type Server struct {
	source   Source
	token    string
	address  string
	server   *http.Server
	done     chan struct{}
	readMu   sync.Mutex
	closing  bool
	bytes    atomic.Int64
	failures atomic.Int64
	slots    chan struct{}
	pending  chan struct{}
}

func StartServer(source Source) (*Server, error) {
	token := make([]byte, 32)
	if _, err := rand.Read(token); err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	s := &Server{source: source, token: hex.EncodeToString(token), address: listener.Addr().String(), done: make(chan struct{}), slots: make(chan struct{}, 2), pending: make(chan struct{}, maxPendingResources)}
	s.server = &http.Server{Handler: s, ReadHeaderTimeout: 3 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 5 * time.Second, MaxHeaderBytes: 8 << 10}
	go func() {
		defer close(s.done)
		_ = s.server.Serve(listener)
	}()
	return s, nil
}

func (s *Server) URL(articlePath string) string {
	u := url.URL{Scheme: "http", Host: s.address, Path: "/" + articlePath}
	q := u.Query()
	q.Set("zimlite_token", s.token)
	u.RawQuery = q.Encode()
	return u.String()
}

func (s *Server) Failures() int64 { return s.failures.Load() }

func (s *Server) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err := s.server.Shutdown(ctx)
	if err != nil {
		_ = s.server.Close()
	}
	<-s.done
	// Shutdown may time out; serializing here also waits for an active read.
	s.readMu.Lock()
	s.closing = true
	s.readMu.Unlock()
	return err
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Host != s.address || (r.Method != http.MethodGet && r.Method != http.MethodHead) {
		http.Error(w, "unsupported request", http.StatusBadRequest)
		return
	}
	token := r.URL.Query().Get("zimlite_token")
	bootstrap := token != ""
	if !bootstrap {
		if c, err := r.Cookie(cookieName); err == nil {
			token = c.Value
		}
	}
	if subtle.ConstantTimeCompare([]byte(token), []byte(s.token)) != 1 {
		http.Error(w, "invalid article session", http.StatusForbidden)
		return
	}
	if bootstrap {
		http.SetCookie(w, &http.Cookie{Name: cookieName, Value: s.token, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode})
	}
	select {
	case s.pending <- struct{}{}:
		defer func() { <-s.pending }()
	default:
		if s.failures.Add(1) <= 8 {
			slog.Warn("Article resource queue full", "path", r.URL.Path)
		}
		profile.Mark("resource.rejected", map[string]any{"path": r.URL.Path, "reason": "queue_full"})
		http.Error(w, "resource queue full", http.StatusTooManyRequests)
		return
	}
	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	case <-r.Context().Done():
		return
	}
	name := strings.TrimPrefix(r.URL.Path, "/")
	if name == "" || strings.ContainsRune(name, '\x00') {
		http.Error(w, "invalid resource path", http.StatusBadRequest)
		return
	}
	readStarted := time.Now()
	profile.Mark("resource.begin", map[string]any{"path": name})
	s.readMu.Lock()
	if s.closing {
		s.readMu.Unlock()
		http.Error(w, "article session closed", http.StatusServiceUnavailable)
		return
	}
	resource, err := s.source.ReadResource(name)
	s.readMu.Unlock()
	if err == nil && strings.HasPrefix(resource.MIME, "text/html") {
		resource.Data = containWideTables(resource.Data)
	}
	fields := map[string]any{"path": name, "bytes": len(resource.Data), "mime": resource.MIME, "canonical": resource.CanonicalPath, "elapsed_ms": time.Since(readStarted).Milliseconds()}
	if err != nil {
		fields["error"] = err.Error()
	}
	profile.Mark("resource.end", fields)
	if err != nil {
		s.failures.Add(1)
		status := http.StatusNotFound
		if errors.Is(err, ErrResourceLimit) {
			status = http.StatusRequestEntityTooLarge
		}
		http.Error(w, "article resource unavailable", status)
		return
	}
	if resource.CanonicalPath != "" && resource.CanonicalPath != name {
		target := url.URL{Path: "/" + resource.CanonicalPath}
		http.Redirect(w, r, target.String(), http.StatusFound)
		return
	}
	if len(resource.Data) > MaxResourceBytes || (strings.HasPrefix(resource.MIME, "text/html") && len(resource.Data) > MaxHTMLBytes) || s.bytes.Add(int64(len(resource.Data))) > maxSessionBytes {
		s.failures.Add(1)
		http.Error(w, "article memory budget exceeded", http.StatusRequestEntityTooLarge)
		return
	}
	if strings.HasPrefix(resource.MIME, "image/") && !strings.HasPrefix(resource.MIME, "image/svg+xml") {
		config, _, e := image.DecodeConfig(bytes.NewReader(resource.Data))
		if e != nil || config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > maxImagePixels {
			s.failures.Add(1)
			http.Error(w, "decoded image exceeds memory budget", http.StatusRequestEntityTooLarge)
			return
		}
	}
	contentType := resource.MIME
	if strings.HasPrefix(contentType, "image/svg+xml") {
		// MediaWiki adds charset/profile parameters to its SVG MIME type.
		// Serve the standard image type so embedders recognize SVG images.
		contentType = "image/svg+xml"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Security-Policy", contentPolicy)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Length", fmt.Sprint(len(resource.Data)))
	if r.Method != http.MethodHead {
		_, _ = w.Write(resource.Data)
	}
}
