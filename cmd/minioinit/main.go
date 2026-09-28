// Command minioinit creates the private object-storage bucket used by Document.
package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	endpoint := envOr("MINIO_ENDPOINT", "minio:9000")
	bucket := envOr("MINIO_BUCKET", "receipts")
	accessKey := strings.TrimSpace(os.Getenv("MINIO_ROOT_USER"))
	secretKey := strings.TrimSpace(os.Getenv("MINIO_ROOT_PASSWORD"))
	if accessKey == "" || secretKey == "" {
		return fmt.Errorf("MINIO_ROOT_USER and MINIO_ROOT_PASSWORD are required")
	}

	secure := strings.EqualFold(envOr("MINIO_USE_SSL", "false"), "true")
	client, err := minio.New(endpoint, &minio.Options{
		Creds:     credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure:    secure,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}},
	})
	if err != nil {
		return fmt.Errorf("create MinIO client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		return fmt.Errorf("check bucket %q: %w", bucket, err)
	}
	if !exists {
		if err := client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
			return fmt.Errorf("create bucket %q: %w", bucket, err)
		}
	}
	if err := client.SetBucketPolicy(ctx, bucket, ""); err != nil {
		return fmt.Errorf("make bucket %q private: %w", bucket, err)
	}
	fmt.Printf("MinIO bucket %q is ready and private\n", bucket)
	return nil
}

func envOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
