// Package file provides abstractions for reading payload files from different sources.
package file

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
)

// Reader interface for reading payload files
type Reader interface {
	io.ReaderAt
	io.Closer
	Size() int64
	Read(offset int64, size int) ([]byte, error)
}

// LocalFile implements Reader interface for local files
type LocalFile struct {
	file *os.File
	size int64
}

// NewLocalFile opens a local file for reading.
// The file must exist and be readable.
// Returns a LocalFile that implements the Reader interface.
func NewLocalFile(path string) (*LocalFile, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	stat, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}

	return &LocalFile{
		file: file,
		size: stat.Size(),
	}, nil
}

func (f *LocalFile) ReadAt(p []byte, off int64) (n int, err error) {
	return f.file.ReadAt(p, off)
}

func (f *LocalFile) Close() error {
	return f.file.Close()
}

func (f *LocalFile) Size() int64 {
	return f.size
}

func (f *LocalFile) Read(offset int64, size int) ([]byte, error) {
	data := make([]byte, size)
	n, err := f.file.ReadAt(data, offset)
	if err != nil && err != io.EOF {
		return nil, err
	}
	return data[:n], nil
}

// HTTPFile implements Reader interface for HTTP files
type HTTPFile struct {
	url    string
	client *http.Client
	size   int64
}

// NewHTTPFile opens an HTTP URL for reading.
// The server must support range requests (Accept-Ranges: bytes).
// Returns an HTTPFile that implements the Reader interface.
func NewHTTPFile(url string) (*HTTPFile, error) {
	client := &http.Client{}

	// Get file size using HEAD request
	resp, err := client.Head(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.Header.Get("Accept-Ranges") != "bytes" {
		return nil, fmt.Errorf("remote does not support ranges")
	}

	contentLength := resp.Header.Get("Content-Length")
	if contentLength == "" {
		return nil, fmt.Errorf("remote has no length")
	}

	size, err := strconv.ParseInt(contentLength, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid content length: %v", err)
	}

	if size == 0 {
		return nil, fmt.Errorf("remote has no length")
	}

	return &HTTPFile{
		url:    url,
		client: client,
		size:   size,
	}, nil
}

func (f *HTTPFile) ReadAt(p []byte, off int64) (n int, err error) {
	data, err := f.Read(off, len(p))
	if err != nil {
		return 0, err
	}
	copy(p, data)
	return len(data), nil
}

func (f *HTTPFile) Close() error {
	// HTTP client doesn't need explicit closing for our use case
	return nil
}

func (f *HTTPFile) Size() int64 {
	return f.size
}

func (f *HTTPFile) Read(offset int64, size int) ([]byte, error) {
	if size == 0 {
		return []byte{}, nil
	}

	endPos := offset + int64(size) - 1
	if endPos >= f.size {
		endPos = f.size - 1
	}

	req, err := http.NewRequest("GET", f.url, nil)
	if err != nil {
		return nil, err
	}

	rangeHeader := fmt.Sprintf("bytes=%d-%d", offset, endPos)
	req.Header.Set("Range", rangeHeader)

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 206 {
		return nil, fmt.Errorf("remote did not return partial content: %d", resp.StatusCode)
	}

	expectedSize := endPos - offset + 1
	data := make([]byte, expectedSize)
	n, err := io.ReadFull(resp.Body, data)
	if err != nil && err != io.ErrUnexpectedEOF {
		return nil, err
	}

	return data[:n], nil
}
