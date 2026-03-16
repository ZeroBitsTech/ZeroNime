package mediaproxy

import "testing"

func TestAllowedAcceptsCloudflareR2Hosts(t *testing.T) {
	t.Parallel()

	proxy := New()
	rawURL := "https://9b82e27cb1c6534b3f978e449d303889.r2.cloudflarestorage.com/kuramadrive/ANIME/WINTER/2026/JJKS_S3/10/MP4/Kuramanime-JJKS_S3-10-720p-BGlobal.mp4"
	if !proxy.Allowed(rawURL) {
		t.Fatalf("Allowed(%q) = false, want true", rawURL)
	}
}

