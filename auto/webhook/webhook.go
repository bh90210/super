// Package webhook provides code for Gihub webhooks.
package webhook

import (
	"sync"

	"github.com/bh90210/super/auto/api"
)

var _ api.GithubServer = (*Service)(nil)

type Service struct {
	api.UnimplementedGithubServer

	streams []api.Github_WebhookServer
	mu      sync.Mutex
}

func NewService() (*Service, error) {
	return &Service{}, nil
}

func (s *Service) Webhook(_ *api.Empty, stream api.Github_WebhookServer) error {
	s.mu.Lock()
	s.streams = append(s.streams, stream)
	s.mu.Unlock()

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
	}
}
