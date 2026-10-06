package adapters

import (
	"context"
	"io"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3DocumentStore implements the DocumentStore interface using AWS S3
type S3DocumentStore struct {
	client *s3.Client
}

// NewS3DocumentStore creates a new S3-based document store
func NewS3DocumentStore(client *s3.Client) *S3DocumentStore {
	return &S3DocumentStore{
		client: client,
	}
}

// GetDocument retrieves a document from S3
func (s *S3DocumentStore) GetDocument(ctx context.Context, bucket, key string) ([]byte, error) {
	output, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})
	if err != nil {
		return nil, err
	}
	defer output.Body.Close()

	return io.ReadAll(output.Body)
}
