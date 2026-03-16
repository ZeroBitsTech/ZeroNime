package mediacache

import "context"

type TieredStore struct {
	local  BlobStore
	remote BlobStore
}

func NewTieredStore(local, remote BlobStore) *TieredStore {
	return &TieredStore{
		local:  local,
		remote: remote,
	}
}

func (s *TieredStore) Put(ctx context.Context, key string, body []byte) error {
	if s == nil {
		return nil
	}
	if s.remote != nil {
		if err := s.remote.Put(ctx, key, body); err != nil {
			return err
		}
	}
	if s.local != nil {
		_ = s.local.Put(ctx, key, body)
	}
	return nil
}

func (s *TieredStore) Get(ctx context.Context, key string) ([]byte, error) {
	if s == nil {
		return nil, nil
	}
	if s.local != nil {
		if body, err := s.local.Get(ctx, key); err == nil {
			return body, nil
		}
	}
	if s.remote == nil {
		return nil, nil
	}
	body, err := s.remote.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if s.local != nil {
		_ = s.local.Put(ctx, key, body)
	}
	return body, nil
}
