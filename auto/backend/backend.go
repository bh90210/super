// Package backend contains functions meant to run from the host machine.
// It is structured to be as procedures doing things with no internal state (or structs.)
// Metrics register with prometheus on init().
package backend

import (
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/exec"

	"github.com/go-playground/webhooks/v6/github"
	githubgoo "github.com/google/go-github/v81/github"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type metrics struct {
	gihubWebhook *prometheus.GaugeVec
	updateSuper  *prometheus.GaugeVec
}

var m metrics

func init() {
	m.gihubWebhook = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "auto_backend_github_webhook_total",
			Help: "The total number of github webhooks received by type.",
		},
		[]string{"type"},
	)

	m.updateSuper = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "auto_backend_update_super",
			Help: "The stepwise status of updating to latest version of super server from github registry package webhook.",
		},
		[]string{"status"},
	)
}

func GithubHandle(hook *github.Webhook) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// Parse the webhook payload.
		payload, err := hook.Parse(r, github.PushEvent, github.ReleaseEvent, github.RegistryPackageEvent)
		if err != nil {
			if errors.Is(err, github.ErrEventNotFound) {
				slog.Error("ignoring unsupported event type")
				return
			}

			slog.Error("could not parse webhook", slog.String("error", err.Error()))
			return
		}

		// Decide what to do based on event type.
		// Then marshal the payload and broadcast it to the backend host running service.
		switch payload := payload.(type) {
		case githubgoo.PushEvent:
			g := m.gihubWebhook.With(prometheus.Labels{
				"type": "push",
			})
			defer g.Dec()

			g.Inc()

			slog.Info("Received push event for repo", slog.String("repo", *payload.Repo.FullName))

		case githubgoo.ReleaseEvent:
			g := m.gihubWebhook.With(prometheus.Labels{
				"type": "release",
			})
			defer g.Dec()

			g.Inc()
			slog.Info("Received release event for repo", slog.String("repo", *payload.Repo.FullName), slog.String("tag", *payload.Release.TagName))

		case githubgoo.RegistryPackageEvent:
			slog.Info("Received registry package event for package", slog.String("package", payload.RegistryPackage.GetName()))
			updateSuper(payload)
		}
	}
}

func updateSuper(payload githubgoo.RegistryPackageEvent) {
	defer m.updateSuper.With(prometheus.Labels{
		"status": "finished",
	}).Set(0)

	// Check is sender is bh90210.
	if payload.Sender.GetLogin() != "github-actions[bot]" {
		m.updateSuper.With(prometheus.Labels{
			"status": "invalid_sender",
		}).Set(-1)

		slog.Info("Ignoring registry package event from sender", slog.String("sender", payload.Sender.GetLogin()))
		return
	}

	m.updateSuper.With(prometheus.Labels{
		"status": "started",
	}).Inc()

	// Check if package name is server.
	if payload.RegistryPackage.GetName() != "server" {
		m.updateSuper.With(prometheus.Labels{
			"status": "invalid_package",
		}).Set(-1)

		slog.Info("Ignoring registry package event for package", slog.String("package", payload.RegistryPackage.GetName()))
		return
	}

	m.updateSuper.With(prometheus.Labels{
		"status": "valid_package",
	}).Inc()

	// Check if action is published.
	if payload.GetAction() != "published" {
		m.updateSuper.With(prometheus.Labels{
			"status": "invalid_action",
		}).Set(-1)

		slog.Info("Ignoring registry package event with action", slog.String("action", payload.GetAction()))
		return
	}

	m.updateSuper.With(prometheus.Labels{
		"status": "valid_action",
	}).Inc()

	// Check if tag is server:latest.
	if payload.RegistryPackage.PackageVersion.ContainerMetadata.Tag.GetName() != "latest" {
		m.updateSuper.With(prometheus.Labels{
			"status": "invalid_tag",
		}).Set(-1)

		slog.Info("Ignoring registry package event with tag", slog.String("tag", payload.RegistryPackage.PackageVersion.ContainerMetadata.Tag.GetName()))
		return
	}

	m.updateSuper.With(prometheus.Labels{
		"status": "valid_tag",
	}).Inc()

	slog.Info("Updating super from registry package webhook event")

	// Get the env viariable and cd in the super directory.
	superServerPath := os.Getenv("SUPER_PATH")

	// Run a command to pull the latest image and deploy it.
	lsCmd := exec.Command("docker", "pull", "ghcr.io/bh90210/server:latest")
	lsCmd.Dir = superServerPath
	lsOut, err := lsCmd.CombinedOutput()
	if err != nil {
		m.updateSuper.With(prometheus.Labels{
			"status": "could_not_pull_latest_image",
		}).Set(-1)

		slog.Error("could not run docker pull", slog.String("error", err.Error()), slog.String("output", string(lsOut)))
		return
	}

	m.updateSuper.With(prometheus.Labels{
		"status": "latest_image_pulled",
	}).Inc()

	slog.Info("docker pull output", slog.String("output", string(lsOut)))

	lsCmd = exec.Command("docker", "stack", "deploy", "-c", "docker-swarm.yaml", "server")
	lsCmd.Dir = superServerPath
	lsOut, err = lsCmd.CombinedOutput()
	if err != nil {
		m.updateSuper.With(prometheus.Labels{
			"status": "could_not_deploy",
		}).Set(-1)

		slog.Error("could not run docker swarm deploy", slog.String("error", err.Error()), slog.String("output", string(lsOut)))
		return
	}

	m.updateSuper.With(prometheus.Labels{
		"status": "super_updated",
	}).Inc()

	slog.Info("docker swarm deploy output", slog.String("output", string(lsOut)))
}
