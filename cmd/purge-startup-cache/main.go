package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"anime/develop/backend/internal/config"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

func main() {
	cfg := config.Load()
	if !cfg.DObjectConfigured() {
		log.Fatal("dobject is not configured")
	}

	client, err := newS3Client(cfg)
	if err != nil {
		log.Fatalf("create s3 client: %v", err)
	}

	ctx := context.Background()
	var deleted int
	pager := s3.NewListObjectsV2Paginator(client, &s3.ListObjectsV2Input{
		Bucket: aws.String(cfg.DObjectBucket),
	})

	for pager.HasMorePages() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			log.Fatalf("list objects: %v", err)
		}
		if len(page.Contents) == 0 {
			continue
		}

		objects := make([]types.ObjectIdentifier, 0, len(page.Contents))
		for _, item := range page.Contents {
			if item.Key == nil || strings.TrimSpace(*item.Key) == "" {
				continue
			}
			objects = append(objects, types.ObjectIdentifier{Key: item.Key})
		}
		if len(objects) == 0 {
			continue
		}

		out, err := client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: aws.String(cfg.DObjectBucket),
			Delete: &types.Delete{Objects: objects, Quiet: aws.Bool(true)},
		})
		if err != nil {
			log.Fatalf("delete objects: %v", err)
		}
		deleted += len(out.Deleted)
	}

	fmt.Printf("bucket=%s deleted=%d\n", cfg.DObjectBucket, deleted)
}

func newS3Client(cfg config.Config) (*s3.Client, error) {
	region := strings.TrimSpace(cfg.DObjectRegion)
	if region == "" {
		region = "us-east-1"
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(
		context.Background(),
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.DObjectAccessKey, cfg.DObjectSecretKey, "")),
	)
	if err != nil {
		return nil, err
	}

	return s3.NewFromConfig(awsCfg, func(options *s3.Options) {
		options.BaseEndpoint = &cfg.DObjectURL
		options.UsePathStyle = cfg.DObjectForcePath
		options.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
		options.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
	}), nil
}
