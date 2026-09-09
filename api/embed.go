// Package api exposes the OpenAPI contract as an embedded asset.
//
// The specification is compiled into the binary rather than read from disk, so
// a deployed instance always serves the exact contract it was built from and
// the documentation cannot drift from the running code because a volume was
// mounted wrong or forgotten.
package api

import _ "embed"

// Spec is the raw contents of openapi.yaml.
//
//go:embed openapi.yaml
var Spec []byte
