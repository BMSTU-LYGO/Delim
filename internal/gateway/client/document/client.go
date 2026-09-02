package document

import (
	"context"
	"fmt"

	documentv1 "delim/pkg/gen/document/v1"
	"delim/pkg/grpcx"
	"google.golang.org/grpc"
)

type Client struct {
	conn   *grpc.ClientConn
	client documentv1.DocumentServiceClient
}

func New(address string) (*Client, error) {
	conn, err := grpcx.NewClient(address)
	if err != nil {
		return nil, err
	}
	return &Client{conn: conn, client: documentv1.NewDocumentServiceClient(conn)}, nil
}

func (c *Client) Ping(ctx context.Context) error {
	response, err := c.client.Ping(ctx, &documentv1.PingRequest{}, grpc.WaitForReady(true))
	if err != nil {
		return fmt.Errorf("document ping: %w", err)
	}
	if response.GetStatus() != "ok" {
		return fmt.Errorf("document ping: unexpected status %q", response.GetStatus())
	}
	return nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}
