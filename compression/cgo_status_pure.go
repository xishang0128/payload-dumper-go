//go:build (!cgo) || force_pure_compression
// +build !cgo force_pure_compression

package compression

func getCGOStatus() bool {
	return false
}
