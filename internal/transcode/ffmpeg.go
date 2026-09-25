package transcode

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// MediaInfo is what ffprobe reports about a source file.
type MediaInfo struct {
	DurationSeconds float64
	Width           int
	Height          int
	HasVideo        bool
	HasAudio        bool
	VideoCodec      string
	AudioCodec      string
}

// Runner executes ffmpeg and ffprobe.
//
// Both are invoked as subprocesses rather than through a binding: the CLI is
// the interface ffmpeg actually supports, and shelling out keeps the worker
// image free of cgo while making every invocation reproducible from a shell.
type Runner struct {
	FFmpegPath  string
	FFprobePath string

	// Timeout bounds a single invocation. Without it a pathological input can
	// pin a worker slot indefinitely, which during a load test looks exactly
	// like saturation and is not.
	Timeout time.Duration

	// SegmentSeconds is the target HLS segment length.
	SegmentSeconds int
}

// NewRunner returns a Runner with the defaults the worker uses.
func NewRunner() *Runner {
	return &Runner{
		FFmpegPath:     "ffmpeg",
		FFprobePath:    "ffprobe",
		Timeout:        30 * time.Minute,
		SegmentSeconds: 6,
	}
}

// Available reports whether both binaries can be found, so a caller can skip
// work -- or a test can skip itself -- instead of failing obscurely later.
func (r *Runner) Available() error {
	for _, bin := range []string{r.ffmpeg(), r.ffprobe()} {
		if _, err := exec.LookPath(bin); err != nil {
			return fmt.Errorf("%s not found in PATH: %w", bin, err)
		}
	}
	return nil
}

func (r *Runner) ffmpeg() string {
	if r.FFmpegPath == "" {
		return "ffmpeg"
	}
	return r.FFmpegPath
}

func (r *Runner) ffprobe() string {
	if r.FFprobePath == "" {
		return "ffprobe"
	}
	return r.FFprobePath
}

func (r *Runner) timeout() time.Duration {
	if r.Timeout <= 0 {
		return 30 * time.Minute
	}
	return r.Timeout
}

func (r *Runner) segmentSeconds() int {
	if r.SegmentSeconds <= 0 {
		return 6
	}
	return r.SegmentSeconds
}

// ffprobeOutput mirrors the subset of `ffprobe -print_format json` we read.
type ffprobeOutput struct {
	Streams []struct {
		CodecType string `json:"codec_type"`
		CodecName string `json:"codec_name"`
		Width     int    `json:"width"`
		Height    int    `json:"height"`
	} `json:"streams"`
	Format struct {
		Duration string `json:"duration"`
	} `json:"format"`
}

// Probe inspects a file and reports its streams.
//
// A file ffprobe cannot read is unprocessable, not a transient failure: the
// bytes will be just as unreadable on the next attempt.
func (r *Runner) Probe(ctx context.Context, path string) (*MediaInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout())
	defer cancel()

	cmd := exec.CommandContext(ctx, r.ffprobe(),
		"-v", "error",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		path,
	)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, fmt.Errorf("ffprobe timed out after %s: %w", r.timeout(), ErrUnprocessable)
		}
		return nil, fmt.Errorf("ffprobe rejected the file (%v): %s: %w",
			err, lastLine(stderr.String()), ErrUnprocessable)
	}

	var out ffprobeOutput
	if err := json.Unmarshal([]byte(stdout.String()), &out); err != nil {
		return nil, fmt.Errorf("could not read ffprobe output: %v: %w", err, ErrUnprocessable)
	}

	info := &MediaInfo{}
	for _, s := range out.Streams {
		switch s.CodecType {
		case "video":
			// Cover art in an audio file shows up as a video stream with no
			// usable dimensions; treating it as video would try to transcode a
			// single JPEG into HLS.
			if s.Width > 0 && s.Height > 0 {
				info.HasVideo = true
				info.Width, info.Height = s.Width, s.Height
				info.VideoCodec = s.CodecName
			}
		case "audio":
			info.HasAudio = true
			info.AudioCodec = s.CodecName
		}
	}
	if d, err := strconv.ParseFloat(out.Format.Duration, 64); err == nil {
		info.DurationSeconds = d
	}

	if !info.HasVideo && !info.HasAudio {
		return nil, fmt.Errorf("file carries no video or audio stream: %w", ErrUnprocessable)
	}
	return info, nil
}

// ToHLS encodes one rendition into outDir, writing its media playlist and
// segments. It does not write the master playlist; MasterPlaylist does.
func (r *Runner) ToHLS(ctx context.Context, inputPath, outDir string, rendition Rendition) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, r.timeout())
	defer cancel()

	args := []string{
		"-nostdin",
		"-y",
		"-i", inputPath,
	}

	if rendition.Height > 0 {
		args = append(args,
			// -2 keeps the aspect ratio and forces an even width, which H.264
			// requires; hardcoding a width would letterbox anything that is not
			// 16:9.
			"-vf", fmt.Sprintf("scale=-2:%d", rendition.Height),
			"-c:v", "libx264",
			"-profile:v", "main",
			"-preset", "veryfast",
			"-b:v", fmt.Sprintf("%dk", rendition.VideoBitrateKbps),
			"-maxrate", fmt.Sprintf("%dk", rendition.VideoBitrateKbps),
			"-bufsize", fmt.Sprintf("%dk", rendition.VideoBitrateKbps*2),
			// Keyframes aligned to the segment length, so a player can switch
			// rendition at any segment boundary instead of mid-GOP.
			"-g", strconv.Itoa(r.segmentSeconds()*30),
			"-keyint_min", strconv.Itoa(r.segmentSeconds()*30),
			"-sc_threshold", "0",
		)
	} else {
		args = append(args, "-vn")
	}

	args = append(args,
		"-c:a", "aac",
		"-b:a", fmt.Sprintf("%dk", rendition.AudioBitrateKbps),
		"-ac", "2",
		"-f", "hls",
		"-hls_time", strconv.Itoa(r.segmentSeconds()),
		"-hls_playlist_type", "vod",
		"-hls_flags", "independent_segments",
		"-hls_segment_filename", filepath.Join(outDir, rendition.SegmentPattern()),
		filepath.Join(outDir, rendition.PlaylistName()),
	)

	cmd := exec.CommandContext(ctx, r.ffmpeg(), args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("ffmpeg timed out after %s encoding %s: %w",
				r.timeout(), rendition.Name, ErrUnprocessable)
		}
		return fmt.Errorf("ffmpeg failed encoding %s (%v): %s: %w",
			rendition.Name, err, lastLine(stderr.String()), ErrUnprocessable)
	}
	return nil
}

// lastLine returns the final non-empty line of ffmpeg's stderr, which is where
// it puts the actual reason. Carrying the whole stream into an error message
// would bury it in progress output.
func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if line := strings.TrimSpace(lines[i]); line != "" {
			return line
		}
	}
	return "no output"
}
