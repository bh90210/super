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

		buf := bytes.NewBuffer(resp.Data)
		dec := gob.NewDecoder(buf)

		switch resp.Hooktype.Type {
		case api.Hook_PUSH:
			var payload githubgoo.PushEvent
			err = dec.Decode(&payload)
			if err != nil {
				fmt.Printf("Could not decode payload: %v\n", err)
				continue
			}

			fmt.Printf("Received push event for repo: %s\n", *payload.Repo.FullName)
			spew.Dump(payload)

		case api.Hook_REGPACK:
			var payload githubgoo.RegistryPackageEvent
			err = dec.Decode(&payload)
			if err != nil {
				fmt.Printf("Could not decode payload: %v\n", err)
				continue
			}

			fmt.Printf("Received package event: %s for package: %s\n", payload.Action, *payload.RegistryPackage.Name)
			spew.Dump(payload)

		case api.Hook_RELEASE:
			var payload githubgoo.ReleaseEvent
			err = dec.Decode(&payload)
			if err != nil {
				fmt.Printf("Could not decode payload: %v\n", err)
				continue
			}

			fmt.Printf("Received release event for repo: %s, tag: %s\n", payload.Repo.FullName, payload.Release.TagName)
			spew.Dump(payload)

		}
	}
}
