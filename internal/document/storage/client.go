package storage

import (
	"fmt"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Client struct {
	S3     *minio.Client
	Bucket string
}

func New(endpoint, accessKey, secretKey, bucket string, useSSL bool) (*Client, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("create object storage client: %w", err)
	}
	return &Client{S3: client, Bucket: bucket}, nil
}
