package userinfo

import (
	"context"
	"strconv"

	pbu "drova/shared/proto/user"
)

type Client struct {
	grpc pbu.UserServiceClient
}

func New(grpc pbu.UserServiceClient) *Client {
	return &Client{grpc: grpc}
}

func (c *Client) GetRiderInfo(ctx context.Context, userID string) (name, avatar string) {
	id, err := strconv.ParseInt(userID, 10, 64)
	if err != nil {
		return "", ""
	}
	resp, err := c.grpc.GetUser(ctx, &pbu.GetUserRequest{UserId: id})
	if err != nil {
		return "", ""
	}
	return resp.GetDisplayName(), resp.GetAvatarUrl()
}
