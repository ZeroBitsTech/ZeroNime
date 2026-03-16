package kuramanime

import (
	"testing"

	"anime/develop/backend/internal/domain"
)

func TestParsePropertyLinksExtractsGenreItems(t *testing.T) {
	html := `
	<div id="animeList" class="row">
	  <div class="container">
	    <div class="kuramanime__genres">
	      <ul>
	        <li><a href="https://v17.kuramanime.ink/properties/genre/action?order_by=text&amp;from_property=1"><span>Action</span></a></li>
	        <li><a href="https://v17.kuramanime.ink/properties/genre/adventure?order_by=text&amp;from_property=1"><span>Adventure</span></a></li>
	      </ul>
	    </div>
	  </div>
	</div>`

	items := parsePropertyLinks(html, "genre")
	if len(items) != 2 {
		t.Fatalf("expected 2 property items, got %d", len(items))
	}

	if items[0].ID != "action" || items[0].Title != "Action" {
		t.Fatalf("unexpected first property item: %+v", items[0])
	}
}

func TestParseDiscoveryOrderOptionsExtractsCurrentOrderAndPagination(t *testing.T) {
	html := `
	<div class="product__page__filter">
	  <form id="filterForm">
	    <select id="filterAnime" name="order_by">
	      <option value="https://v17.kuramanime.ink/properties/genre/action?order_by=ascending&page=1" selected>A-Z</option>
	      <option value="https://v17.kuramanime.ink/properties/genre/action?order_by=descending&page=1">Z-A</option>
	      <option value="https://v17.kuramanime.ink/properties/genre/action?order_by=most_viewed&page=1">Terpopuler</option>
	    </select>
	  </form>
	</div>
	<div class="product__pagination">
	  <a aria-disabled="true" class="gray__color"><i class="fa fa-angle-left"></i></a>
	  <a class="current-page">1</a>
	  <a href="/properties/genre/action?order_by=ascending&page=2" class="page__link">2</a>
	  <a href="/properties/genre/action?order_by=ascending&page=3" class="page__link">3</a>
	  <a href="/properties/genre/action?order_by=ascending&page=76" class="page__link">76</a>
	  <a href="/properties/genre/action?order_by=ascending&page=2" class="page__link"><i class="fa fa-angle-right"></i></a>
	</div>`

	options, current, pagination := parseDiscoveryControls(html)
	if len(options) != 3 {
		t.Fatalf("expected 3 order options, got %d", len(options))
	}
	if current != "ascending" {
		t.Fatalf("expected current order ascending, got %q", current)
	}
	if pagination.CurrentPage != 1 || pagination.NextPage != 2 || pagination.TotalPages != 76 {
		t.Fatalf("unexpected pagination: %+v", pagination)
	}
}

func TestParseScheduleCardsUsesScheduleMeta(t *testing.T) {
	html := `
	<div id="animeList" class="row">
	  <input type="hidden" class="actual-schedule-ep-1480-real" value="76">
	  <div class="col-lg-4 col-md-6 col-sm-6">
	    <div class="product__item">
	      <a href="https://v17.kuramanime.ink/anime/1480/urusei-yatsura">
	        <div class="product__item__pic set-bg" data-setbg="https://img.test/cover.jpg">
	          <div class="ep">
	            <span class="actual-schedule-ep-1480" style="display: none;">Selanjutnya: Ep 76</span>
	          </div>
	          <div class="view-end">
	            <ul>
	              <li><i class="fa fa-calendar"></i><span class="actual-schedule-info-1480">Tidak Tentu</span></li>
	              <li><i class="fa fa-clock"></i><span class="actual-schedule-info-1480">19:00 WIB</span></li>
	            </ul>
	          </div>
	        </div>
	      </a>
	      <div class="product__item__text">
	        <ul>
	          <a href="https://v17.kuramanime.ink/properties/type/TV"><li>TV</li></a>
	          <a href="https://v17.kuramanime.ink/properties/quality/BD"><li>BD</li></a>
	        </ul>
	        <h5><a href="https://v17.kuramanime.ink/anime/1480/urusei-yatsura">Urusei Yatsura</a></h5>
	      </div>
	    </div>
	  </div>
	</div>`

	items := parseScheduleCardsHTML(html)
	if len(items) != 1 {
		t.Fatalf("expected 1 schedule card, got %d", len(items))
	}

	if items[0].EpisodeLabel != "Ep 76" {
		t.Fatalf("expected episode label Ep 76, got %q", items[0].EpisodeLabel)
	}

	if items[0].StatusLabel != "Tidak Tentu" {
		t.Fatalf("expected day label, got %q", items[0].StatusLabel)
	}
}

func TestParseTextModeCardsExtractsAnimeLinks(t *testing.T) {
	html := `
	<div id="animeList" class="row">
	  <div class="col-lg-12 col-md-12 col-sm-12">
	    <h5 class="text-white mt-4" id="A">A</h5>
	  </div>
	  <div class="col-lg-6 col-md-6 col-sm-6 anime__text">
	    <a href="https://v17.kuramanime.ink/anime/833/ai-no-utagoe-wo-kikasete" class="anime__list__link" target="_blank">
	      Ai no Utagoe wo Kikasete
	    </a>
	  </div>
	</div>`

	items := parseTextModeCards(html)
	if len(items) != 1 {
		t.Fatalf("expected 1 text mode item, got %d", len(items))
	}

	if items[0].ID != catalogIDFromSlug("833/ai-no-utagoe-wo-kikasete") {
		t.Fatalf("unexpected text mode id: %+v", items[0])
	}
}

func TestSanitizeEpisodeBadgeDropsUnknownTotal(t *testing.T) {
	if got := sanitizeEpisodeBadge("Ep 131 / ?"); got != "Ep 131" {
		t.Fatalf("expected sanitized badge Ep 131, got %q", got)
	}
}

func TestLatestSeasonUsesLastItemInOriginalList(t *testing.T) {
	items := []domain.PropertyItem{
		{ID: "fall-2025", Title: "Fall 2025"},
		{ID: "winter-2026", Title: "Winter 2026"},
	}

	latest, ok := latestSeason(items)
	if !ok {
		t.Fatal("expected latest season to be found")
	}
	if latest.ID != "winter-2026" {
		t.Fatalf("expected winter-2026, got %+v", latest)
	}
}
