// Command shop runs the Tinfoil/CyberFoil custom shop server.
package main

import (
	"flag"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/ryansese/my-switch-shop/internal/shop"
)

// version is set at build time via -ldflags "-X main.version=...".
var version = "dev"

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// statusRecorder captures the response status for request logging.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

// logRequests logs every request except health checks, so connection
// problems from the Switch show up in `docker compose logs`.
func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			next.ServeHTTP(w, r)
			return
		}
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		log.Printf("%s %s %s %d %q", r.RemoteAddr, r.Method, r.URL.RequestURI(), rec.status, r.UserAgent())
	})
}

func main() {
	healthcheck := flag.Bool("healthcheck", false, "probe the local server and exit 0 if healthy")
	flag.Parse()

	port := env("SHOP_PORT", "8080")

	// The runtime image has no shell or curl, so the container health check
	// calls the binary itself.
	if *healthcheck {
		client := http.Client{Timeout: 3 * time.Second}
		resp, err := client.Get("http://127.0.0.1:" + port + "/healthz")
		if err != nil || resp.StatusCode != http.StatusNoContent {
			os.Exit(1)
		}
		os.Exit(0)
	}

	gamesDir := env("SHOP_GAMES_DIR", "/games")
	s, err := shop.New(gamesDir, env("SHOP_MESSAGE", "Welcome to my Switch shop"))
	if err != nil {
		log.Fatalf("open games directory %s: %v", gamesDir, err)
	}
	defer s.Close()

	mux := http.NewServeMux()
	mux.Handle("/", s.Handler())
	mux.HandleFunc("GET /version", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		io.WriteString(w, version+"\n")
	})

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           logRequests(mux),
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("shop %s serving %s on :%s", version, gamesDir, port)
	log.Fatal(srv.ListenAndServe())
}
