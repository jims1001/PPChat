package user

import (
	"PProject/gen/user"
	"context"
)

type UserServiceProvider struct {
}

func (s *UserServiceProvider) GetUser(ctx context.Context, req *user.UserRequest) (*user.UserReply, error) {
	return &user.UserReply{
		Id:   req.Id,
		Name: "Alice",
	}, nil
}
