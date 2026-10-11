// Package s3 provides a thin wrapper around the AWS S3 SDK v2.
// All operations use context for cancellation and timeout propagation.
package s3

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/kithly/kithly-ws-go/internal/core/config"
)

// CloudFrontBaseURL is prepended to bare S3 keys in API responses.
// S3 keys are stored without this prefix in the database.
const CloudFrontBaseURL = "https://d2y1zi6ybjmm4o.cloudfront.net/"

// Client wraps the AWS S3 client with application-level helpers.
type Client struct {
	s3     *awss3.Client
	bucket string
}

// New creates an S3 Client using credentials from cfg.
func New(ctx context.Context, cfg *config.Config) (*Client, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(cfg.AWSRegion),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AWSAccessKeyID,
			cfg.AWSSecretAccessKey,
			"",
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("s3.New: load aws config: %w", err)
	}
	return &Client{
		s3:     awss3.NewFromConfig(awsCfg),
		bucket: cfg.S3Bucket,
	}, nil
}

// UploadFile uploads body to the given S3 key with the specified content type.
// Uses the multipart manager for large files.
func (c *Client) UploadFile(ctx context.Context, key, contentType string, body io.Reader) error {
	uploader := manager.NewUploader(c.s3)
	_, err := uploader.Upload(ctx, &awss3.PutObjectInput{
		Bucket:      aws.String(c.bucket),
		Key:         aws.String(key),
		Body:        body,
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("s3.UploadFile %q: %w", key, err)
	}
	return nil
}

// DownloadFile downloads the object at key and returns its body.
// The caller must close the returned ReadCloser.
func (c *Client) DownloadFile(ctx context.Context, key string) (io.ReadCloser, error) {
	out, err := c.s3.GetObject(ctx, &awss3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("s3.DownloadFile %q: %w", key, err)
	}
	return out.Body, nil
}

// PresignGetURL generates a pre-signed GET URL for the given key.
// FFmpeg and other HTTP clients can use it to stream the object directly
// without downloading it to disk first. The URL expires after expiresIn.
func (c *Client) PresignGetURL(ctx context.Context, key string, expiresIn time.Duration) (string, error) {
	presign := awss3.NewPresignClient(c.s3)
	req, err := presign.PresignGetObject(ctx, &awss3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	}, awss3.WithPresignExpires(expiresIn))
	if err != nil {
		return "", fmt.Errorf("s3.PresignGetURL %q: %w", key, err)
	}
	return req.URL, nil
}

// PresignedUploadURL generates a pre-signed PUT URL for the given key.
// The URL expires after expiresIn.
func (c *Client) PresignedUploadURL(ctx context.Context, key, contentType string, expiresIn time.Duration) (string, error) {
	presign := awss3.NewPresignClient(c.s3)
	req, err := presign.PresignPutObject(ctx, &awss3.PutObjectInput{
		Bucket:      aws.String(c.bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	}, awss3.WithPresignExpires(expiresIn))
	if err != nil {
		return "", fmt.Errorf("s3.PresignedUploadURL %q: %w", key, err)
	}
	return req.URL, nil
}

// DeleteFile removes the object at key.
func (c *Client) DeleteFile(ctx context.Context, key string) error {
	_, err := c.s3.DeleteObject(ctx, &awss3.DeleteObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("s3.DeleteFile %q: %w", key, err)
	}
	return nil
}

// CloudFrontURL prepends the CloudFront base URL to a bare S3 key.
// Returns an empty string if key is empty.
func CloudFrontURL(key string) string {
	if key == "" {
		return ""
	}
	return CloudFrontBaseURL + key
}
