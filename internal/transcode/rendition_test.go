package transcode

import (
	"errors"
	"strings"
	"testing"
)

// The statement rules out upscaling, and it is the one property of the ladder
// worth pinning down: a rendition taller than the source costs the same CPU as
// real work and carries no extra detail.
func TestSelectRenditionsNeverUpscales(t *testing.T) {
	cases := []struct {
		name        string
		info        MediaInfo
		wantHeights []int
	}{
		{
			"1080p source takes the whole ladder",
			MediaInfo{HasVideo: true, Width: 1920, Height: 1080},
			[]int{360, 720},
		},
		{
			"720p source stops at its own height",
			MediaInfo{HasVideo: true, Width: 1280, Height: 720},
			[]int{360, 720},
		},
		{
			"480p source is capped to itself instead of reaching 720p",
			MediaInfo{HasVideo: true, Width: 854, Height: 480},
			[]int{360, 480},
		},
		{
			"240p source, below the whole ladder, is encoded at its own height",
			MediaInfo{HasVideo: true, Width: 426, Height: 240},
			[]int{240},
		},
		{
			"360p source matches the floor exactly",
			MediaInfo{HasVideo: true, Width: 640, Height: 360},
			[]int{360},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := SelectRenditions(&tc.info, DefaultLadder)
			if err != nil {
				t.Fatalf("SelectRenditions: %v", err)
			}
			if len(got) != len(tc.wantHeights) {
				t.Fatalf("got %d renditions %v, want %d", len(got), heights(got), len(tc.wantHeights))
			}
			for i, h := range tc.wantHeights {
				if got[i].Height != h {
					t.Errorf("rendition %d height = %d, want %d", i, got[i].Height, h)
				}
			}
			for _, r := range got {
				if r.Height > tc.info.Height {
					t.Errorf("rendition %s (%dp) upscales a %dp source", r.Name, r.Height, tc.info.Height)
				}
			}
		})
	}
}

func TestSelectRenditionsAudioOnly(t *testing.T) {
	got, err := SelectRenditions(&MediaInfo{HasAudio: true}, DefaultLadder)
	if err != nil {
		t.Fatalf("SelectRenditions: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d renditions, want 1", len(got))
	}
	if got[0].Height != 0 {
		t.Errorf("audio-only rendition has height %d, want 0", got[0].Height)
	}
	if got[0].VideoBitrateKbps != 0 {
		t.Errorf("audio-only rendition carries a video bitrate of %d", got[0].VideoBitrateKbps)
	}
}

// A file with nothing in it is a bad upload, not a transient failure, and the
// caller needs to be able to tell those apart to decide whether to retry.
func TestSelectRenditionsRejectsUnusableInput(t *testing.T) {
	cases := []struct {
		name string
		info *MediaInfo
	}{
		{"nil info", nil},
		{"no streams at all", &MediaInfo{}},
		{"video stream with no height", &MediaInfo{HasVideo: true, Width: 100, Height: 0}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := SelectRenditions(tc.info, DefaultLadder)
			if !errors.Is(err, ErrUnprocessable) {
				t.Fatalf("error = %v, want ErrUnprocessable", err)
			}
		})
	}
}

func TestMasterPlaylist(t *testing.T) {
	renditions, err := SelectRenditions(&MediaInfo{HasVideo: true, Width: 1920, Height: 1080}, DefaultLadder)
	if err != nil {
		t.Fatalf("SelectRenditions: %v", err)
	}

	manifest, err := MasterPlaylist(renditions, 1920, 1080)
	if err != nil {
		t.Fatalf("MasterPlaylist: %v", err)
	}

	if !strings.HasPrefix(manifest, "#EXTM3U\n") {
		t.Errorf("manifest does not open with #EXTM3U:\n%s", manifest)
	}
	for _, want := range []string{"360p.m3u8", "720p.m3u8", "RESOLUTION=640x360", "RESOLUTION=1280x720"} {
		if !strings.Contains(manifest, want) {
			t.Errorf("manifest is missing %q:\n%s", want, manifest)
		}
	}
	// One STREAM-INF per rendition, each followed by its playlist.
	if got := strings.Count(manifest, "#EXT-X-STREAM-INF"); got != len(renditions) {
		t.Errorf("found %d stream entries, want %d", got, len(renditions))
	}
}

// A 4:3 source must not be advertised as 16:9, or a player picks the wrong box
// for it.
func TestMasterPlaylistKeepsAspectRatio(t *testing.T) {
	manifest, err := MasterPlaylist([]Rendition{{Name: "360p", Height: 360, VideoBitrateKbps: 800, AudioBitrateKbps: 96}}, 640, 480)
	if err != nil {
		t.Fatalf("MasterPlaylist: %v", err)
	}
	if !strings.Contains(manifest, "RESOLUTION=480x360") {
		t.Errorf("expected a 4:3 resolution of 480x360:\n%s", manifest)
	}
}

func TestMasterPlaylistAudioOnlyHasNoResolution(t *testing.T) {
	manifest, err := MasterPlaylist([]Rendition{AudioOnly}, 0, 0)
	if err != nil {
		t.Fatalf("MasterPlaylist: %v", err)
	}
	if strings.Contains(manifest, "RESOLUTION") {
		t.Errorf("audio-only manifest should not advertise a resolution:\n%s", manifest)
	}
	if !strings.Contains(manifest, "audio.m3u8") {
		t.Errorf("audio-only manifest does not reference its playlist:\n%s", manifest)
	}
}

func TestMasterPlaylistRejectsEmptyLadder(t *testing.T) {
	if _, err := MasterPlaylist(nil, 1920, 1080); !errors.Is(err, ErrUnprocessable) {
		t.Fatalf("error = %v, want ErrUnprocessable", err)
	}
}

func TestRenditionFilenames(t *testing.T) {
	r := Rendition{Name: "720p", Height: 720}
	if got := r.PlaylistName(); got != "720p.m3u8" {
		t.Errorf("PlaylistName = %q", got)
	}
	if got := r.SegmentPattern(); got != "720p_%04d.ts" {
		t.Errorf("SegmentPattern = %q", got)
	}
}

func heights(rs []Rendition) []int {
	out := make([]int, len(rs))
	for i, r := range rs {
		out[i] = r.Height
	}
	return out
}

func TestParseLadder(t *testing.T) {
	t.Run("empty falls back to the default", func(t *testing.T) {
		got, err := ParseLadder("  ")
		if err != nil {
			t.Fatalf("ParseLadder: %v", err)
		}
		if FormatLadder(got) != FormatLadder(DefaultLadder) {
			t.Errorf("got %q, want the default %q", FormatLadder(got), FormatLadder(DefaultLadder))
		}
	})

	t.Run("a single rung", func(t *testing.T) {
		got, err := ParseLadder("480:1200:96")
		if err != nil {
			t.Fatalf("ParseLadder: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("got %d rungs, want 1", len(got))
		}
		if got[0].Name != "480p" || got[0].Height != 480 ||
			got[0].VideoBitrateKbps != 1200 || got[0].AudioBitrateKbps != 96 {
			t.Errorf("parsed %+v", got[0])
		}
	})

	t.Run("out-of-order rungs are sorted", func(t *testing.T) {
		got, err := ParseLadder("720:2500:128, 360:800:96")
		if err != nil {
			t.Fatalf("ParseLadder: %v", err)
		}
		if len(got) != 2 || got[0].Height != 360 || got[1].Height != 720 {
			t.Errorf("got %v, want 360 then 720", heights(got))
		}
	})

	t.Run("round-trips through FormatLadder", func(t *testing.T) {
		const spec = "360:800:96,720:2500:128"
		got, err := ParseLadder(spec)
		if err != nil {
			t.Fatalf("ParseLadder: %v", err)
		}
		if FormatLadder(got) != spec {
			t.Errorf("round trip produced %q, want %q", FormatLadder(got), spec)
		}
	})
}

// A ladder that cannot be parsed has to stop the worker at startup. Discovering
// it mid-run would invalidate a capacity measurement without anyone noticing.
func TestParseLadderRejectsBadSpecs(t *testing.T) {
	cases := map[string]string{
		"missing fields":     "360",
		"too many fields":    "360:800:96:extra",
		"height not numeric": "hd:800:96",
		"negative bitrate":   "360:-800:96",
		"zero height":        "0:800:96",
		"odd height":         "361:800:96",
		"duplicate height":   "360:800:96,360:900:96",
		"only separators":    ",,",
	}

	for name, spec := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseLadder(spec); err == nil {
				t.Fatalf("ParseLadder(%q) succeeded; expected an error", spec)
			}
		})
	}
}

// The configured ladder still must not upscale: it bounds what may be produced,
// it does not override the source.
func TestParsedLadderStillDoesNotUpscale(t *testing.T) {
	ladder, err := ParseLadder("720:2500:128,1080:5000:192")
	if err != nil {
		t.Fatalf("ParseLadder: %v", err)
	}
	got, err := SelectRenditions(&MediaInfo{HasVideo: true, Width: 640, Height: 360}, ladder)
	if err != nil {
		t.Fatalf("SelectRenditions: %v", err)
	}
	for _, r := range got {
		if r.Height > 360 {
			t.Errorf("rendition %s upscales a 360p source", r.Name)
		}
	}
}
