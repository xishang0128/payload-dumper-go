package file

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/UruhaLushia/piko"
	"github.com/xishang0128/payload-dumper-go/common/i18n"
)

const httpReadCacheSize = 1 << 20

// HTTPFile implements Reader for HTTP range resources.
type HTTPFile struct {
	url        string
	downloader *piko.Client
	size       int64

	cache      []byte
	cacheStart int64
	cacheEnd   int64
	cacheMu    sync.Mutex
}

// NewHTTPFile opens an HTTP URL and probes its size through piko.
func NewHTTPFile(url string) (*HTTPFile, error) {
	httpOpts := piko.DefaultHTTPOptions()
	if httpConnections > 0 {
		httpOpts.MaxConnsPerHost = httpConnections
	}
	httpClient, err := piko.NewHTTPClient(httpOpts)
	if err != nil {
		return nil, err
	}
	opts := piko.DefaultOptions()
	opts.Offset = 0
	opts.Length = 1
	opts.UserAgent = UserAgent
	opts.HTTPClient = httpClient
	if httpConnections > 0 {
		opts.Connections = httpConnections
	}
	if httpPartSize > 0 {
		opts.PartSize = httpPartSize
	}
	downloader, err := piko.NewClient(opts)
	if err != nil {
		return nil, err
	}

	_, result, err := downloader.DownloadBytes(context.Background(), url)
	if err != nil {
		return nil, err
	}
	if result.TotalSize <= 0 {
		return nil, fmt.Errorf("%s", i18n.I18nMsg.Common.HTTPRemoteHasNoLength)
	}

	return &HTTPFile{
		url:        url,
		downloader: downloader,
		size:       result.TotalSize,
	}, nil
}

func (f *HTTPFile) ReadAt(p []byte, off int64) (int, error) {
	data, err := f.Read(off, len(p))
	if err != nil {
		return 0, err
	}
	copy(p, data)
	if len(data) < len(p) {
		return len(data), io.EOF
	}
	return len(data), nil
}

func (f *HTTPFile) Close() error {
	return nil
}

func (f *HTTPFile) Size() int64 {
	return f.size
}

func (f *HTTPFile) Read(offset int64, size int) ([]byte, error) {
	if size == 0 {
		return []byte{}, nil
	}
	if offset < 0 || size < 0 {
		return nil, fmt.Errorf("invalid byte range: offset %d size %d", offset, size)
	}
	if offset >= f.size {
		return nil, io.EOF
	}

	end := f.size
	if int64(size) < f.size-offset {
		end = offset + int64(size)
	}
	if data, ok := f.readCache(offset, end); ok {
		return data, nil
	}

	fetchEnd := end
	if httpReadCacheSize < f.size-offset {
		fetchEnd = max(end, offset+httpReadCacheSize)
	}
	fetched, _, err := f.downloader.DownloadBytesRange(
		context.Background(), f.url, offset, fetchEnd-offset,
	)
	if err != nil {
		return nil, err
	}
	expected := end - offset
	if int64(len(fetched)) < expected {
		return nil, fmt.Errorf(i18n.I18nMsg.Common.HTTPRemoteReadUnexpectedEOF,
			fmt.Sprintf("bytes=%d-%d", offset, fetchEnd-1), len(fetched), fetchEnd-offset)
	}

	f.storeCache(offset, fetched)
	return fetched[:expected], nil
}

func (f *HTTPFile) readCache(start, end int64) ([]byte, bool) {
	f.cacheMu.Lock()
	defer f.cacheMu.Unlock()

	if f.cache == nil || start < f.cacheStart || end-1 > f.cacheEnd {
		return nil, false
	}
	cacheStart := int(start - f.cacheStart)
	cacheEnd := cacheStart + int(end-start)
	data := make([]byte, cacheEnd-cacheStart)
	copy(data, f.cache[cacheStart:cacheEnd])
	return data, true
}

func (f *HTTPFile) storeCache(start int64, data []byte) {
	f.cacheMu.Lock()
	defer f.cacheMu.Unlock()

	f.cache = data
	f.cacheStart = start
	f.cacheEnd = start + int64(len(data)) - 1
}
