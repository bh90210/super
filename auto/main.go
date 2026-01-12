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
	"time"

	"github.com/bh90210/super/auto/api"
	"github.com/bh90210/super/auto/webhook"
	"github.com/go-playground/webhooks/v6/github"
	githubgoo "github.com/google/go-github/v81/github"
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
				log.Printf("could not connect to webhook server: %v", err)
				time.Sleep(2 * time.Second)
				continue
			}

			client := api.NewGithubClient(conn)
			w, err := client.Webhook(context.Background(), &api.Empty{})
			if err != nil {
				log.Printf("could not call webhook: %v", err)
				time.Sleep(2 * time.Second)
				continue
			}

			err = webhook.GithubWebhook(w)
			if err != nil {
				log.Printf("could not handle webhook response: %v", err)
				time.Sleep(2 * time.Second)
				continue
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
			payload, err := hook.Parse(r, github.PushEvent, github.ReleaseEvent, github.RegistryPackageEvent)
			if err != nil {
				if errors.Is(err, github.ErrEventNotFound) {
					fmt.Println(err)
					return
				}

				fmt.Printf("Could not parse webhook: %v\n", err)
				return
			}

			switch payload := payload.(type) {
			case githubgoo.PushEvent:
				var buf bytes.Buffer
				enc := gob.NewEncoder(&buf)
				err := enc.Encode(payload)
				if err != nil {
					fmt.Printf("Could not encode payload: %v\n", err)
					return
				}

				service.Broadcast(api.Hook_PUSH, buf.Bytes())

				return

			case githubgoo.ReleaseEvent:
				var buf bytes.Buffer
				enc := gob.NewEncoder(&buf)
				err := enc.Encode(payload)
				if err != nil {
					fmt.Printf("Could not encode payload: %v\n", err)
					return
				}

				service.Broadcast(api.Hook_RELEASE, buf.Bytes())

			case githubgoo.RegistryPackageEvent:
				var buf bytes.Buffer
				enc := gob.NewEncoder(&buf)
				err := enc.Encode(payload)
				if err != nil {
					fmt.Printf("Could not encode payload: %v\n", err)
					return
				}

				service.Broadcast(api.Hook_REGPACK, buf.Bytes())
			}

		})

		fmt.Println("listening for github webhooks callbacks")

		http.ListenAndServe(":3000", nil)
	}
}
