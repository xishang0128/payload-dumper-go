//go:build (cgo && !force_pure_compression) || force_cgo_compression
// +build cgo,!force_pure_compression force_cgo_compression

package compression

func getCGOStatus() bool {
	return true
}
