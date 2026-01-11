package main

import (
	"bytes"
	"context"
	"encoding/gob"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"

	"github.com/bh90210/super/auto/api"
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

			var payload github.PushPayload
			buf := bytes.NewBuffer(resp.Data)
			dec := gob.NewDecoder(buf)
			err = dec.Decode(&payload)
			if err != nil {
				fmt.Printf("Could not decode payload: %v\n", err)
				continue
			}

			fmt.Printf("Received push event for repo: %s\n", payload.Repository.FullName)
		}

	case true:
		githubSecret := os.Getenv("GITHUB_SECRET")
		hook, err := github.New(github.Options.Secret(githubSecret))
		if err != nil {
			fmt.Printf("Could not create github webhook: %v\n", err)
			return
		}

		http.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			payload, err := hook.Parse(r, github.PushEvent)
			if err != nil {
				// if err == github.ErrEventNotFound {
				// 	fmt.Println("Event not found")
				// 	return
				// }
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

				return

			default:
				fmt.Printf("Event not handled: %+v\n", payload)
				return
			}
		})

		go func() {
			lis, err := net.Listen("tcp", "0.0.0.0:3005")
			if err != nil {
				log.Fatalf("failed to listen: %v", err)
			}

			grpcServer := grpc.NewServer()

			api.RegisterGithubServer(grpcServer, api.UnimplementedGithubServer{})

			if err := grpcServer.Serve(lis); err != nil {
				log.Fatalf("failed to serve: %v", err)
			}
		}()

		fmt.Println("listening for github webhooks callbacks")
		http.ListenAndServe(":3000", nil)
	}
}
