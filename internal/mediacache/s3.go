package mediacache

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type s3Client interface {
	PutObject(ctx context.Context, bucket, key string, body []byte, contentType string) error
	GetObject(ctx context.Context, bucket, key string) (io.ReadCloser, error)
	HeadBucket(ctx context.Context, bucket string) error
	CreateBucket(ctx context.Context, bucket string) error
}

type s3ClientFactory func(cfg S3Config) (s3Client, error)

type S3Store struct {
	bucket string
	client s3Client
}

func NewS3Store(cfg S3Config, factory s3ClientFactory) (*S3Store, error) {
	if factory == nil {
		factory = newAWSv2S3Client
	}
	if strings.TrimSpace(cfg.Bucket) == "" {
		return nil, fmt.Errorf("dobject bucket is required")
	}

	client, err := factory(cfg)
	if err != nil {
		return nil, err
	}
	if cfg.AutoCreate {
		if err := client.HeadBucket(context.Background(), cfg.Bucket); err != nil {
			if createErr := client.CreateBucket(context.Background(), cfg.Bucket); createErr != nil {
				return nil, createErr
			}
		}
	}

	return &S3Store{
		bucket: cfg.Bucket,
		client: client,
	}, nil
}

func (s *S3Store) Put(ctx context.Context, key string, body []byte) error {
	if s == nil || s.client == nil {
		return fmt.Errorf("s3 store is not configured")
	}
	return s.client.PutObject(ctx, s.bucket, key, body, "application/octet-stream")
}

func (s *S3Store) Get(ctx context.Context, key string) ([]byte, error) {
	if s == nil || s.client == nil {
		return nil, fmt.Errorf("s3 store is not configured")
	}
	reader, err := s.client.GetObject(ctx, s.bucket, key)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	return body, nil
}

type awsV2S3Client struct {
	client *s3.Client
}

func newAWSv2S3Client(cfg S3Config) (s3Client, error) {
	region := strings.TrimSpace(cfg.Region)
	if region == "" {
		region = "us-east-1"
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(
		context.Background(),
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, "")),
	)
	if err != nil {
		return nil, err
	}

	client := s3.NewFromConfig(awsCfg, func(options *s3.Options) {
		options.BaseEndpoint = &cfg.EndpointURL
		options.UsePathStyle = cfg.ForcePath
		options.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
		options.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
	})
	return &awsV2S3Client{client: client}, nil
}

func (c *awsV2S3Client) PutObject(ctx context.Context, bucket, key string, body []byte, contentType string) error {
	_, err := c.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        &bucket,
		Key:           &key,
		Body:          bytes.NewReader(body),
		ContentType:   &contentType,
		ContentLength: ptrInt64(int64(len(body))),
	})
	return err
}

func (c *awsV2S3Client) GetObject(ctx context.Context, bucket, key string) (io.ReadCloser, error) {
	output, err := c.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})
	if err != nil {
		return nil, err
	}
	return output.Body, nil
}

func (c *awsV2S3Client) HeadBucket(ctx context.Context, bucket string) error {
	_, err := c.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: &bucket})
	return err
}

func (c *awsV2S3Client) CreateBucket(ctx context.Context, bucket string) error {
	_, err := c.client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: &bucket})
	return err
}

func ptrInt64(value int64) *int64 {
	return &value
}
