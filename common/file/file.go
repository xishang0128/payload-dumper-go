// Package file provides abstractions for reading payload files from different sources.
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

	"github.com/xishang0128/payload-dumper-go/common/i18n"
)

// Default User-Agent string for HTTP requests
var UserAgent string = "curl/7.68.0" // Mimic curl User-Agent for better compatibility

// SetUserAgent sets the User-Agent string for HTTP requests
func SetUserAgent(ua string) {
	if ua != "" {
		UserAgent = ua
	}
}

// HTTPClientTimeout controls the timeout used by the HTTP client.
// Default increased from 10s to 60s to be more tolerant of slow/large range requests.
var HTTPClientTimeout = 5 * time.Second

// SetHTTPClientTimeout allows adjusting HTTP client timeout programmatically.
func SetHTTPClientTimeout(d time.Duration) {
	if d > 0 {
		HTTPClientTimeout = d
	}
}

// HTTPMaxConcurrentRequests controls the maximum number of concurrent HTTP range
// requests issued by HTTPFile.Read. A value of 0 means unlimited.
var HTTPMaxConcurrentRequests int

// httpRequestSem is a semaphore used to limit concurrent HTTP requests when
// HTTPMaxConcurrentRequests > 0. It is lazily initialized when the max is set.
var httpRequestSem chan struct{}

// SetHTTPMaxConcurrentRequests sets the maximum number of concurrent HTTP
// requests. If max <= 0, concurrency is unlimited.
func SetHTTPMaxConcurrentRequests(max int) {
	HTTPMaxConcurrentRequests = max
	if max > 0 {
		// initialize or replace semaphore
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

// createCustomCertPool creates a certificate pool with built-in certificates
func createCustomCertPool() *x509.CertPool {
	// Try to use system cert pool first
	certPool, err := x509.SystemCertPool()
	if err != nil {
		// If system cert pool is not available, create empty pool
		certPool = x509.NewCertPool()
		certPool.AppendCertsFromPEM([]byte(BuiltInCerts))
	}

	return certPool
}

// createHTTPClientWithDNS creates an HTTP client with custom DNS configuration
// For systems without /etc/resolv.conf, it sets up a fallback DNS server
func createHTTPClientWithDNS() *http.Client {
	// Check if /etc/resolv.conf exists
	if _, err := os.Stat("/etc/resolv.conf"); os.IsNotExist(err) {
		dnsServers := []string{"223.5.5.5:53", "1.1.1.1:53"}
		fmt.Printf("%s, %s", i18n.I18nMsg.Common.DNSResolvConfNotFound,
			fmt.Sprintf(i18n.I18nMsg.Common.DNSUsingFallbackServers, dnsServers))

		dialer := &net.Dialer{
			Timeout: HTTPClientTimeout,
		}

		// Create custom resolver with fallback DNS servers
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

		// Create custom transport with custom resolver
		transport := &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				host, port, err := net.SplitHostPort(addr)
				if err != nil {
					return nil, err
				}

				// Resolve using custom resolver
				ips, err := resolver.LookupIPAddr(ctx, host)
				if err != nil {
					return nil, err
				}

				if len(ips) == 0 {
					return nil, fmt.Errorf(i18n.I18nMsg.Common.DNSNoIPAddressesFound, host)
				}

				// Try to connect to resolved IPs
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

	// System has /etc/resolv.conf, use default client with custom cert pool
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
// For systems without /etc/resolv.conf, it automatically configures fallback DNS.
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

	// Retry logic for transient network issues and partial reads
	const maxRetries = 3
	var lastErr error
	expectedSize := endPos - offset + 1
	// If a global semaphore is set, acquire a token to limit concurrent HTTP requests
	if httpRequestSem != nil {
		httpRequestSem <- struct{}{}
		defer func() { <-httpRequestSem }()
	}
	for attempt := 0; attempt <= maxRetries; attempt++ {
		req, err := http.NewRequest("GET", f.url, nil)
		if err != nil {
			return nil, err
		}

		req.Header.Set("User-Agent", UserAgent)
		rangeHeader := fmt.Sprintf("bytes=%d-%d", offset, endPos)
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

		func() {
			defer resp.Body.Close()

			if resp.StatusCode >= 500 && resp.StatusCode < 600 {
				lastErr = fmt.Errorf(i18n.I18nMsg.Common.HTTPRemoteDidNotReturnPartial, resp.StatusCode)
				return
			}

			if resp.StatusCode != 206 {
				lastErr = fmt.Errorf(i18n.I18nMsg.Common.HTTPRemoteDidNotReturnPartial, resp.StatusCode)
				return
			}

			data := make([]byte, expectedSize)
			n, err := io.ReadFull(resp.Body, data)
			if err != nil {
				if err == io.ErrUnexpectedEOF {
					// Treat unexpected EOF as retryable
					lastErr = fmt.Errorf(i18n.I18nMsg.Common.HTTPRemoteReadUnexpectedEOF, rangeHeader, n, expectedSize)
					return
				}
				lastErr = err
				return
			}

			lastErr = nil
		}()

		if lastErr == nil {
			req2, err := http.NewRequest("GET", f.url, nil)
			if err != nil {
				return nil, err
			}
			req2.Header.Set("User-Agent", UserAgent)
			req2.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", offset, endPos))
			resp2, err := f.client.Do(req2)
			if err != nil {
				lastErr = err
				if attempt < maxRetries {
					time.Sleep(time.Duration(1<<attempt) * 100 * time.Millisecond)
					continue
				}
				return nil, err
			}
			defer resp2.Body.Close()

			if resp2.StatusCode != 206 {
				lastErr = fmt.Errorf(i18n.I18nMsg.Common.HTTPRemoteDidNotReturnPartial, resp2.StatusCode)
				if attempt < maxRetries {
					time.Sleep(time.Duration(1<<attempt) * 100 * time.Millisecond)
					continue
				}
				return nil, lastErr
			}

			data := make([]byte, expectedSize)
			n, err := io.ReadFull(resp2.Body, data)
			if err != nil {
				lastErr = err
				if attempt < maxRetries {
					time.Sleep(time.Duration(1<<attempt) * 100 * time.Millisecond)
					continue
				}
				return nil, err
			}

			return data[:n], nil
		}

		if attempt < maxRetries {
			time.Sleep(time.Duration(1<<attempt) * 100 * time.Millisecond)
			continue
		}
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf(i18n.I18nMsg.Common.HTTPReadFailedAfterRetries)
}
