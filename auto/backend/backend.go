// Package backend contains functions meant to run from the host machine.
// It is structured to be as procedures doing things with no internal state (or structs.)
// Metrics register with prometheus on init().
package backend

import (
	"encoding/json"
	"log/slog"
	"os"
	"os/exec"

	"github.com/bh90210/super/auto/api"
	githubgoo "github.com/google/go-github/v81/github"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
)

type metrics struct {
	opsProcessed prometheus.Counter
}

var m metrics

func init() {

}

func GithubWebhook(w grpc.ServerStreamingClient[api.WebhookResponse]) error {
	for {
		resp, err := w.Recv()
		if err != nil {
			slog.Error("could not receive webhook response", slog.String("error", err.Error()))
			return err
		}

		switch resp.Hooktype.Type {
		case api.Hook_PUSH:
			var payload githubgoo.PushEvent
			err = json.Unmarshal(resp.Data, &payload)
			if err != nil {
				slog.Error("could not decode payload", slog.String("error", err.Error()))
				continue
			}

			slog.Info("Received push event for repo", slog.String("repo", *payload.Repo.FullName))

		case api.Hook_REGPACK:
			var payload githubgoo.RegistryPackageEvent
			err = json.Unmarshal(resp.Data, &payload)
			if err != nil {
				slog.Error("could not decode payload", slog.String("error", err.Error()))
				continue
			}

			updateSuper(payload)

		case api.Hook_RELEASE:
			var payload githubgoo.ReleaseEvent
			err = json.Unmarshal(resp.Data, &payload)
			if err != nil {
				slog.Error("could not decode payload", slog.String("error", err.Error()))
				continue
			}

			slog.Info("Received release event for repo", slog.String("repo", *payload.Repo.FullName), slog.String("tag", *payload.Release.TagName))
		}
	}
}

func updateSuper(payload githubgoo.RegistryPackageEvent) {
	// Check is sender is bh90210.
	if payload.Sender.GetLogin() != "github-actions[bot]" {
		slog.Info("Ignoring registry package event from sender", slog.String("sender", payload.Sender.GetLogin()))
		return
	}

	// Check if package name is server.
	if payload.RegistryPackage.GetName() != "server" {
		slog.Info("Ignoring registry package event for package", slog.String("package", payload.RegistryPackage.GetName()))
		return
	}

	// Check if action is published.
	if payload.GetAction() != "published" {
		slog.Info("Ignoring registry package event with action", slog.String("action", payload.GetAction()))
		return
	}

	// Check if tag is server:latest.
	if payload.RegistryPackage.PackageVersion.ContainerMetadata.Tag.GetName() != "latest" {
		slog.Info("Ignoring registry package event with tag", slog.String("tag", payload.RegistryPackage.PackageVersion.ContainerMetadata.Tag.GetName()))
		return
	}

	// Get the env viariable and cd in the super directory.
	superPathFile := os.Getenv("SUPER_PATH")
	dat, err := os.ReadFile(superPathFile)
	if err != nil {
		slog.Error("could not read super path file", slog.String("error", err.Error()))
		return
	}

	superPath := string(dat)

	// Run a command to pull the latest image and deploy it.
	lsCmd := exec.Command("docker", "compose", "pull")
	lsCmd.Dir = superPath
	lsOut, err := lsCmd.CombinedOutput()
	if err != nil {
		slog.Error("could not run docker pull", slog.String("error", err.Error()), slog.String("output", string(lsOut)))
		return
	}

	slog.Info("docker pull output", slog.String("output", string(lsOut)))

	lsCmd = exec.Command("docker", "compose", "up", "-d")
	lsCmd.Dir = superPath
	lsOut, err = lsCmd.CombinedOutput()
	if err != nil {
		slog.Error("could not run docker up", slog.String("error", err.Error()), slog.String("output", string(lsOut)))
		return
	}

	slog.Info("docker up output", slog.String("output", string(lsOut)))
}
