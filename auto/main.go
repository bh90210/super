package main

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"

	"github.com/bh90210/super/auto/api"
	"github.com/bh90210/super/auto/webhook"
	"github.com/go-playground/webhooks/v6/github"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	path = "/webhooks"
)

func main() {
	isServer := flag.Bool("server", true, "Run as server. Default is true.")
	flag.Parse()

	switch *isServer {
	case false:
		conn, err := grpc.NewClient("localhost:3005",
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			panic("grpc")
		}
		defer conn.Close()

		client := api.NewGithubClient(conn)
		webhook, err := client.Webhook(context.Background(), &api.Empty{})
		if err != nil {
			log.Fatalf("could not call webhook: %v", err)
		}

		for {
			resp, err := webhook.Recv()
			if err != nil {
				log.Fatalf("could not receive webhook response: %v", err)
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

			case api.Hook_REGPUSH:
				var payload PackageEvent
				err = dec.Decode(&payload)
				if err != nil {
					fmt.Printf("Could not decode payload: %v\n", err)
					continue
				}

				fmt.Printf("Received package event: %s for package: %s\n", payload.Action, payload.Package.Name)

			case api.Hook_RELEASE:
				var payload github.ReleasePayload
				err = dec.Decode(&payload)
				if err != nil {
					fmt.Printf("Could not decode payload: %v\n", err)
					continue
				}

				fmt.Printf("Received release event for repo: %s, tag: %s\n", payload.Repository.FullName, payload.Release.TagName)

			}
		}

	case true:
		grpcServer := grpc.NewServer()

		service, err := webhook.NewService()
		if err != nil {
			log.Fatalf("failed to create webhook service: %v", err)
		}

		api.RegisterGithubServer(grpcServer, service)

		go func() {
			lis, err := net.Listen("tcp", "0.0.0.0:3005")
			if err != nil {
				log.Fatalf("failed to listen: %v", err)
			}

			if err := grpcServer.Serve(lis); err != nil {
				log.Fatalf("failed to serve: %v", err)
			}
		}()

		githubSecret := os.Getenv("GITHUB_SECRET")
		hook, err := github.New(github.Options.Secret(githubSecret))
		if err != nil {
			fmt.Printf("Could not create github webhook: %v\n", err)
			return
		}

		http.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			payload, err := hook.Parse(r, github.PushEvent, github.Event(github.TagSubtype), github.ReleaseEvent, "package")
			if err != nil {
				if errors.Is(err, github.ErrEventNotFound) {
					fmt.Println("Event not found")
					return
				}

				fmt.Printf("Could not parse webhook: %v\n", err)
				return
			}

			switch payload := payload.(type) {
			case github.PushPayload:
				var buf bytes.Buffer
				enc := gob.NewEncoder(&buf)
				err := enc.Encode(payload)
				if err != nil {
					fmt.Printf("Could not encode payload: %v\n", err)
					return
				}

				service.Broadcast(api.Hook_PUSH, buf.Bytes())

				return

			case github.ReleasePayload:
				var buf bytes.Buffer
				enc := gob.NewEncoder(&buf)
				err := enc.Encode(payload)
				if err != nil {
					fmt.Printf("Could not encode payload: %v\n", err)
					return
				}

				service.Broadcast(api.Hook_RELEASE, buf.Bytes())

			case PackageEvent:
				var buf bytes.Buffer
				enc := gob.NewEncoder(&buf)
				err := enc.Encode(payload)
				if err != nil {
					fmt.Printf("Could not encode payload: %v\n", err)
					return
				}

				service.Broadcast(api.Hook_REGPUSH, buf.Bytes())

				// case github.PackagePayload:
			}

		})

		fmt.Println("listening for github webhooks callbacks")

		http.ListenAndServe(":3000", nil)
	}
}

// PackageEvent represents the GitHub "package" webhook payload.
// X-GitHub-Event: package
type PackageEvent struct {
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
