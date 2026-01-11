// Package webhook provides code for Gihub webhooks.
package webhook

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"log"
	"sync"

	"github.com/bh90210/super/auto/api"
	"github.com/davecgh/go-spew/spew"
	"github.com/go-playground/webhooks/v6/github"
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

		buf := bytes.NewBuffer(resp.Data)
		dec := gob.NewDecoder(buf)

		switch resp.Hooktype.Type {
		case api.Hook_PUSH:
			var payload github.PushPayload
			err = dec.Decode(&payload)
			if err != nil {
				fmt.Printf("Could not decode payload: %v\n", err)
				continue
			}

			fmt.Printf("Received push event for repo: %s\n", payload.Repository.FullName)
			spew.Dump(payload)

		case api.Hook_REGPACK:
			var payload RegistryPackageEvent
			err = dec.Decode(&payload)
			if err != nil {
				fmt.Printf("Could not decode payload: %v\n", err)
				continue
			}

			fmt.Printf("Received package event: %s for package: %s\n", payload.Action, payload.Package.Name)
			spew.Dump(payload)

		case api.Hook_RELEASE:
			var payload github.ReleasePayload
			err = dec.Decode(&payload)
			if err != nil {
				fmt.Printf("Could not decode payload: %v\n", err)
				continue
			}

			fmt.Printf("Received release event for repo: %s, tag: %s\n", payload.Repository.FullName, payload.Release.TagName)
			spew.Dump(payload)

		}
	}
}

// RegistryPackageEvent represents the GitHub "package" webhook payload.
// X-GitHub-Event: package
type RegistryPackageEvent struct {
	Action       string        `json:"action"`                 // published, updated, deleted
	Package      Package       `json:"package"`                // Package metadata
	Organization *Organization `json:"organization,omitempty"` // Present for org-owned packages
	Sender       User          `json:"sender"`                 // Actor that triggered the event
}

// Package represents a GitHub Package (npm, container, maven, etc.)
type Package struct {
	ID             int64           `json:"id"`
	Name           string          `json:"name"`
	PackageType    string          `json:"package_type"` // npm, maven, container, nuget, rubygems
	HTMLURL        string          `json:"html_url"`
	CreatedAt      string          `json:"created_at"` // ISO-8601 timestamp
	UpdatedAt      string          `json:"updated_at"` // ISO-8601 timestamp
	Owner          User            `json:"owner"`
	PackageVersion *PackageVersion `json:"package_version,omitempty"` // Nil on delete
}

// PackageVersion represents a specific version of a package.
type PackageVersion struct {
	ID          int64           `json:"id"`
	Name        string          `json:"name"` // Version or digest
	Description string          `json:"description"`
	Summary     string          `json:"summary"`
	Body        string          `json:"body"`
	HTMLURL     string          `json:"html_url"`
	CreatedAt   string          `json:"created_at"` // ISO-8601 timestamp
	UpdatedAt   string          `json:"updated_at"` // ISO-8601 timestamp
	Metadata    PackageMetadata `json:"metadata"`
}

// PackageMetadata holds registry-specific metadata.
type PackageMetadata struct {
	PackageType string `json:"package_type"` // npm, container, etc.
}

// Organization represents a GitHub organization.
type Organization struct {
	Login string `json:"login"`
	ID    int64  `json:"id"`
}

// User represents a GitHub user or organization account.
type User struct {
	Login string `json:"login"`
	ID    int64  `json:"id"`
	Type  string `json:"type"` // User or Organization
}
