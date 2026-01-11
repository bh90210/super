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
		for {
			conn, err := grpc.NewClient("localhost:3005",
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			)
			if err != nil {
				panic("grpc")
			}

			client := api.NewGithubClient(conn)
			w, err := client.Webhook(context.Background(), &api.Empty{})
			if err != nil {
				log.Fatalf("could not call webhook: %v", err)
			}

			err = webhook.GithubWebhook(w)
			if err != nil {
				log.Fatalf("could not handle webhook response: %v", err)
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

			case webhook.PackageEvent:
				var buf bytes.Buffer
				enc := gob.NewEncoder(&buf)
				err := enc.Encode(payload)
				if err != nil {
					fmt.Printf("Could not encode payload: %v\n", err)
					return
				}

				service.Broadcast(api.Hook_REGPUSH, buf.Bytes())
			}

		})

		fmt.Println("listening for github webhooks callbacks")

		http.ListenAndServe(":3000", nil)
	}
}
