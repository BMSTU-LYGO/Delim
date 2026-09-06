package document

import (
	"context"
	"fmt"
	"time"

	documentv1 "delim/pkg/gen/document/v1"
	"delim/pkg/grpcx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/status"
)

const defaultTimeout = 10 * time.Second

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
	response, err := c.client.CreateReceipt(callCtx, req, grpc.WaitForReady(false))
	return response, c.normalizeUnavailable(err)
}

func (c *Client) GetReceipt(ctx context.Context, req *documentv1.GetReceiptRequest) (*documentv1.GetReceiptResponse, error) {
	callCtx, cancel := withDeadline(ctx)
	defer cancel()
	response, err := c.client.GetReceipt(callCtx, req, grpc.WaitForReady(false))
	return response, c.normalizeUnavailable(err)
}

func (c *Client) GetDocumentJob(ctx context.Context, req *documentv1.GetDocumentJobRequest) (*documentv1.GetDocumentJobResponse, error) {
	callCtx, cancel := withDeadline(ctx)
	defer cancel()
	response, err := c.client.GetDocumentJob(callCtx, req, grpc.WaitForReady(false))
	return response, c.normalizeUnavailable(err)
}

func (c *Client) GetOCRResult(ctx context.Context, req *documentv1.GetOCRResultRequest) (*documentv1.GetOCRResultResponse, error) {
	callCtx, cancel := withDeadline(ctx)
	defer cancel()
	response, err := c.client.GetOCRResult(callCtx, req, grpc.WaitForReady(false))
	return response, c.normalizeUnavailable(err)
}

func (c *Client) RetryReceiptOCR(ctx context.Context, req *documentv1.RetryReceiptOCRRequest) (*documentv1.RetryReceiptOCRResponse, error) {
	callCtx, cancel := withDeadline(ctx)
	defer cancel()
	response, err := c.client.RetryReceiptOCR(callCtx, req, grpc.WaitForReady(false))
	return response, c.normalizeUnavailable(err)
}

func (c *Client) DeleteReceipt(ctx context.Context, req *documentv1.DeleteReceiptRequest) (*documentv1.DeleteReceiptResponse, error) {
	callCtx, cancel := withDeadline(ctx)
	defer cancel()
	response, err := c.client.DeleteReceipt(callCtx, req, grpc.WaitForReady(false))
	return response, c.normalizeUnavailable(err)
}

func (c *Client) CreateExport(ctx context.Context, req *documentv1.CreateExportRequest) (*documentv1.CreateExportResponse, error) {
	callCtx, cancel := withDeadline(ctx)
	defer cancel()
	response, err := c.client.CreateExport(callCtx, req, grpc.WaitForReady(false))
	return response, c.normalizeUnavailable(err)
}

func (c *Client) GetExport(ctx context.Context, req *documentv1.GetExportRequest) (*documentv1.GetExportResponse, error) {
	callCtx, cancel := withDeadline(ctx)
	defer cancel()
	response, err := c.client.GetExport(callCtx, req, grpc.WaitForReady(false))
	return response, c.normalizeUnavailable(err)
}

func (c *Client) DownloadExport(ctx context.Context, req *documentv1.DownloadExportRequest) (grpc.ServerStreamingClient[documentv1.DownloadExportChunk], error) {
	callCtx, cancel := withDeadline(ctx)
	stream, err := c.client.DownloadExport(callCtx, req, grpc.WaitForReady(false))
	if err != nil {
		cancel()
		return nil, c.normalizeUnavailable(err)
	}
	return &downloadExportStream{ServerStreamingClient: stream, cancel: cancel}, nil
}

func (c *Client) normalizeUnavailable(err error) error {
	if status.Code(err) != codes.DeadlineExceeded {
		return err
	}
	state := c.conn.GetState()
	if state == connectivity.Ready {
		return err
	}
	return status.Error(codes.Unavailable, "document service is unavailable")
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
