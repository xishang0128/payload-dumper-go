package file

import (
	_ "embed"
)

// Built-in root certificates for systems that may not have proper CA bundles
// These are common root CAs that are widely trusted
//
//go:embed ca-certificates.crt
var builtInCerts string
