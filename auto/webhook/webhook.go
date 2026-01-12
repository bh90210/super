// Package webhook provides code for Gihub webhooks.
package webhook

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"sync"

	"github.com/bh90210/super/auto/api"
	githubgoo "github.com/google/go-github/v81/github"
	"google.golang.org/grpc"
)

var _ api.GithubServer = (*Service)(nil)

type Service struct {
	api.UnimplementedGithubServer

	streams []api.Github_WebhookServer
	wgs     []*sync.WaitGroup
	mu      sync.Mutex
}

func NewService() (*Service, error) {
	return &Service{}, nil
}

func (s *Service) Webhook(_ *api.Empty, stream api.Github_WebhookServer) error {
	s.mu.Lock()
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s.streams = append(s.streams, stream)
	s.wgs = append(s.wgs, wg)
	s.mu.Unlock()

	wg.Wait()

	return nil
}

func (s *Service) Broadcast(hooktype api.Hook_Type, data []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var removeIndex []int
	for i, stream := range s.streams {
		err := stream.Send(&api.WebhookResponse{
			Hooktype: &api.Hook{
				Type: hooktype,
			},
			Data: data,
		})
		if err != nil {
			removeIndex = append(removeIndex, i)
			continue
		}
	}

	for _, v := range removeIndex {
		s.streams = append(s.streams[:v], s.streams[v+1:]...)
		s.wgs[v].Done()
		s.wgs = append(s.wgs[:v], s.wgs[v+1:]...)
	}
}

func GithubWebhook(w grpc.ServerStreamingClient[api.WebhookResponse]) error {
	for {
		resp, err := w.Recv()
		if err != nil {
			log.Printf("could not receive webhook response: %v", err)
			return err
		}

		switch resp.Hooktype.Type {
		case api.Hook_PUSH:
			var payload githubgoo.PushEvent
			err = json.Unmarshal(resp.Data, &payload)
			if err != nil {
				fmt.Printf("Could not decode payload: %v\n", err)
				continue
			}

			fmt.Printf("Received push event for repo: %s\n", *payload.Repo.FullName)

		case api.Hook_REGPACK:
			var payload githubgoo.RegistryPackageEvent
			err = json.Unmarshal(resp.Data, &payload)
			if err != nil {
				fmt.Printf("Could not decode payload: %v\n", err)
				continue
			}

			updateSuper(payload)

		case api.Hook_RELEASE:
			var payload githubgoo.ReleaseEvent
			err = json.Unmarshal(resp.Data, &payload)
			if err != nil {
				fmt.Printf("Could not decode payload: %v\n", err)
				continue
			}

			fmt.Printf("Received release event for repo: %s, tag: %s\n", payload.Repo.FullName, payload.Release.TagName)
		}
	}
}

func updateSuper(payload githubgoo.RegistryPackageEvent) {
	// Check is sender is bh90210.
	if payload.Sender.GetLogin() != "bh90210" {
		fmt.Printf("Ignoring registry package event from sender: %s\n", payload.Sender.GetLogin())
		return
	}

	// Check if package name is super.
	if payload.RegistryPackage.GetName() != "super" {
		fmt.Printf("Ignoring registry package event for package: %s\n", payload.RegistryPackage.GetName())
		return
	}

	// Check if action is published.
	if payload.GetAction() != "published" {
		fmt.Printf("Ignoring registry package event with action: %s\n", payload.GetAction())
		return
	}

	// Check if tag is server.latest.
	if payload.RegistryPackage.PackageVersion.ContainerMetadata.Tag.GetName() != "server.latest" {
		fmt.Printf("Ignoring registry package event with tag: %s\n", payload.RegistryPackage.PackageVersion.ContainerMetadata.Tag.GetName())
		return
	}

	// Get the env viariable and cd in the super directory.
	superPath := os.Getenv("SUPER_PATH")

	// Run a command to pull the latest image and deploy it.
	lsCmd := exec.Command("docker", "compose", "pull")
	lsCmd.Dir = superPath
	lsOut, err := lsCmd.CombinedOutput()
	if err != nil {
		log.Printf("Could not run docker pull: %v, %s", err, string(lsOut))
		return
	}

	fmt.Println(string(lsOut))

	lsCmd = exec.Command("docker", "compose", "up", "-d")
	lsCmd.Dir = superPath
	lsOut, err = lsCmd.CombinedOutput()
	if err != nil {
		log.Printf("Could not run docker up: %v, %s", err, string(lsOut))
		return
	}

	fmt.Println(string(lsOut))
}
