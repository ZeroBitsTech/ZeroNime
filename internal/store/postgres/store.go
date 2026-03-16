package postgres

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"anime/develop/backend/internal/domain"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Store struct {
	db *gorm.DB
}

type AnonymousClient struct {
	ID        uint   `gorm:"primaryKey"`
	Token     string `gorm:"uniqueIndex;size:128;not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (AnonymousClient) TableName() string {
	return "anonymous_clients_v2"
}

type Watchlist struct {
	ID         uint   `gorm:"primaryKey"`
	ClientID   uint   `gorm:"index;not null"`
	CatalogID  string `gorm:"index;size:255;not null"`
	Title      string `gorm:"size:255"`
	CoverImage string `gorm:"size:500"`
	Provider   string `gorm:"size:50"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (Watchlist) TableName() string {
	return "watchlists_v2"
}

type WatchHistory struct {
	ID              uint   `gorm:"primaryKey"`
	ClientID        uint   `gorm:"index;not null"`
	CatalogID       string `gorm:"index;size:255;not null"`
	EpisodeID       string `gorm:"size:255;not null"`
	Title           string `gorm:"size:255"`
	CoverImage      string `gorm:"size:500"`
	Provider        string `gorm:"size:50"`
	PositionSeconds int    `gorm:"not null"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (WatchHistory) TableName() string {
	return "watch_histories_v2"
}

type StreamCache struct {
	ID        uint      `gorm:"primaryKey"`
	EpisodeID string    `gorm:"uniqueIndex;size:255;not null"`
	Payload   string    `gorm:"type:jsonb;not null"`
	ExpiresAt time.Time `gorm:"index;not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (StreamCache) TableName() string {
	return "stream_caches_v2"
}

type CatalogCache struct {
	ID        uint   `gorm:"primaryKey"`
	CatalogID string `gorm:"uniqueIndex;size:255;not null"`
	Payload   string `gorm:"type:jsonb;not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (CatalogCache) TableName() string {
	return "catalog_caches_v2"
}

type DiscoveryCache struct {
	ID        uint   `gorm:"primaryKey"`
	CacheKey  string `gorm:"uniqueIndex;size:255;not null"`
	Payload   string `gorm:"type:jsonb;not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (DiscoveryCache) TableName() string {
	return "discovery_caches_v2"
}

type StartupMediaCache struct {
	ID        uint   `gorm:"primaryKey"`
	EpisodeID string `gorm:"uniqueIndex;size:255;not null"`
	Payload   string `gorm:"type:jsonb;not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (StartupMediaCache) TableName() string {
	return "startup_media_caches_v2"
}

type IDAlias struct {
	ID         uint   `gorm:"primaryKey"`
	Kind       string `gorm:"size:32;not null;index:idx_kind_internal,unique"`
	InternalID string `gorm:"size:255;not null;index:idx_kind_internal,unique"`
	PublicID   string `gorm:"size:36;not null;uniqueIndex"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (IDAlias) TableName() string {
	return "id_aliases_v2"
}

func Open(databaseURL string) (*Store, error) {
	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(&AnonymousClient{}, &Watchlist{}, &WatchHistory{}, &StreamCache{}, &CatalogCache{}, &DiscoveryCache{}, &StartupMediaCache{}, &IDAlias{}); err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Ping() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Ping()
}

func (s *Store) EnsureClient(token string) (*AnonymousClient, bool, error) {
	var client AnonymousClient
	err := s.db.Where("token = ?", token).First(&client).Error
	switch {
	case err == nil:
		return &client, false, nil
	case errors.Is(err, gorm.ErrRecordNotFound):
		client.Token = token
		if err := s.db.Create(&client).Error; err != nil {
			return nil, false, err
		}
		return &client, true, nil
	default:
		return nil, false, err
	}
}

func (s *Store) FindClient(token string) (*AnonymousClient, error) {
	var client AnonymousClient
	if err := s.db.Where("token = ?", token).First(&client).Error; err != nil {
		return nil, err
	}
	return &client, nil
}

func (s *Store) UpsertWatchlist(clientID uint, catalogID, title, coverImage, provider string) error {
	var row Watchlist
	err := s.db.Where("client_id = ? AND catalog_id = ?", clientID, catalogID).First(&row).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	row.ClientID = clientID
	row.CatalogID = catalogID
	row.Title = title
	row.CoverImage = coverImage
	row.Provider = provider
	return s.db.Save(&row).Error
}

func (s *Store) DeleteWatchlist(clientID uint, catalogID string) error {
	return s.db.Where("client_id = ? AND catalog_id = ?", clientID, catalogID).Delete(&Watchlist{}).Error
}

func (s *Store) ListWatchlist(clientID uint) ([]Watchlist, error) {
	var rows []Watchlist
	err := s.db.Where("client_id = ?", clientID).Order("updated_at DESC").Find(&rows).Error
	return rows, err
}

func (s *Store) UpsertHistory(clientID uint, catalogID, episodeID string, positionSeconds int, title, coverImage, provider string) error {
	var row WatchHistory
	err := s.db.Where("client_id = ? AND catalog_id = ?", clientID, catalogID).First(&row).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	row.ClientID = clientID
	row.CatalogID = catalogID
	row.EpisodeID = episodeID
	if row.Title == "" || strings.TrimSpace(title) != "" {
		row.Title = title
	}
	if row.CoverImage == "" || strings.TrimSpace(coverImage) != "" {
		row.CoverImage = coverImage
	}
	if row.Provider == "" || strings.TrimSpace(provider) != "" {
		row.Provider = provider
	}
	row.PositionSeconds = positionSeconds
	return s.db.Save(&row).Error
}

func (s *Store) DeleteHistory(clientID uint, catalogID string) error {
	return s.db.Where("client_id = ? AND catalog_id = ?", clientID, catalogID).Delete(&WatchHistory{}).Error
}

func (s *Store) ListHistory(clientID uint) ([]WatchHistory, error) {
	var rows []WatchHistory
	err := s.db.Where("client_id = ?", clientID).Order("updated_at DESC").Find(&rows).Error
	return rows, err
}

func (s *Store) GetStreamCache(episodeID string, now time.Time) (*domain.StreamResult, bool, error) {
	var row StreamCache
	err := s.db.Where("episode_id = ?", episodeID).First(&row).Error
	switch {
	case err == nil:
	case errors.Is(err, gorm.ErrRecordNotFound):
		return nil, false, nil
	default:
		return nil, false, err
	}

	if !row.ExpiresAt.After(now) {
		return nil, false, s.db.Delete(&row).Error
	}

	var result domain.StreamResult
	if err := json.Unmarshal([]byte(row.Payload), &result); err != nil {
		return nil, false, err
	}
	return &result, true, nil
}

func (s *Store) UpsertStreamCache(episodeID string, result domain.StreamResult, expiresAt time.Time) error {
	payload, err := json.Marshal(result)
	if err != nil {
		return err
	}

	var row StreamCache
	findErr := s.db.Where("episode_id = ?", episodeID).First(&row).Error
	if findErr != nil && !errors.Is(findErr, gorm.ErrRecordNotFound) {
		return findErr
	}

	row.EpisodeID = episodeID
	row.Payload = string(payload)
	row.ExpiresAt = expiresAt
	return s.db.Save(&row).Error
}

func (s *Store) DeleteStreamCache(episodeID string) error {
	return s.db.Where("episode_id = ?", episodeID).Delete(&StreamCache{}).Error
}

func (s *Store) GetCatalogCache(catalogID string) (*domain.AnimeDetail, bool, error) {
	var row CatalogCache
	err := s.db.Where("catalog_id = ?", catalogID).First(&row).Error
	switch {
	case err == nil:
	case errors.Is(err, gorm.ErrRecordNotFound):
		return nil, false, nil
	default:
		return nil, false, err
	}

	var result domain.AnimeDetail
	if err := json.Unmarshal([]byte(row.Payload), &result); err != nil {
		return nil, false, err
	}
	return &result, true, nil
}

func (s *Store) UpsertCatalogCache(catalogID string, detail domain.AnimeDetail) error {
	payload, err := json.Marshal(detail)
	if err != nil {
		return err
	}

	var row CatalogCache
	findErr := s.db.Where("catalog_id = ?", catalogID).First(&row).Error
	if findErr != nil && !errors.Is(findErr, gorm.ErrRecordNotFound) {
		return findErr
	}

	row.CatalogID = catalogID
	row.Payload = string(payload)
	return s.db.Save(&row).Error
}

func (s *Store) GetDiscoveryCache(cacheKey string, dest any) (bool, error) {
	var row DiscoveryCache
	err := s.db.Where("cache_key = ?", cacheKey).First(&row).Error
	switch {
	case err == nil:
	case errors.Is(err, gorm.ErrRecordNotFound):
		return false, nil
	default:
		return false, err
	}

	if err := json.Unmarshal([]byte(row.Payload), dest); err != nil {
		return false, err
	}
	return true, nil
}

func (s *Store) UpsertDiscoveryCache(cacheKey string, value any) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}

	var row DiscoveryCache
	findErr := s.db.Where("cache_key = ?", cacheKey).First(&row).Error
	if findErr != nil && !errors.Is(findErr, gorm.ErrRecordNotFound) {
		return findErr
	}

	row.CacheKey = cacheKey
	row.Payload = string(payload)
	return s.db.Save(&row).Error
}

func (s *Store) GetStartupMediaCache(episodeID string) (*domain.StartupMediaCache, bool, error) {
	var row StartupMediaCache
	err := s.db.Where("episode_id = ?", episodeID).First(&row).Error
	switch {
	case err == nil:
	case errors.Is(err, gorm.ErrRecordNotFound):
		return nil, false, nil
	default:
		return nil, false, err
	}

	var result domain.StartupMediaCache
	if err := json.Unmarshal([]byte(row.Payload), &result); err != nil {
		return nil, false, err
	}
	return &result, true, nil
}

func (s *Store) UpsertStartupMediaCache(entry domain.StartupMediaCache) error {
	payload, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	var row StartupMediaCache
	findErr := s.db.Where("episode_id = ?", entry.EpisodeID).First(&row).Error
	if findErr != nil && !errors.Is(findErr, gorm.ErrRecordNotFound) {
		return findErr
	}

	row.EpisodeID = entry.EpisodeID
	row.Payload = string(payload)
	return s.db.Save(&row).Error
}

func (s *Store) EnsureAlias(kind, internalID string) (string, error) {
	var row IDAlias
	err := s.db.Where("kind = ? AND internal_id = ?", kind, internalID).First(&row).Error
	switch {
	case err == nil:
		return row.PublicID, nil
	case !errors.Is(err, gorm.ErrRecordNotFound):
		return "", err
	}

	row.Kind = kind
	row.InternalID = internalID
	row.PublicID = newUUID()
	if saveErr := s.db.Create(&row).Error; saveErr != nil {
		if retryErr := s.db.Where("kind = ? AND internal_id = ?", kind, internalID).First(&row).Error; retryErr == nil {
			return row.PublicID, nil
		}
		return "", saveErr
	}
	return row.PublicID, nil
}

func (s *Store) ResolveAlias(kind, publicID string) (string, bool, error) {
	var row IDAlias
	err := s.db.Where("kind = ? AND public_id = ?", kind, publicID).First(&row).Error
	switch {
	case err == nil:
		return row.InternalID, true, nil
	case errors.Is(err, gorm.ErrRecordNotFound):
		return "", false, nil
	default:
		return "", false, err
	}
}

func newUUID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		panic(err)
	}
	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	return fmt.Sprintf(
		"%08x-%04x-%04x-%04x-%012x",
		bytes[0:4],
		bytes[4:6],
		bytes[6:8],
		bytes[8:10],
		bytes[10:16],
	)
}
