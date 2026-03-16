package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

type AnimeCard struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	CoverImage   string `json:"coverImage"`
	StatusLabel  string `json:"statusLabel,omitempty"`
	EpisodeLabel string `json:"episodeLabel,omitempty"`
	RatingLabel  string `json:"ratingLabel,omitempty"`
	ViewsLabel   string `json:"viewsLabel,omitempty"`
	Provider     string `json:"provider"`
}

type HomeFeed struct {
	Featured []AnimeCard `json:"featured"`
	Ongoing  []AnimeCard `json:"ongoing"`
	Recent   []AnimeCard `json:"recent"`
}

type AnimeDetail struct {
	ID              string            `json:"id"`
	Title           string            `json:"title"`
	Synopsis        string            `json:"synopsis"`
	CoverImage      string            `json:"coverImage"`
	BannerImage     string            `json:"bannerImage,omitempty"`
	Genres          []string          `json:"genres"`
	Metadata        map[string]string `json:"metadata"`
	Episodes        []Episode         `json:"episodes"`
	Recommendations []AnimeCard       `json:"recommendations"`
	Provider        string            `json:"provider"`
}

type Episode struct {
	ID         string `json:"id"`
	CatalogID  string `json:"catalogId"`
	Number     int    `json:"number"`
	Title      string `json:"title"`
	Label      string `json:"label,omitempty"`
	ReleasedAt string `json:"releasedAt,omitempty"`
}

type ScheduleDay struct {
	Day   string      `json:"day"`
	Items []AnimeCard `json:"items"`
}

type SelectOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type Pagination struct {
	CurrentPage int `json:"currentPage"`
	PrevPage    int `json:"prevPage,omitempty"`
	NextPage    int `json:"nextPage,omitempty"`
	TotalPages  int `json:"totalPages,omitempty"`
}

type PropertyItem struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type PropertyList struct {
	Kind  string         `json:"kind"`
	Title string         `json:"title"`
	Items []PropertyItem `json:"items"`
}

type CollectionPage struct {
	Kind          string         `json:"kind"`
	Key           string         `json:"key,omitempty"`
	Title         string         `json:"title"`
	Subtitle      string         `json:"subtitle,omitempty"`
	Items         []AnimeCard    `json:"items"`
	CurrentFilter string         `json:"currentFilter,omitempty"`
	CurrentOrder  string         `json:"currentOrder,omitempty"`
	FilterOptions []SelectOption `json:"filterOptions,omitempty"`
	OrderOptions  []SelectOption `json:"orderOptions,omitempty"`
	Pagination    Pagination     `json:"pagination"`
}

type StreamCandidate struct {
	URL               string `json:"url"`
	Container         string `json:"container"`
	Quality           string `json:"quality"`
	Codec             string `json:"codec,omitempty"`
	IsDirect          bool   `json:"isDirect"`
	Playable          bool   `json:"playable"`
	EstimatedPriority int    `json:"estimatedPriority"`
	Label             string `json:"label,omitempty"`
}

type StreamResult struct {
	EpisodeID       string            `json:"episodeId"`
	PreferredStream *StreamCandidate  `json:"preferredStream,omitempty"`
	Candidates      []StreamCandidate `json:"candidates"`
	SelectionReason string            `json:"selectionReason"`
	Provider        string            `json:"provider"`
}

type StartupMediaCache struct {
	EpisodeID     string `json:"episodeId"`
	SourceURL     string `json:"sourceUrl"`
	SourceKey     string `json:"sourceKey,omitempty"`
	Container     string `json:"container"`
	Quality       string `json:"quality"`
	ContentType   string `json:"contentType"`
	ContentLength int64  `json:"contentLength"`
	HeadKey       string `json:"headKey"`
	HeadBytes     int64  `json:"headBytes"`
	TailKey       string `json:"tailKey,omitempty"`
	TailBytes     int64  `json:"tailBytes,omitempty"`
}

type WatchlistItem struct {
	CatalogID   string    `json:"catalogId"`
	Title       string    `json:"title"`
	CoverImage  string    `json:"coverImage"`
	Provider    string    `json:"provider"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type HistoryItem struct {
	CatalogID        string    `json:"catalogId"`
	EpisodeID        string    `json:"episodeId"`
	PositionSeconds  int       `json:"positionSeconds"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

type ContinueWatchingItem struct {
	CatalogID        string    `json:"catalogId"`
	EpisodeID        string    `json:"episodeId"`
	Title            string    `json:"title"`
	CoverImage       string    `json:"coverImage"`
	PositionSeconds  int       `json:"positionSeconds"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

var ErrInvalidIdentifier = errors.New("invalid identifier")

func NewCatalogID(provider, slug string) string {
	return fmt.Sprintf("%s:%s", provider, strings.Trim(strings.TrimSpace(slug), "/"))
}

func ParseQualifiedID(value string) (provider, slug string, err error) {
	parts := strings.SplitN(strings.TrimSpace(value), ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", ErrInvalidIdentifier
	}
	return parts[0], strings.Trim(parts[1], "/"), nil
}
