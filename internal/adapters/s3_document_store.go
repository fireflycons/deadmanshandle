package adapters

import (
	"context"

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

	// Read the entire object into memory
	data := make([]byte, 0)
	if output.ContentLength != nil {
		data = make([]byte, 0, *output.ContentLength)
	}

	buf := make([]byte, 1024*1024) // 1MB chunks
	for {
		n, err := output.Body.Read(buf)
		if n > 0 {
			data = append(data, buf[:n]...)
		}
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return nil, err
		}
	}

	return data, nil
}
