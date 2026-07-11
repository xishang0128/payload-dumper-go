package file

import "io"

var UserAgent = "curl/7.68.0"

var httpConnections int

var httpPartSize int64

// SetUserAgent sets the User-Agent string for HTTP requests.
func SetUserAgent(ua string) {
	if ua != "" {
		UserAgent = ua
	}
}

// SetHTTPMaxConcurrentRequests overrides piko's default concurrency.
func SetHTTPMaxConcurrentRequests(max int) {
	if max >= 0 {
		httpConnections = max
	}
}

// SetHTTPPartSize overrides piko's default initial part size.
func SetHTTPPartSize(size int64) {
	if size >= 0 {
		httpPartSize = size
	}
}

// Reader reads payload data from local or remote files.
type Reader interface {
	io.ReaderAt
	io.Closer
	Size() int64
	Read(offset int64, size int) ([]byte, error)
}
