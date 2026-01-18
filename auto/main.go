package main

import (
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
	// Intitiate prometheus metrics server.
	go func() {
		slog.Info("starting metrics server on :2112")

		http.Handle(prometheusPath, promhttp.Handler())
		http.ListenAndServe(":2112", nil)
	}()

	// Start HTTP server to listen for GitHub webhooks.
	githubSecret := os.Getenv("GITHUB_SECRET")

	hook, err := github.New(github.Options.Secret(githubSecret))
	if err != nil {
		slog.Error("could not create github webhook", slog.String("error", err.Error()))
		return
	}

	slog.Info("server started on :3005")

	http.Handle(githubPath, http.HandlerFunc(backend.GithubHandle(hook)))
	http.ListenAndServe(":3005", nil)
}
