package storage

import (
	"strings"
	"testing"
)

func TestNormalizeExtension(t *testing.T) {
	cases := []struct {
		name     string
		filename string
		wantExt  string
		wantArea string
		wantOK   bool
	}{
		{"video", "lecture.mp4", ".mp4", PrefixOriginals, true},
		{"audio", "podcast.mp3", ".mp3", PrefixOriginals, true},
		{"document", "syllabus.pdf", ".pdf", PrefixDocuments, true},
		{"image", "cover.png", ".png", PrefixThumbnails, true},
		{"uppercase is accepted", "LECTURE.MP4", ".mp4", PrefixOriginals, true},
		{"surrounding space is trimmed", "  notes.pdf  ", ".pdf", PrefixDocuments, true},
		{"unknown extension", "payload.exe", ".exe", "", false},
		{"no extension", "README", "", "", false},
		{"double extension keeps the last", "archive.pdf.exe", ".exe", "", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ext, area, ok := NormalizeExtension(tc.filename)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if ext != tc.wantExt {
				t.Errorf("ext = %q, want %q", ext, tc.wantExt)
			}
			if area != tc.wantArea {
				t.Errorf("area = %q, want %q", area, tc.wantArea)
			}
		})
	}
}

func TestOriginalKeyRoutesByArea(t *testing.T) {
	const stableID = "0f2b6c44-1a9e-4d1b-9a0c-6c2d2c9f1111"

	cases := []struct {
		filename   string
		wantPrefix string
	}{
		{"lecture.mp4", PrefixOriginals + "/" + stableID + "/"},
		{"syllabus.pdf", PrefixDocuments + "/" + stableID + "/"},
		{"cover.jpg", PrefixThumbnails + "/" + stableID + "/"},
	}

	for _, tc := range cases {
		t.Run(tc.filename, func(t *testing.T) {
			key, err := OriginalKey(stableID, tc.filename)
			if err != nil {
				t.Fatalf("OriginalKey: %v", err)
			}
			if !strings.HasPrefix(key, tc.wantPrefix) {
				t.Fatalf("key = %q, want prefix %q", key, tc.wantPrefix)
			}
		})
	}
}

// The client's filename must not survive into the key: it is attacker-chosen
// text that would otherwise end up in a URL and in the bucket's namespace.
func TestOriginalKeyDiscardsClientFilename(t *testing.T) {
	const stableID = "0f2b6c44-1a9e-4d1b-9a0c-6c2d2c9f1111"

	key, err := OriginalKey(stableID, "../../../etc/passwd.mp4")
	if err != nil {
		t.Fatalf("OriginalKey: %v", err)
	}
	if strings.Contains(key, "..") {
		t.Errorf("key %q still carries a traversal segment", key)
	}
	if strings.Contains(key, "passwd") {
		t.Errorf("key %q still carries the client filename", key)
	}
	if want := PrefixOriginals + "/" + stableID + "/"; !strings.HasPrefix(key, want) {
		t.Errorf("key = %q, want prefix %q", key, want)
	}
	if !strings.HasSuffix(key, ".mp4") {
		t.Errorf("key = %q, want the .mp4 extension preserved", key)
	}
}

// A re-upload must not collide with the object a running transcode is reading.
func TestOriginalKeyIsUniquePerCall(t *testing.T) {
	const stableID = "0f2b6c44-1a9e-4d1b-9a0c-6c2d2c9f1111"

	first, err := OriginalKey(stableID, "lecture.mp4")
	if err != nil {
		t.Fatalf("first OriginalKey: %v", err)
	}
	second, err := OriginalKey(stableID, "lecture.mp4")
	if err != nil {
		t.Fatalf("second OriginalKey: %v", err)
	}
	if first == second {
		t.Fatalf("two calls produced the same key %q", first)
	}
}

func TestOriginalKeyRejectsBadInput(t *testing.T) {
	t.Run("unsupported extension", func(t *testing.T) {
		if _, err := OriginalKey("stable", "payload.exe"); err == nil {
			t.Fatal("expected an error for an unsupported extension")
		}
	})
	t.Run("missing stable id", func(t *testing.T) {
		if _, err := OriginalKey("   ", "lecture.mp4"); err == nil {
			t.Fatal("expected an error for a blank stable id")
		}
	})
}

func TestDerivativeKeys(t *testing.T) {
	const stableID = "0f2b6c44-1a9e-4d1b-9a0c-6c2d2c9f1111"

	if got, want := HLSPrefix(stableID), "hls/"+stableID; got != want {
		t.Errorf("HLSPrefix = %q, want %q", got, want)
	}
	if got, want := HLSMasterKey(stableID), "hls/"+stableID+"/master.m3u8"; got != want {
		t.Errorf("HLSMasterKey = %q, want %q", got, want)
	}
	if got, want := ThumbnailKey(stableID), "thumbnails/"+stableID+"/poster.jpg"; got != want {
		t.Errorf("ThumbnailKey = %q, want %q", got, want)
	}
}

// Derivatives are written by the worker's credentials. The API must never sign
// an upload into that prefix, or a client could publish its own "transcode".
func TestIsDerivativeKey(t *testing.T) {
	const stableID = "0f2b6c44-1a9e-4d1b-9a0c-6c2d2c9f1111"

	if !IsDerivativeKey(HLSMasterKey(stableID)) {
		t.Error("the HLS master manifest should be recognised as a derivative")
	}
	if IsDerivativeKey(PrefixOriginals + "/" + stableID + "/file.mp4") {
		t.Error("an original should not be recognised as a derivative")
	}
	if IsDerivativeKey("hls-not-a-prefix/file.mp4") {
		t.Error("a key that merely starts with the letters hls is not a derivative")
	}
}
