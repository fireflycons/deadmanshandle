package adapters

import (
	"context"
	"errors"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	"github.com/fireflycons/deadmanshandle/internal/ports"
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

// GetDocument retrieves a document from S3, with IfMatch so that S3
// refuses (412 PreconditionFailed) if the object has been replaced
func (s *S3DocumentStore) GetDocument(ctx context.Context, bucket, key, etag string) ([]byte, error) {
	output, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket:  &bucket,
		Key:     &key,
		IfMatch: &etag,
	})
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) && apiErr.ErrorCode() == "PreconditionFailed" {
		return nil, ports.ErrDocumentChanged
	}
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = output.Body.Close()
	}()

	return io.ReadAll(output.Body)
}

// DocumentETag checks for the object with HeadObject. S3 only reports a
// missing object as NotFound when the caller may list the bucket; without
// s3:ListBucket it is AccessDenied, which is returned as an error.
func (s *S3DocumentStore) DocumentETag(ctx context.Context, bucket, key string) (string, bool, error) {
	output, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})
	if err == nil {
		return aws.ToString(output.ETag), true, nil
	}

	var notFound *types.NotFound
	if errors.As(err, &notFound) {
		return "", false, nil
	}
	return "", false, err
}
