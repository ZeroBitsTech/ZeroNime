package mediacache

import (
	"anime/develop/backend/internal/config"
)

type S3Config struct {
	EndpointURL string
	AccessKey   string
	SecretKey   string
	Bucket      string
	Region      string
	ForcePath   bool
	AutoCreate  bool
}

type blobStoreOptions struct {
	s3Factory s3ClientFactory
}

type BlobStoreOption func(*blobStoreOptions)

func WithS3ClientFactory(factory s3ClientFactory) BlobStoreOption {
	return func(options *blobStoreOptions) {
		options.s3Factory = factory
	}
}

func NewBlobStore(cfg config.Config, opts ...BlobStoreOption) (BlobStore, error) {
	options := blobStoreOptions{
		s3Factory: newAWSv2S3Client,
	}
	for _, opt := range opts {
		opt(&options)
	}

	if cfg.DObjectUseWhenReady && cfg.DObjectConfigured() {
		localStore := NewFilesystemStore(cfg.MediaCacheDir)
		s3Store, err := NewS3Store(S3Config{
			EndpointURL: cfg.DObjectURL,
			AccessKey:   cfg.DObjectAccessKey,
			SecretKey:   cfg.DObjectSecretKey,
			Bucket:      cfg.DObjectBucket,
			Region:      cfg.DObjectRegion,
			ForcePath:   cfg.DObjectForcePath,
			AutoCreate:  cfg.DObjectAutoCreate,
		}, options.s3Factory)
		if err != nil {
			return nil, err
		}
		return NewTieredStore(localStore, s3Store), nil
	}

	return NewFilesystemStore(cfg.MediaCacheDir), nil
}
