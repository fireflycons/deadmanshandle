package adapters

import (
	"context"
	"errors"
	"io"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
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
	defer func() {
		_ = output.Body.Close()
	}()

	return io.ReadAll(output.Body)
}

// DocumentExists checks for the object with HeadObject. S3 only reports a
// missing object as NotFound when the caller may list the bucket; without
// s3:ListBucket it is AccessDenied, which is returned as an error.
func (s *S3DocumentStore) DocumentExists(ctx context.Context, bucket, key string) (bool, error) {
	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})
	if err == nil {
		return true, nil
	}

	var notFound *types.NotFound
	if errors.As(err, &notFound) {
		return false, nil
	}
	return false, err
}
