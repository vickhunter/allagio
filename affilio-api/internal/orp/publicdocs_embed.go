package orp

import "embed"

// publicBundle is the copy of SPEC, GAPS, schemas, and examples shipped in
// the API image. The image build context is affilio-api, so the files live
// in this package. TestPublicBundleMatchesRepo fails when the copy drifts.
//
//go:embed bundled
var publicBundle embed.FS
