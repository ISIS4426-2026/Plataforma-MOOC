// Package storage implements the object storage port declared in
// internal/domain. Two adapters sit behind the same interface: Cloud Storage
// for the deployed environment and MinIO for local development, so the
// application code never learns which one it is talking to.
package storage

import (
	"fmt"
	"path"
	"strings"

	"github.com/google/uuid"
)

// The four logical areas of the bucket. Keeping originals, derivatives,
// documents and thumbnails under separate prefixes is what lets IAM grant the
// API and the workers different rights on the same bucket: the API signs and
// reads, the worker writes derivatives and nothing else.
const (
	PrefixOriginals  = "originals"
	PrefixHLS        = "hls"
	PrefixDocuments  = "documents"
	PrefixThumbnails = "thumbnails"
)

// Keys are built on the resource's stable id, not its row id. A published
// course version copies rows, but the stable id survives the copy, so the
// object a learner streams stays reachable across versions instead of being
// orphaned by the next publish.

// fileKind is what an accepted extension implies: where it is stored and the
// content type we will bind into the signature.
type fileKind struct {
	area string
	mime string
}

// extensionAllowlist maps the extensions we accept to their area and canonical
// content type. The client's filename never becomes part of the key: only its
// extension survives, and only if it is on this list. That closes path
// traversal and content-type confusion in one step, since a key is otherwise
// attacker-chosen text that ends up in a URL.
var extensionAllowlist = map[string]fileKind{
	".mp4":  {PrefixOriginals, "video/mp4"},
	".mov":  {PrefixOriginals, "video/quicktime"},
	".mkv":  {PrefixOriginals, "video/x-matroska"},
	".webm": {PrefixOriginals, "video/webm"},
	".mp3":  {PrefixOriginals, "audio/mpeg"},
	".m4a":  {PrefixOriginals, "audio/mp4"},
	".wav":  {PrefixOriginals, "audio/wav"},
	".ogg":  {PrefixOriginals, "audio/ogg"},

	".pdf":  {PrefixDocuments, "application/pdf"},
	".ppt":  {PrefixDocuments, "application/vnd.ms-powerpoint"},
	".pptx": {PrefixDocuments, "application/vnd.openxmlformats-officedocument.presentationml.presentation"},
	".doc":  {PrefixDocuments, "application/msword"},
	".docx": {PrefixDocuments, "application/vnd.openxmlformats-officedocument.wordprocessingml.document"},

	".jpg":  {PrefixThumbnails, "image/jpeg"},
	".jpeg": {PrefixThumbnails, "image/jpeg"},
	".png":  {PrefixThumbnails, "image/png"},
	".webp": {PrefixThumbnails, "image/webp"},
}

// NormalizeExtension lowercases the extension of filename and reports whether
// it is one we accept, along with the bucket area it belongs to.
func NormalizeExtension(filename string) (ext string, area string, ok bool) {
	ext = strings.ToLower(path.Ext(strings.TrimSpace(filename)))
	kind, ok := extensionAllowlist[ext]
	return ext, kind.area, ok
}

// CanonicalMIME is the content type we sign for an extension.
//
// The client's declared type has to match it, and the check is not cosmetic:
// the signature binds the content type, so a mismatch makes the transfer fail
// at the bucket. Refusing at authorization time turns a confusing upload
// failure into a clear 400, and it is half of the "validación MIME" the
// specification asks for -- the other half is the worker probing the real
// bytes, since a declared type is only ever a claim.
func CanonicalMIME(ext string) (string, bool) {
	kind, ok := extensionAllowlist[strings.ToLower(strings.TrimSpace(ext))]
	return kind.mime, ok
}

// KeyBelongsToResource reports whether objectKey is one we could have issued
// for this resource. The confirmation step needs it: the client hands back the
// key it was given, and without this check it could hand back somebody else's.
func KeyBelongsToResource(objectKey, resourceStableID string) bool {
	if strings.TrimSpace(resourceStableID) == "" {
		return false
	}
	for _, area := range []string{PrefixOriginals, PrefixDocuments, PrefixThumbnails} {
		if strings.HasPrefix(objectKey, area+"/"+resourceStableID+"/") {
			return true
		}
	}
	return false
}

// OriginalKey returns the key for the untouched upload of a resource. The
// random component means a re-upload never silently overwrites the object a
// running transcode is reading.
func OriginalKey(resourceStableID, filename string) (string, error) {
	ext, area, ok := NormalizeExtension(filename)
	if !ok {
		return "", fmt.Errorf("unsupported file extension %q", path.Ext(filename))
	}
	if strings.TrimSpace(resourceStableID) == "" {
		return "", fmt.Errorf("resource stable id is required to build an object key")
	}
	return fmt.Sprintf("%s/%s/%s%s", area, resourceStableID, uuid.NewString(), ext), nil
}

// HLSPrefix is where a resource's transcoded renditions live. The worker writes
// the whole tree under it; nothing else does.
func HLSPrefix(resourceStableID string) string {
	return fmt.Sprintf("%s/%s", PrefixHLS, resourceStableID)
}

// HLSMasterKey is the manifest a player fetches first.
func HLSMasterKey(resourceStableID string) string {
	return HLSPrefix(resourceStableID) + "/master.m3u8"
}

// ThumbnailKey is where a generated poster frame lands.
func ThumbnailKey(resourceStableID string) string {
	return fmt.Sprintf("%s/%s/poster.jpg", PrefixThumbnails, resourceStableID)
}

// IsDerivativeKey reports whether a key belongs to worker-generated output.
// The API refuses to sign uploads for these: derivatives are produced by the
// worker's credentials, never by a client.
func IsDerivativeKey(objectKey string) bool {
	return strings.HasPrefix(objectKey, PrefixHLS+"/")
}
