package kuramanime

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func TestDetailCoverFromDocFallsBackToMobileCover(t *testing.T) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(`
		<html>
			<body>
				<div class="anime__details__pic__mobile" data-setbg="https://r2.nyomo.my.id/images/example.jpg"></div>
			</body>
		</html>
	`))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if got := detailCoverFromDoc(doc); got != "https://r2.nyomo.my.id/images/example.jpg" {
		t.Fatalf("expected mobile detail cover fallback, got %q", got)
	}
}

func TestPreferredDetailCoverUsesAlternativeWhenPrimaryIsMyAnimeList(t *testing.T) {
	got := preferredDetailCover(
		"https://cdn.myanimelist.net/images/anime/1222/139173l.jpg",
		"https://r2.nyomo.my.id/images/example.jpg",
	)

	if got != "https://r2.nyomo.my.id/images/example.jpg" {
		t.Fatalf("expected alternative cover, got %q", got)
	}
}
