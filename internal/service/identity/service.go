package identity

import (
	"crypto/rand"
	"encoding/hex"
	"strings"

	"anime/develop/backend/internal/store/postgres"
)

type Service struct {
	store *postgres.Store
}

func New(store *postgres.Store) *Service {
	return &Service{store: store}
}

func (s *Service) Ensure(existing string) (*postgres.AnonymousClient, bool, error) {
	token := strings.TrimSpace(existing)
	if token == "" || len(token) < 16 {
		return s.store.EnsureClient(randomToken())
	}
	client, err := s.store.FindClient(token)
	if err == nil {
		return client, false, nil
	}
	return s.store.EnsureClient(randomToken())
}

func randomToken() string {
	buf := make([]byte, 24)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}
