package kitestream

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.disney.com/SANCR225/koda/internal/pkg"
)

// Server is the KiteStream HTTP server.
type Server struct {
	steerRoot string
	targetDir string
	port      int
	token     string
	bridge    *Bridge
	mux       *http.ServeMux
	srv       *http.Server
}

// NewServer creates a configured KiteStream server.
func NewServer(steerRoot, targetDir string, port int, token string) *Server {
	s := &Server{
		steerRoot: steerRoot,
		targetDir: targetDir,
		port:      port,
		token:     token,
		bridge:    NewBridge(),
		mux:       http.NewServeMux(),
	}
	s.routes()
	s.srv = &http.Server{
		Addr:    fmt.Sprintf("127.0.0.1:%d", port),
		Handler: s.mux,
	}
	return s
}

func (s *Server) ListenAndServe() error {
	return s.srv.ListenAndServe()
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]string{"status": "ok", "version": "0.1.0"})
	})

	s.mux.HandleFunc("GET /ws", func(w http.ResponseWriter, r *http.Request) {
		if !s.authenticate(r) {
			http.Error(w, "Unauthorized", 401)
			return
		}
		s.bridge.HandleWebSocket(w, r)
	})

	// API catch-all using a custom handler
	apiHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.authenticate(r) {
			writeJSON(w, 401, map[string]string{"error": "Unauthorized"})
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB
		s.handleAPI(w, r)
	})
	s.mux.Handle("GET /api/", apiHandler)
	s.mux.Handle("POST /api/", apiHandler)
	s.mux.Handle("DELETE /api/", apiHandler)

	// Static files (catch-all for everything else)
	s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.SetCookie(w, &http.Cookie{
				Name: "kitestream_token", Value: s.token,
				HttpOnly: true, SameSite: http.SameSiteStrictMode, Path: "/",
			})
		}
		s.serveStatic(w, r)
	})
}

func (s *Server) authenticate(r *http.Request) bool {
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ") == s.token
	}
	if c, err := r.Cookie("kitestream_token"); err == nil {
		return c.Value == s.token
	}
	return false
}

func (s *Server) serveStatic(w http.ResponseWriter, r *http.Request) {
	bundleDist := filepath.Join(pkg.BundlePath("kitestream"), "client", "dist")
	if _, err := os.Stat(bundleDist); err == nil {
		http.FileServer(http.Dir(bundleDist)).ServeHTTP(w, r)
		return
	}
	devDist := os.Getenv("KITESTREAM_DEV_DIST")
	if devDist != "" {
		http.FileServer(http.Dir(devDist)).ServeHTTP(w, r)
		return
	}
	http.NotFound(w, r)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
