package shop

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func newTestShop(t *testing.T, files map[string]string) *httptest.Server {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	s, err := New(dir, "Welcome")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)
	return srv
}

func get(t *testing.T, url string, header http.Header) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header = header
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

func TestIndexListsOnlyGameFiles(t *testing.T) {
	srv := newTestShop(t, map[string]string{
		"Game One.nsp":       "12345",
		"sub/Game Two.XCI":   "abc",
		"sub/deeper/dlc.nsz": "x",
		"readme.txt":         "ignored",
		".hidden/secret.nsp": "ignored",
		".partial.nsp":       "ignored",
	})

	resp := get(t, srv.URL+"/", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var idx Index
	if err := json.NewDecoder(resp.Body).Decode(&idx); err != nil {
		t.Fatal(err)
	}
	if idx.Success != "Welcome" {
		t.Errorf("success = %q", idx.Success)
	}

	got := map[string]int64{}
	for _, f := range idx.Files {
		got[f.URL] = f.Size
	}
	want := map[string]int64{
		srv.URL + "/files/Game%20One.nsp":     5,
		srv.URL + "/files/sub/Game%20Two.XCI": 3,
		srv.URL + "/files/sub/deeper/dlc.nsz": 1,
	}
	if len(got) != len(want) {
		t.Fatalf("files = %v, want %v", got, want)
	}
	for u, size := range want {
		if got[u] != size {
			t.Errorf("file %s size = %d, want %d (all: %v)", u, got[u], size, got)
		}
	}
}

func TestIndexEmptyDirReturnsEmptyList(t *testing.T) {
	srv := newTestShop(t, nil)
	resp := get(t, srv.URL+"/", nil)
	body, _ := io.ReadAll(resp.Body)
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatal(err)
	}
	if string(raw["files"]) != "[]" {
		t.Errorf("files = %s, want []", raw["files"])
	}
}

func TestIndexURLsAreDownloadable(t *testing.T) {
	srv := newTestShop(t, map[string]string{"sub/Game #1 [v0].nsp": "content"})
	var idx Index
	if err := json.NewDecoder(get(t, srv.URL+"/", nil).Body).Decode(&idx); err != nil {
		t.Fatal(err)
	}
	if len(idx.Files) != 1 {
		t.Fatalf("files = %v", idx.Files)
	}
	resp := get(t, idx.Files[0].URL, nil)
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || string(body) != "content" {
		t.Errorf("status = %d, body = %q", resp.StatusCode, body)
	}
}

func TestServeFileSupportsRange(t *testing.T) {
	srv := newTestShop(t, map[string]string{"game.nsp": "0123456789"})
	resp := get(t, srv.URL+"/files/game.nsp", http.Header{"Range": {"bytes=2-5"}})
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusPartialContent {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if string(body) != "2345" {
		t.Errorf("body = %q", body)
	}
}

func TestServeFileRejectsNonGamesAndEscapes(t *testing.T) {
	srv := newTestShop(t, map[string]string{"readme.txt": "no", "sub/game.nsp": "ok"})

	paths := []string{
		"/files/readme.txt",
		"/files/missing.nsp",
		"/files/sub",
		"/files/../go.mod",
		"/files/sub/..%2F..%2Fetc%2Fpasswd.nsp",
		"/files/%2e%2e/outside.nsp",
	}
	for _, p := range paths {
		resp := get(t, srv.URL+p, nil)
		if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusPartialContent {
			t.Errorf("GET %s: status = %d, want an error", p, resp.StatusCode)
		}
	}
}

func TestServeFileDoesNotFollowSymlinkOutsideRoot(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.nsp")
	if err := os.WriteFile(outside, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "link.nsp")); err != nil {
		t.Skip("symlinks unsupported:", err)
	}
	s, err := New(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	resp := get(t, srv.URL+"/files/link.nsp", nil)
	if resp.StatusCode == http.StatusOK {
		t.Errorf("symlink outside root was served")
	}
	var idx Index
	if err := json.NewDecoder(get(t, srv.URL+"/", nil).Body).Decode(&idx); err != nil {
		t.Fatal(err)
	}
	if len(idx.Files) != 0 {
		t.Errorf("symlink outside root listed in index: %v", idx.Files)
	}
}

func TestHealthz(t *testing.T) {
	srv := newTestShop(t, nil)
	if resp := get(t, srv.URL+"/healthz", nil); resp.StatusCode != http.StatusNoContent {
		t.Errorf("status = %d", resp.StatusCode)
	}
}
