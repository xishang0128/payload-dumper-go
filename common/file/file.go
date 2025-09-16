package file

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"sync"

	"github.com/xishang0128/payload-dumper-go/common/i18n"
)

var UserAgent string = "curl/7.68.0"

// SetUserAgent sets the User-Agent string for HTTP requests
func SetUserAgent(ua string) {
	if ua != "" {
		UserAgent = ua
	}
}

var HTTPClientTimeout = 5 * time.Second

// SetHTTPClientTimeout allows adjusting HTTP client timeout programmatically
func SetHTTPClientTimeout(d time.Duration) {
	if d > 0 {
		HTTPClientTimeout = d
	}
}

var HTTPMaxConcurrentRequests int

var httpRequestSem chan struct{}

var HTTPReadCacheSize int64 = 1 << 20

// SetHTTPReadCacheSize allows adjusting read cache size programmatically
func SetHTTPReadCacheSize(n int64) {
	if n >= 0 {
		HTTPReadCacheSize = n
	}
}

// SetHTTPMaxConcurrentRequests sets the maximum number of concurrent HTTP requests
func SetHTTPMaxConcurrentRequests(max int) {
	HTTPMaxConcurrentRequests = max
	if max > 0 {
		httpRequestSem = make(chan struct{}, max)
	} else {
		httpRequestSem = nil
	}
}

// Reader interface for reading payload files
type Reader interface {
	io.ReaderAt
	io.Closer
	Size() int64
	Read(offset int64, size int) ([]byte, error)
}

func createCustomCertPool() *x509.CertPool {
	certPool, err := x509.SystemCertPool()
	if err != nil {
		certPool = x509.NewCertPool()
		certPool.AppendCertsFromPEM([]byte(BuiltInCerts))
	}

	return certPool
}

func createHTTPClientWithDNS() *http.Client {
	if _, err := os.Stat("/etc/resolv.conf"); os.IsNotExist(err) {
		dnsServers := []string{"223.5.5.5:53", "1.1.1.1:53"}
		fmt.Printf("%s, %s", i18n.I18nMsg.Common.DNSResolvConfNotFound,
			fmt.Sprintf(i18n.I18nMsg.Common.DNSUsingFallbackServers, dnsServers))

		dialer := &net.Dialer{
			Timeout: HTTPClientTimeout,
		}

		resolver := &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				var lastErr error
				for _, server := range dnsServers {
					conn, err := dialer.DialContext(ctx, network, server)
					if err == nil {
						return conn, nil
					}
					lastErr = err
				}
				return nil, fmt.Errorf(i18n.I18nMsg.Common.DNSFailedToConnectToServers, lastErr)
			},
		}

		transport := &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				host, port, err := net.SplitHostPort(addr)
				if err != nil {
					return nil, err
				}

				ips, err := resolver.LookupIPAddr(ctx, host)
				if err != nil {
					return nil, err
				}

				if len(ips) == 0 {
					return nil, fmt.Errorf(i18n.I18nMsg.Common.DNSNoIPAddressesFound, host)
				}

				for _, ip := range ips {
					addr := net.JoinHostPort(ip.IP.String(), port)
					conn, err := dialer.DialContext(ctx, network, addr)
					if err == nil {
						return conn, nil
					}
				}

				return nil, fmt.Errorf(i18n.I18nMsg.Common.DNSFailedToConnect, addr)
			},
			TLSClientConfig: &tls.Config{
				RootCAs: createCustomCertPool(),
			},
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   100,
			MaxConnsPerHost:       100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		}

		return &http.Client{
			Transport: transport,
			Timeout:   HTTPClientTimeout,
		}
	}

	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: createCustomCertPool(),
			},
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   100,
			MaxConnsPerHost:       100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
		Timeout: HTTPClientTimeout,
	}
}

// LocalFile implements Reader interface for local files
type LocalFile struct {
	file *os.File
	size int64
}

// NewLocalFile opens a local file for reading
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

	cache      []byte
	cacheStart int64
	cacheEnd   int64
	cacheMu    sync.Mutex
}

// NewHTTPFile opens an HTTP URL for reading
func NewHTTPFile(url string) (*HTTPFile, error) {
	client := createHTTPClientWithDNS()

	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", UserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.Header.Get("Accept-Ranges") != "bytes" {
		return nil, fmt.Errorf(i18n.I18nMsg.Common.HTTPRemoteDoesNotSupportRanges)
	}

	contentLength := resp.Header.Get("Content-Length")
	if contentLength == "" {
		return nil, fmt.Errorf(i18n.I18nMsg.Common.HTTPRemoteHasNoLength)
	}

	size, err := strconv.ParseInt(contentLength, 10, 64)
	if err != nil {
		return nil, fmt.Errorf(i18n.I18nMsg.Common.HTTPInvalidContentLength, err)
	}

	if size == 0 {
		return nil, fmt.Errorf(i18n.I18nMsg.Common.HTTPRemoteHasNoLength)
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

	const maxRetries = 3
	var lastErr error
	expectedSize := endPos - offset + 1
	if HTTPReadCacheSize > 0 {
		f.cacheMu.Lock()
		if f.cache != nil && offset >= f.cacheStart && endPos <= f.cacheEnd {
			start64 := offset - f.cacheStart
			reqLen64 := expectedSize
			if start64 >= 0 && reqLen64 >= 0 {
				iStart := int(start64)
				iLen := int(reqLen64)
				iEnd := iStart + iLen
				if iStart >= 0 && iEnd <= len(f.cache) {
					data := make([]byte, iLen)
					copy(data, f.cache[iStart:iEnd])
					f.cacheMu.Unlock()
					return data, nil
				}
			}
		}
		f.cacheMu.Unlock()
	}

	fetchSize := expectedSize
	if HTTPReadCacheSize > 0 && HTTPReadCacheSize > fetchSize {
		fetchSize = HTTPReadCacheSize
	}

	if httpRequestSem != nil {
		httpRequestSem <- struct{}{}
		defer func() { <-httpRequestSem }()
	}

	for attempt := 0; attempt <= maxRetries; attempt++ {
		fetchEnd := offset + fetchSize - 1
		if fetchEnd >= f.size {
			fetchEnd = f.size - 1
		}

		req, err := http.NewRequest("GET", f.url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", UserAgent)
		rangeHeader := fmt.Sprintf("bytes=%d-%d", offset, fetchEnd)
		req.Header.Set("Range", rangeHeader)

		resp, err := f.client.Do(req)
		if err != nil {
			lastErr = err
			if attempt < maxRetries {
				time.Sleep(time.Duration(1<<attempt) * 100 * time.Millisecond)
				continue
			}
			return nil, err
		}

		if resp.StatusCode >= 500 && resp.StatusCode < 600 {
			lastErr = fmt.Errorf(i18n.I18nMsg.Common.HTTPRemoteDidNotReturnPartial, resp.StatusCode)
			resp.Body.Close()
			if attempt < maxRetries {
				time.Sleep(time.Duration(1<<attempt) * 100 * time.Millisecond)
				continue
			}
			return nil, lastErr
		}

		if resp.StatusCode != 206 {
			lastErr = fmt.Errorf(i18n.I18nMsg.Common.HTTPRemoteDidNotReturnPartial, resp.StatusCode)
			resp.Body.Close()
			if attempt < maxRetries {
				time.Sleep(time.Duration(1<<attempt) * 100 * time.Millisecond)
				continue
			}
			return nil, lastErr
		}

		fetchedLen := fetchEnd - offset + 1
		fetchedData := make([]byte, fetchedLen)
		n, err := io.ReadFull(resp.Body, fetchedData)
		resp.Body.Close()
		if err != nil {
			if err == io.ErrUnexpectedEOF {
				lastErr = fmt.Errorf(i18n.I18nMsg.Common.HTTPRemoteReadUnexpectedEOF, rangeHeader, n, fetchedLen)
				if attempt < maxRetries {
					time.Sleep(time.Duration(1<<attempt) * 100 * time.Millisecond)
					continue
				}
				return nil, lastErr
			}
			lastErr = err
			if attempt < maxRetries {
				time.Sleep(time.Duration(1<<attempt) * 100 * time.Millisecond)
				continue
			}
			return nil, err
		}

		if HTTPReadCacheSize > 0 {
			if int64(len(fetchedData)) < expectedSize {
				lastErr = fmt.Errorf(i18n.I18nMsg.Common.HTTPRemoteReadUnexpectedEOF, rangeHeader, n, fetchedLen)
				if attempt < maxRetries {
					time.Sleep(time.Duration(1<<attempt) * 100 * time.Millisecond)
					continue
				}
				return nil, lastErr
			}

			f.cacheMu.Lock()
			f.cache = make([]byte, len(fetchedData))
			copy(f.cache, fetchedData)
			f.cacheStart = offset
			f.cacheEnd = offset + int64(len(fetchedData)) - 1
			f.cacheMu.Unlock()
			data := make([]byte, int(expectedSize))
			copy(data, fetchedData[0:int(expectedSize)])
			return data, nil
		}

		if int64(len(fetchedData)) >= expectedSize {
			return fetchedData[:expectedSize], nil
		}

		lastErr = fmt.Errorf(i18n.I18nMsg.Common.HTTPReadFailedAfterRetries)
		if attempt < maxRetries {
			time.Sleep(time.Duration(1<<attempt) * 100 * time.Millisecond)
			continue
		}
		return nil, lastErr
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf(i18n.I18nMsg.Common.HTTPReadFailedAfterRetries)
}
