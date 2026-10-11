// Command playground is a local developer tool for manually triggering and
// inspecting video processing jobs without needing a full frontend.
//
// It serves an embedded HTML UI and proxies API requests to the Kithly API
// server, forwarding the caller's credential (Authorization or X-Guest-Token)
// from the browser's input.
//
// Usage:
//
//	PLAYGROUND_PORT=3003 API_URL=http://localhost:3001 go run ./cmd/playground
package main

import (
	_ "embed"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

//go:embed index.html
var indexHTML []byte

func main() {
	apiBase := os.Getenv("API_URL")
	if apiBase == "" {
		apiBase = "http://localhost:3001"
	}
	listenPort := os.Getenv("PLAYGROUND_PORT")
	if listenPort == "" {
		listenPort = "3003"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", serveIndex)
	mux.HandleFunc("/proxy/", makeProxy(apiBase))

	log.Printf("playground listening on :%s  →  API: %s", listenPort, apiBase)
	if err := http.ListenAndServe(":"+listenPort, mux); err != nil {
		log.Fatalf("playground: %v", err)
	}
}

// serveIndex returns the embedded single-page UI.
func serveIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(indexHTML)
}

// makeProxy returns an http.HandlerFunc that strips the /proxy prefix and
// forwards the request to apiBase with the caller's credential headers.
func makeProxy(apiBase string) http.HandlerFunc {
	client := &http.Client{Timeout: 30 * time.Second}

	return func(w http.ResponseWriter, r *http.Request) {
		apiPath := strings.TrimPrefix(r.URL.Path, "/proxy")
		target := apiBase + apiPath
		if r.URL.RawQuery != "" {
			target += "?" + r.URL.RawQuery
		}

		req, err := http.NewRequestWithContext(r.Context(), r.Method, target, r.Body)
		if err != nil {
			http.Error(w, "proxy: build request: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Forward Content-Type for JSON bodies.
		if ct := r.Header.Get("Content-Type"); ct != "" {
			req.Header.Set("Content-Type", ct)
		}

		// Pass through the credentials the API's auth middleware accepts.
		for _, h := range []string{"Authorization", "X-Guest-Token"} {
			if v := r.Header.Get(h); v != "" {
				req.Header.Set(h, v)
			}
		}

		resp, err := client.Do(req)
		if err != nil {
			http.Error(w, "proxy: upstream: "+err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close() //nolint:errcheck

		w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
	}
}
