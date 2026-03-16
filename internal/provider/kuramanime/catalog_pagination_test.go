package kuramanime

import "testing"

func TestExtractEpisodeRefsFromHTMLSupportsRanges(t *testing.T) {
	html := `
	<div id="episodeListsSection">
	  <a href="https://v17.kuramanime.ink/anime/1929/pokemon-2023/episode/1-2">Ep 1-2</a>
	  <a href="https://v17.kuramanime.ink/anime/1929/pokemon-2023/episode/3">Ep 3</a>
	  <a href="https://v17.kuramanime.ink/anime/1929/pokemon-2023/episode/131">Ep 131</a>
	</div>`

	refs := extractEpisodeRefsFromHTML(html, "1929/pokemon-2023")
	if len(refs) != 3 {
		t.Fatalf("expected 3 episode refs, got %d", len(refs))
	}

	if refs[0].Slug != "1929/pokemon-2023/episode/1-2" {
		t.Fatalf("expected first episode ref to keep range slug, got %q", refs[0].Slug)
	}

	if refs[0].Number != 1 {
		t.Fatalf("expected range episode to sort by first number, got %d", refs[0].Number)
	}
}

func TestExtractEpisodeRefsFromHTMLSupportsPopoverContent(t *testing.T) {
	html := `
	<a
	  id="episodeLists"
	  data-content="
	    <a class='btn' href='https://v17.kuramanime.ink/anime/1929/pokemon-2023/episode/1-2' target='_blank'>Ep 1-2 (Terlama)</a>
	    <a class='btn' href='https://v17.kuramanime.ink/anime/1929/pokemon-2023/episode/131' target='_blank'>Ep 131 (Terbaru)</a>
	  "
	>Daftar Episode</a>`

	refs := extractEpisodeRefsFromHTML(html, "1929/pokemon-2023")
	if len(refs) != 2 {
		t.Fatalf("expected 2 refs from popover content, got %d", len(refs))
	}

	if refs[0].Slug != "1929/pokemon-2023/episode/1-2" {
		t.Fatalf("expected first popover ref to keep range slug, got %q", refs[0].Slug)
	}
}

func TestExtractEpisodePageLinksFindsPaginationRoutes(t *testing.T) {
	html := `
	<div id="animeEpisodes">
	  <a href="/anime/1929/pokemon-2023/episode/1-2" class="active-ep ep-button">Ep 1-2</a>
	  <a href="/anime/1929/pokemon-2023/episode/3" class="ep-button">Ep 3</a>
	  <a href="/anime/1929/pokemon-2023/episode/7?page=2" class="page__link__episode">Next</a>
	  <a href="/anime/1929/pokemon-2023/episode/7?page=3" class="page__link__episode">Last</a>
	</div>`

	pageLinks := extractEpisodePageLinks(html)
	if len(pageLinks) != 2 {
		t.Fatalf("expected 2 page links, got %d", len(pageLinks))
	}

	if pageLinks[0] != "/anime/1929/pokemon-2023/episode/7?page=2" {
		t.Fatalf("unexpected first page link %q", pageLinks[0])
	}
}

func TestEpisodeDisplayValueUsesEpisodeSegmentFromSlug(t *testing.T) {
	value := episodeDisplayValue("", "1929/pokemon-2023/episode/1-2")
	if value != "1-2" {
		t.Fatalf("expected display value from episode slug to be 1-2, got %q", value)
	}
}
