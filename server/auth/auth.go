package auth

import (
	"context"

	"github.com/bh90210/super/server/api"
)

type Authentication struct {
	api.UnimplementedAuthServiceServer
}

var _ api.AuthServiceServer = (*Authentication)(nil)

func NewAuthentication() *Authentication {
	return &Authentication{}
}

func (a *Authentication) InitializeFlow(ctx context.Context, req *api.InitializeFlowRequest) (*api.InitializeFlowResponse, error) {
	return &api.InitializeFlowResponse{}, nil
}

func (a *Authentication) RequestLoginCode(ctx context.Context, req *api.RequestLoginCodeRequest) (*api.RequestLoginCodeResponse, error) {
	return &api.RequestLoginCodeResponse{}, nil
}

func (a *Authentication) SubmitAuth(ctx context.Context, req *api.SubmitAuthRequest) (*api.SubmitAuthResponse, error) {
	return &api.SubmitAuthResponse{}, nil
}

func (a *Authentication) WhoAmI(ctx context.Context, req *api.WhoAmIRequest) (*api.WhoAmIResponse, error) {
	return &api.WhoAmIResponse{}, nil
}

func (a *Authentication) Logout(ctx context.Context, req *api.LogoutRequest) (*api.LogoutResponse, error) {
	return &api.LogoutResponse{}, nil
}

func (a *Authentication) InviteUser(ctx context.Context, req *api.InviteUserRequest) (*api.InviteUserResponse, error) {
	return &api.InviteUserResponse{}, nil
}
