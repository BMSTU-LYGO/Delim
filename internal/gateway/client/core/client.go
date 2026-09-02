package core

import (
	"context"
	"fmt"

	corev1 "delim/pkg/gen/core/v1"
	"delim/pkg/grpcx"
	"google.golang.org/grpc"
)

type Client struct {
	conn   *grpc.ClientConn
	client corev1.CoreServiceClient
}

func New(address string) (*Client, error) {
	conn, err := grpcx.NewClient(address)
	if err != nil {
		return nil, err
	}
	return &Client{conn: conn, client: corev1.NewCoreServiceClient(conn)}, nil
}

func (c *Client) Ping(ctx context.Context) error {
	response, err := c.client.Ping(ctx, &corev1.PingRequest{}, grpc.WaitForReady(true))
	if err != nil {
		return fmt.Errorf("core ping: %w", err)
	}
	if response.GetStatus() != "ok" {
		return fmt.Errorf("core ping: unexpected status %q", response.GetStatus())
	}
	return nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}
