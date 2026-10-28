// Package shop serves a directory of Switch game files as a Tinfoil/CyberFoil
// custom shop: a JSON index at "/" and the files themselves under "/files/".
package shop

import (
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
)

// gameExtensions are the only file types listed in the index and served.
var gameExtensions = map[string]bool{
	".nsp": true,
	".nsz": true,
	".xci": true,
	".xcz": true,
}

// Index is the shop document Tinfoil and CyberFoil read.
type Index struct {
	Files   []File `json:"files"`
	Success string `json:"success,omitempty"`
}

// File is one downloadable entry in the index.
type File struct {
	URL  string `json:"url"`
	Size int64  `json:"size"`
}

// Shop serves the games under a single root directory.
type Shop struct {
	root    *os.Root
	message string
}

// New opens gamesDir as the shop root. All file access goes through an
// os.Root, so requests cannot escape the directory, even via symlinks.
func New(gamesDir, message string) (*Shop, error) {
	root, err := os.OpenRoot(gamesDir)
	if err != nil {
		return nil, err
	}
	return &Shop{root: root, message: message}, nil
}

// Close releases the root directory handle.
func (s *Shop) Close() error {
	return s.root.Close()
}

// Handler returns the shop's HTTP routes.
func (s *Shop) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.serveIndex)
	mux.HandleFunc("GET /files/{path...}", s.serveFile)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	return mux
}

func isGame(name string) bool {
	return gameExtensions[strings.ToLower(path.Ext(name))]
}

// scan walks the root and returns every game file, rebuilt on each call so
// newly added files appear without a restart.
func (s *Shop) scan(baseURL string) ([]File, error) {
	files := []File{}
	err := fs.WalkDir(s.root.FS(), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			// Skip unreadable entries instead of failing the whole index.
			log.Printf("scan %s: %v", p, err)
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if p != "." && strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() || !isGame(p) {
			return nil
		}
		info, err := d.Info()
		if err == nil && d.Type()&fs.ModeSymlink != 0 {
			// Resolve through the root: links that escape it fail here.
			info, err = s.root.Stat(p)
		}
		if err != nil {
			log.Printf("stat %s: %v", p, err)
			return nil
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		files = append(files, File{URL: baseURL + escapePath(p), Size: info.Size()})
		return nil
	})
	return files, err
}

// escapePath percent-encodes each segment of a slash-separated path.
func escapePath(p string) string {
	segments := strings.Split(p, "/")
	for i, seg := range segments {
		segments[i] = url.PathEscape(seg)
	}
	return strings.Join(segments, "/")
}

func (s *Shop) serveIndex(w http.ResponseWriter, r *http.Request) {
	files, err := s.scan("http://" + r.Host + "/files/")
	if err != nil {
		log.Printf("index: %v", err)
		http.Error(w, "failed to read games directory", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(Index{Files: files, Success: s.message}); err != nil {
		log.Printf("index: write: %v", err)
	}
}

func (s *Shop) serveFile(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("path")
	if !fs.ValidPath(name) || !isGame(name) {
		http.NotFound(w, r)
		return
	}
	f, err := s.root.Open(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}
	// ServeContent handles Range requests, which Tinfoil uses for resumed and
	// multi-part downloads.
	http.ServeContent(w, r, info.Name(), info.ModTime(), f)
}
