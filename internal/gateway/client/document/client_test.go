package document

import (
	"context"
	"testing"

	documentv1 "delim/pkg/gen/document/v1"
	"google.golang.org/grpc/metadata"
)

func TestDownloadExportStreamCloseCancelsCall(t *testing.T) {
	t.Parallel()

	underlying := &stubDownloadExportStream{}
	cancelled := false
	stream := &downloadExportStream{
		ServerStreamingClient: underlying,
		cancel:                func() { cancelled = true },
		normalize:             func(err error) error { return err },
	}

	if err := stream.CloseSend(); err != nil {
		t.Fatalf("close stream: %v", err)
	}
	if !underlying.closed {
		t.Fatal("underlying stream was not closed")
	}
	if !cancelled {
		t.Fatal("stream context was not cancelled")
	}
}

type stubDownloadExportStream struct {
	closed bool
}

func (*stubDownloadExportStream) Recv() (*documentv1.DownloadExportChunk, error) {
	return nil, nil
}

func (*stubDownloadExportStream) Header() (metadata.MD, error) { return nil, nil }
func (*stubDownloadExportStream) Trailer() metadata.MD         { return nil }
func (s *stubDownloadExportStream) CloseSend() error {
	s.closed = true
	return nil
}
func (*stubDownloadExportStream) Context() context.Context { return context.Background() }
func (*stubDownloadExportStream) SendMsg(any) error        { return nil }
func (*stubDownloadExportStream) RecvMsg(any) error        { return nil }
