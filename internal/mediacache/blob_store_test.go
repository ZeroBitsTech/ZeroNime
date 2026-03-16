package mediacache

import (
	"bytes"
	"context"
	"io"
	"testing"

	"anime/develop/backend/internal/config"
)

type fakeS3Client struct {
	putBucket string
	putKey    string
	putBody   []byte
	getBucket string
	getKey    string
	getBody   []byte
}

func (f *fakeS3Client) PutObject(_ context.Context, bucket, key string, body []byte, contentType string) error {
	f.putBucket = bucket
	f.putKey = key
	f.putBody = append([]byte(nil), body...)
	return nil
}

func (f *fakeS3Client) GetObject(_ context.Context, bucket, key string) (io.ReadCloser, error) {
	f.getBucket = bucket
	f.getKey = key
	return io.NopCloser(bytes.NewReader(f.getBody)), nil
}

func (f *fakeS3Client) HeadBucket(_ context.Context, bucket string) error {
	return nil
}

func (f *fakeS3Client) CreateBucket(_ context.Context, bucket string) error {
	return nil
}

func TestS3StorePutAndGet(t *testing.T) {
	t.Parallel()

	client := &fakeS3Client{getBody: []byte("cached-object")}
	store := &S3Store{
		bucket: "zeronime-cache",
		client: client,
	}

	if err := store.Put(context.Background(), "startup-media/episode-1/head.bin", []byte("hello")); err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	if client.putBucket != "zeronime-cache" {
		t.Fatalf("put bucket = %q", client.putBucket)
	}
	if client.putKey != "startup-media/episode-1/head.bin" {
		t.Fatalf("put key = %q", client.putKey)
	}
	if string(client.putBody) != "hello" {
		t.Fatalf("put body = %q", string(client.putBody))
	}

	body, err := store.Get(context.Background(), "startup-media/episode-1/head.bin")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if client.getBucket != "zeronime-cache" {
		t.Fatalf("get bucket = %q", client.getBucket)
	}
	if client.getKey != "startup-media/episode-1/head.bin" {
		t.Fatalf("get key = %q", client.getKey)
	}
	if string(body) != "cached-object" {
		t.Fatalf("body = %q", string(body))
	}
}

func TestNewBlobStorePrefersFilesystemWhenDObjectIncomplete(t *testing.T) {
	t.Parallel()

	store, err := NewBlobStore(config.Config{MediaCacheDir: "./var/media-cache"})
	if err != nil {
		t.Fatalf("NewBlobStore() error = %v", err)
	}

	if _, ok := store.(*FilesystemStore); !ok {
		t.Fatalf("store type = %T, want *FilesystemStore", store)
	}
}

func TestNewBlobStoreUsesS3WhenDObjectConfigured(t *testing.T) {
	t.Parallel()

	store, err := NewBlobStore(config.Config{
		MediaCacheDir:       "./var/media-cache",
		DObjectURL:          "https://s3.example.com",
		DObjectAccessKey:    "access",
		DObjectSecretKey:    "secret",
		DObjectBucket:       "zeronime-cache",
		DObjectRegion:       "us-east-1",
		DObjectForcePath:    true,
		DObjectAutoCreate:   false,
		DObjectUseWhenReady: true,
	}, WithS3ClientFactory(func(cfg S3Config) (s3Client, error) {
		return &fakeS3Client{}, nil
	}))
	if err != nil {
		t.Fatalf("NewBlobStore() error = %v", err)
	}

	tiered, ok := store.(*TieredStore)
	if !ok {
		t.Fatalf("store type = %T, want *TieredStore", store)
	}
	if _, ok := tiered.local.(*FilesystemStore); !ok {
		t.Fatalf("local store type = %T, want *FilesystemStore", tiered.local)
	}
	if _, ok := tiered.remote.(*S3Store); !ok {
		t.Fatalf("remote store type = %T, want *S3Store", tiered.remote)
	}
}

func TestTieredStoreReadsThroughLocalCache(t *testing.T) {
	t.Parallel()

	localDir := t.TempDir()
	local := NewFilesystemStore(localDir)
	remote := &S3Store{
		bucket: "zeronime-cache",
		client: &fakeS3Client{getBody: []byte("from-remote")},
	}
	store := NewTieredStore(local, remote)

	body, err := store.Get(context.Background(), "startup-media/episode-2/head.bin")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if string(body) != "from-remote" {
		t.Fatalf("body = %q", string(body))
	}

	cachedBody, err := local.Get(context.Background(), "startup-media/episode-2/head.bin")
	if err != nil {
		t.Fatalf("local Get() error = %v", err)
	}
	if string(cachedBody) != "from-remote" {
		t.Fatalf("cached body = %q", string(cachedBody))
	}
}

func TestTieredStorePutWritesLocalAndRemote(t *testing.T) {
	t.Parallel()

	localDir := t.TempDir()
	local := NewFilesystemStore(localDir)
	fakeRemote := &fakeS3Client{}
	remote := &S3Store{
		bucket: "zeronime-cache",
		client: fakeRemote,
	}
	store := NewTieredStore(local, remote)

	if err := store.Put(context.Background(), "startup-media/episode-3/head.bin", []byte("hello-tiered")); err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	if string(fakeRemote.putBody) != "hello-tiered" {
		t.Fatalf("remote put body = %q", string(fakeRemote.putBody))
	}

	localBody, err := local.Get(context.Background(), "startup-media/episode-3/head.bin")
	if err != nil {
		t.Fatalf("local Get() error = %v", err)
	}
	if string(localBody) != "hello-tiered" {
		t.Fatalf("local body = %q", string(localBody))
	}
}
