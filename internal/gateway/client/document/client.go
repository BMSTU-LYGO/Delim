package document

import (
	"context"
	"fmt"
	"time"

	documentv1 "delim/pkg/gen/document/v1"
	"delim/pkg/grpcx"
	"google.golang.org/grpc"
)

const defaultTimeout = 15 * time.Second

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
	callCtx, cancel := withDeadline(ctx)
	defer cancel()
	response, err := c.client.Ping(callCtx, &documentv1.PingRequest{}, grpc.WaitForReady(true))
	if err != nil {
		return fmt.Errorf("document ping: %w", err)
	}
	if response.GetStatus() != "ok" {
		return fmt.Errorf("document ping: unexpected status %q", response.GetStatus())
	}
	return nil
}

func (c *Client) CreateReceipt(ctx context.Context, req *documentv1.CreateReceiptRequest) (*documentv1.CreateReceiptResponse, error) {
	callCtx, cancel := withDeadline(ctx)
	defer cancel()
	return c.client.CreateReceipt(callCtx, req)
}

func (c *Client) GetReceipt(ctx context.Context, req *documentv1.GetReceiptRequest) (*documentv1.GetReceiptResponse, error) {
	callCtx, cancel := withDeadline(ctx)
	defer cancel()
	return c.client.GetReceipt(callCtx, req)
}

func (c *Client) GetDocumentJob(ctx context.Context, req *documentv1.GetDocumentJobRequest) (*documentv1.GetDocumentJobResponse, error) {
	callCtx, cancel := withDeadline(ctx)
	defer cancel()
	return c.client.GetDocumentJob(callCtx, req)
}

func (c *Client) GetOCRResult(ctx context.Context, req *documentv1.GetOCRResultRequest) (*documentv1.GetOCRResultResponse, error) {
	callCtx, cancel := withDeadline(ctx)
	defer cancel()
	return c.client.GetOCRResult(callCtx, req)
}

func (c *Client) RetryReceiptOCR(ctx context.Context, req *documentv1.RetryReceiptOCRRequest) (*documentv1.RetryReceiptOCRResponse, error) {
	callCtx, cancel := withDeadline(ctx)
	defer cancel()
	return c.client.RetryReceiptOCR(callCtx, req)
}

func (c *Client) DeleteReceipt(ctx context.Context, req *documentv1.DeleteReceiptRequest) (*documentv1.DeleteReceiptResponse, error) {
	callCtx, cancel := withDeadline(ctx)
	defer cancel()
	return c.client.DeleteReceipt(callCtx, req)
}

func (c *Client) CreateExport(ctx context.Context, req *documentv1.CreateExportRequest) (*documentv1.CreateExportResponse, error) {
	callCtx, cancel := withDeadline(ctx)
	defer cancel()
	return c.client.CreateExport(callCtx, req)
}

func (c *Client) GetExport(ctx context.Context, req *documentv1.GetExportRequest) (*documentv1.GetExportResponse, error) {
	callCtx, cancel := withDeadline(ctx)
	defer cancel()
	return c.client.GetExport(callCtx, req)
}

func (c *Client) DownloadExport(ctx context.Context, req *documentv1.DownloadExportRequest) (grpc.ServerStreamingClient[documentv1.DownloadExportChunk], error) {
	callCtx, cancel := withDeadline(ctx)
	stream, err := c.client.DownloadExport(callCtx, req)
	if err != nil {
		cancel()
		return nil, err
	}
	return &downloadExportStream{ServerStreamingClient: stream, cancel: cancel}, nil
}

type downloadExportStream struct {
	grpc.ServerStreamingClient[documentv1.DownloadExportChunk]
	cancel context.CancelFunc
}

func (s *downloadExportStream) Recv() (*documentv1.DownloadExportChunk, error) {
	chunk, err := s.ServerStreamingClient.Recv()
	if err != nil {
		s.cancel()
	}
	return chunk, err
}

func withDeadline(ctx context.Context) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, defaultTimeout)
}

func (c *Client) Close() error {
	return c.conn.Close()
}
