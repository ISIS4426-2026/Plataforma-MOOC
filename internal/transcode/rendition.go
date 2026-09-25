// Package transcode turns an uploaded original into the HLS renditions a player
// streams, wrapping ffmpeg and ffprobe.
//
// It deliberately knows nothing about storage, queues or the database: it takes
// a file on disk and writes files to a directory. That is what lets the ladder
// and the manifest be tested without a bucket, a worker, or ffmpeg itself.
package transcode

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// ErrUnprocessable marks input the pipeline cannot handle no matter how often it
// is retried: a truncated file, a codec ffmpeg will not read, a container with
// no streams at all.
//
// The distinction matters to the caller. A storage timeout deserves a retry; a
// corrupt upload does not, and retrying it three times only delays the moment
// the author is told their file is bad.
var ErrUnprocessable = errors.New("media cannot be processed")

// Rendition is one output of the ladder.
type Rendition struct {
	// Name is both the label in the master playlist and the filename prefix.
	Name string
	// Height in pixels. Width follows from the source's aspect ratio, which is
	// why only one dimension is declared.
	Height int
	// VideoBitrateKbps is 0 for audio-only output.
	VideoBitrateKbps int
	AudioBitrateKbps int
}

// AudioOnly is what a file with no video stream produces: one audio rendition,
// no scaling, no video encoder.
var AudioOnly = Rendition{Name: "audio", Height: 0, VideoBitrateKbps: 0, AudioBitrateKbps: 128}

// DefaultLadder is the declared set of renditions, smallest first.
//
// The delivery's statement asks for the expected renditions to be declared
// rather than discovered, so this is the declaration: a 360p floor for weak
// connections and a 720p ceiling. Nothing above 720p, because the capacity
// scenario measures a fixed worker concurrency and a 1080p pass roughly triples
// the CPU per job for output most course video does not need.
var DefaultLadder = []Rendition{
	{Name: "360p", Height: 360, VideoBitrateKbps: 800, AudioBitrateKbps: 96},
	{Name: "720p", Height: 720, VideoBitrateKbps: 2500, AudioBitrateKbps: 128},
}

// SelectRenditions picks what to produce for a given source.
//
// Nothing is ever upscaled: a 480p original yields 360p and a 480p pass, never
// 720p. Upscaling costs the same CPU as real work and produces a larger file
// that carries no extra detail, and the project statement rules it out
// explicitly.
func SelectRenditions(info *MediaInfo, ladder []Rendition) ([]Rendition, error) {
	if info == nil {
		return nil, fmt.Errorf("no media information: %w", ErrUnprocessable)
	}
	if !info.HasVideo && !info.HasAudio {
		return nil, fmt.Errorf("file carries neither a video nor an audio stream: %w", ErrUnprocessable)
	}
	if !info.HasVideo {
		return []Rendition{AudioOnly}, nil
	}
	if info.Height <= 0 {
		return nil, fmt.Errorf("video stream reports a height of %d: %w", info.Height, ErrUnprocessable)
	}

	sorted := append([]Rendition(nil), ladder...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Height < sorted[j].Height })

	var out []Rendition
	for _, r := range sorted {
		if r.Height <= info.Height {
			out = append(out, r)
		}
	}

	// A source shorter than the lowest rung still deserves an output, so it is
	// encoded at its own height rather than being dropped or blown up.
	if len(out) == 0 {
		lowest := sorted[0]
		lowest.Name = fmt.Sprintf("%dp", info.Height)
		lowest.Height = info.Height
		return []Rendition{lowest}, nil
	}

	// When the source sits between two rungs -- 480p against a 360/720 ladder --
	// the top rung is capped to the source instead of upscaling to it.
	if top := out[len(out)-1]; top.Height < info.Height {
		for _, r := range sorted {
			if r.Height > info.Height {
				capped := r
				capped.Name = fmt.Sprintf("%dp", info.Height)
				capped.Height = info.Height
				out = append(out, capped)
				break
			}
		}
	}
	return out, nil
}

// PlaylistName is the per-rendition media playlist a variant points at.
func (r Rendition) PlaylistName() string { return r.Name + ".m3u8" }

// SegmentPattern is the ffmpeg template for this rendition's segments.
func (r Rendition) SegmentPattern() string { return r.Name + "_%04d.ts" }

// bandwidth is the advertised bitrate in bits per second, video plus audio.
func (r Rendition) bandwidth() int {
	return (r.VideoBitrateKbps + r.AudioBitrateKbps) * 1000
}

// MasterPlaylist builds the manifest a player fetches first.
//
// It is written here rather than by ffmpeg's -var_stream_map: one pass per
// rendition keeps a failure attributable to a single output, and the master is
// four lines of text that are far easier to assert on than ffmpeg's muxer
// options.
func MasterPlaylist(renditions []Rendition, sourceWidth, sourceHeight int) (string, error) {
	if len(renditions) == 0 {
		return "", fmt.Errorf("no renditions to write: %w", ErrUnprocessable)
	}

	var b strings.Builder
	b.WriteString("#EXTM3U\n")
	b.WriteString("#EXT-X-VERSION:3\n")

	for _, r := range renditions {
		if r.Height == 0 {
			// Audio-only: no RESOLUTION attribute, since there is nothing to resolve.
			fmt.Fprintf(&b, "#EXT-X-STREAM-INF:BANDWIDTH=%d,CODECS=\"mp4a.40.2\"\n", r.bandwidth())
			b.WriteString(r.PlaylistName() + "\n")
			continue
		}
		width := scaledWidth(sourceWidth, sourceHeight, r.Height)
		fmt.Fprintf(&b, "#EXT-X-STREAM-INF:BANDWIDTH=%d,RESOLUTION=%dx%d\n", r.bandwidth(), width, r.Height)
		b.WriteString(r.PlaylistName() + "\n")
	}
	return b.String(), nil
}

// scaledWidth keeps the source aspect ratio and rounds to an even number, which
// is what the H.264 encoder requires of both dimensions.
func scaledWidth(sourceWidth, sourceHeight, targetHeight int) int {
	if sourceWidth <= 0 || sourceHeight <= 0 {
		return 0
	}
	w := sourceWidth * targetHeight / sourceHeight
	if w%2 != 0 {
		w++
	}
	return w
}

// ParseLadder reads a ladder from configuration.
//
// The format is one rung per comma-separated entry, as
// `height:videoKbps:audioKbps` -- for example `360:800:96,720:2500:128`, which
// is DefaultLadder. An empty string yields DefaultLadder.
//
// Bitrates are required rather than inferred from the height. The delivery's
// statement asks for the expected renditions to be declared, and a capacity run
// that changes the ladder has to be able to say exactly what it changed; a
// heuristic would make two runs incomparable without anyone noticing.
func ParseLadder(spec string) ([]Rendition, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return DefaultLadder, nil
	}

	var out []Rendition
	seen := map[int]bool{}

	for _, entry := range strings.Split(spec, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		parts := strings.Split(entry, ":")
		if len(parts) != 3 {
			return nil, fmt.Errorf("ladder entry %q must be height:videoKbps:audioKbps", entry)
		}

		height, err := positiveInt(parts[0], "height", entry)
		if err != nil {
			return nil, err
		}
		videoKbps, err := positiveInt(parts[1], "video bitrate", entry)
		if err != nil {
			return nil, err
		}
		audioKbps, err := positiveInt(parts[2], "audio bitrate", entry)
		if err != nil {
			return nil, err
		}

		// H.264 requires even dimensions, and an odd height would be silently
		// rounded by the scaler -- making the produced rendition differ from the
		// declared one.
		if height%2 != 0 {
			return nil, fmt.Errorf("ladder entry %q: height must be even", entry)
		}
		if seen[height] {
			return nil, fmt.Errorf("ladder lists height %d twice", height)
		}
		seen[height] = true

		out = append(out, Rendition{
			Name:             fmt.Sprintf("%dp", height),
			Height:           height,
			VideoBitrateKbps: videoKbps,
			AudioBitrateKbps: audioKbps,
		})
	}

	if len(out) == 0 {
		return nil, fmt.Errorf("ladder %q declares no renditions", spec)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Height < out[j].Height })
	return out, nil
}

// String renders the ladder back into the configuration format, so what a run
// actually used can be logged and pasted into the next one.
func FormatLadder(ladder []Rendition) string {
	parts := make([]string, len(ladder))
	for i, r := range ladder {
		parts[i] = fmt.Sprintf("%d:%d:%d", r.Height, r.VideoBitrateKbps, r.AudioBitrateKbps)
	}
	return strings.Join(parts, ",")
}

func positiveInt(s, field, entry string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0, fmt.Errorf("ladder entry %q: %s is not a number", entry, field)
	}
	if n <= 0 {
		return 0, fmt.Errorf("ladder entry %q: %s must be positive", entry, field)
	}
	return n, nil
}
