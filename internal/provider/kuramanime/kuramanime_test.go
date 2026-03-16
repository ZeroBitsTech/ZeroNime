package kuramanime_test

import (
	"testing"

	"anime/develop/backend/internal/provider/kuramanime"
)

func TestExtractStreamCandidatesPrefersDirectMP4Sources(t *testing.T) {
	html := `
	<div id="animeVideoPlayer" class="anime_vid_player">
	  <div class="plyr__video-wrapper hide-before init-ratio">
	    <video id="player" crossorigin="anonymous" playsinline preload="none"
	      src="https://kitasan.my.id/kdrive/jItAjpQD06hxHn/Kuramanime-DIGIMON_ALT7-22-480p.mp4?lud=1773546692&pid=46618&sid=243832&cce=1">
	      <source id="source720" src="https://kitasan.my.id/kdrive/6EeJtTN2olf7h/Kuramanime-DIGIMON_ALT7-22-720p.mp4?lud=1773546694&pid=46618&sid=243833&cce=1" type="video/mp4" size="720">
	      <source id="source480" src="https://kitasan.my.id/kdrive/jItAjpQD06hxHn/Kuramanime-DIGIMON_ALT7-22-480p.mp4?lud=1773546692&pid=46618&sid=243832&cce=1" type="video/mp4" size="480">
	      <source id="source360" src="https://kitasan.my.id/kdrive/uFHak4EqYrrFw3Y/Kuramanime-DIGIMON_ALT7-22-360p.mp4?lud=1773546691&pid=46618&sid=243831&cce=1" type="video/mp4" size="360">
	    </video>
	  </div>
	</div>`

	candidates, err := kuramanime.ExtractStreamCandidates(html)
	if err != nil {
		t.Fatalf("ExtractStreamCandidates returned error: %v", err)
	}

	if len(candidates) != 3 {
		t.Fatalf("expected 3 stream candidates, got %d", len(candidates))
	}

	if candidates[0].Container != "mp4" {
		t.Fatalf("expected mp4 container, got %s", candidates[0].Container)
	}

	if candidates[0].Quality != "720p" {
		t.Fatalf("expected first candidate quality 720p, got %s", candidates[0].Quality)
	}

	if !candidates[0].IsDirect || !candidates[0].Playable {
		t.Fatalf("expected first candidate to be direct and playable")
	}
}
