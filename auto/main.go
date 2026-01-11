package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/go-playground/webhooks/v6/github"
)

const (
	path = "/webhooks"
)

func main() {
	isServer := flag.Bool("server", false, "Run as server. Default is false (run as client).")
	flag.Parse()

	switch *isServer {
	case true:
		//

	case false:
		githubSecret := os.Getenv("GITHUB_SECRET")
		hook, _ := github.New(github.Options.Secret(githubSecret))

		http.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			payload, err := hook.Parse(r, github.PushEvent)
			if err != nil {
				if err == github.ErrEventNotFound {
					// ok event wasn;t one of the ones asked to be parsed
				}
			}

			switch payload := payload.(type) {
			case github.PushPayload:
				// Do whatever you want from here...
				fmt.Printf("%+v", payload)

			default:
				fmt.Printf("Event not handled: %T", payload)
			}
		})

		fmt.Println("listening for github webhooks callbacks")
		http.ListenAndServe(":3000", nil)
	}
}
