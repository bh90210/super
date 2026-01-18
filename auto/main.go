package main

import (
	"flag"
	"io"
	"log/slog"
	"net/http"
	"os"

	"github.com/bh90210/super/auto/backend"
	"github.com/go-playground/webhooks/v6/github"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	githubPath     = "/webhooks"
	prometheusPath = "/metrics"
)

var _ = backend.T

func main() {
	isServer := flag.Bool("proxy", true, "Run as server or as a proxy. Default is proxy.")
	flag.Parse()

	switch *isServer {
	// Server mode: handle webhooks and expose metrics.
	case false:
		// Intitiate prometheus metrics server.
		go func() {
			slog.Info("starting metrics server on :2112")

			http.Handle(prometheusPath, promhttp.Handler())
			http.ListenAndServe(":2112", nil)
		}()

		for {
			// Start HTTP server to listen for GitHub webhooks.
			githubSecret := os.Getenv("GITHUB_SECRET")

			hook, err := github.New(github.Options.Secret(githubSecret))
			if err != nil {
				slog.Error("could not create github webhook", slog.String("error", err.Error()))
				return
			}

			slog.Info("server started on :3000")

			http.Handle(githubPath, http.HandlerFunc(backend.GithubHandle(hook)))
			http.ListenAndServe(":3000", nil)
		}

	// Proxy mode: just pass through requests to the backend service.
	case true:
		slog.Info("auto started on :3000")

		http.Handle(githubPath, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Forward the request to the backend server
			proxyURL := "http://super1:3000" + r.URL.Path

			// Create a new request with the same method and body
			proxyReq, err := http.NewRequestWithContext(r.Context(), r.Method, proxyURL, r.Body)
			if err != nil {
				slog.Error("failed to create proxy request", slog.String("error", err.Error()))
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}

			// Copy headers from original request
			proxyReq.Header = r.Header.Clone()

			// Send the request to the backend
			client := &http.Client{}
			resp, err := client.Do(proxyReq)
			if err != nil {
				slog.Error("failed to proxy request", slog.String("error", err.Error()))
				http.Error(w, "Bad Gateway", http.StatusBadGateway)
				return
			}
			defer resp.Body.Close()

			// Copy response headers
			for key, values := range resp.Header {
				for _, value := range values {
					w.Header().Add(key, value)
				}
			}

			// Copy status code
			w.WriteHeader(resp.StatusCode)

			// Copy response body
			io.Copy(w, resp.Body)
		}))

		http.ListenAndServe(":3000", nil)
	}
}
