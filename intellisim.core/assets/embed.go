// Package assets embeds static binary assets (PDF fonts) used by the
// simulations analysis report renderer. Embedding keeps the app a single
// self-contained binary with no filesystem font lookups.
package assets

import _ "embed"

// DejaVuSans is a Unicode TTF (Latin + Cyrillic) used as the analysis PDF body font.
//
//go:embed fonts/DejaVuSans.ttf
var DejaVuSans []byte

// DejaVuSansBold is the bold variant used for PDF headings.
//
//go:embed fonts/DejaVuSans-Bold.ttf
var DejaVuSansBold []byte
