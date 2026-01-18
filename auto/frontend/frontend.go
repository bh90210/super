// Package frontend provides code for Gihub webhooks.
package frontend

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sync"

	"github.com/bh90210/super/auto/api"
	"github.com/go-playground/webhooks/v6/github"
	githubgoo "github.com/google/go-github/v81/github"
)

var _ api.GithubServer = (*Service)(nil)

type Service struct {
	api.UnimplementedGithubServer

	stream api.Github_WebhookServer
	wg     *sync.WaitGroup
	mu     sync.Mutex
}

func NewService() (*Service, error) {
	return &Service{}, nil
}

func (s *Service) Webhook(_ *api.Empty, stream api.Github_WebhookServer) error {
	s.mu.Lock()
	s.stream = stream
	wg := &sync.WaitGroup{}
	s.wg = wg
	wg.Add(1)
	s.mu.Unlock()

	wg.Wait()

	return nil
}

func (s *Service) Broadcast(hooktype api.Hook_Type, data []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.stream != nil {
		err := s.stream.Send(&api.WebhookResponse{
			Hooktype: &api.Hook{
				Type: hooktype,
			},
			Data: data,
		})
		if err != nil {
			slog.Error("could not send webhook response", slog.String("error", err.Error()))

			if s.wg != nil {
				s.wg.Done()
			}
		}
	}
}

func HandleFunc(hook *github.Webhook, service *Service) func(w http.ResponseWriter, r *http.Request) {
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
			payloadBytes, err := json.Marshal(payload)
			if err != nil {
				slog.Error("could not marshal push event", slog.String("error", err.Error()))
				return
			}

			service.Broadcast(api.Hook_PUSH, payloadBytes)

			return

		case githubgoo.ReleaseEvent:
			payloadBytes, err := json.Marshal(payload)
			if err != nil {
				slog.Error("could not marshal release event", slog.String("error", err.Error()))
				return
			}

			service.Broadcast(api.Hook_RELEASE, payloadBytes)

		case githubgoo.RegistryPackageEvent:
			payloadBytes, err := json.Marshal(payload)
			if err != nil {
				slog.Error("could not marshal registry package event", slog.String("error", err.Error()))
				return
			}

			service.Broadcast(api.Hook_REGPACK, payloadBytes)
		}
	}
}
