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
			Timeout: 10 * time.Second,
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
		}

		return &http.Client{
			Transport: transport,
			Timeout:   10 * time.Second,
		}
	}

	// System has /etc/resolv.conf, use default client with custom cert pool
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: createCustomCertPool(),
			},
		},
		Timeout: 10 * time.Second,
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

	req, err := http.NewRequest("GET", f.url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", UserAgent)
	rangeHeader := fmt.Sprintf("bytes=%d-%d", offset, endPos)
	req.Header.Set("Range", rangeHeader)

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 206 {
		return nil, fmt.Errorf(i18n.I18nMsg.Common.HTTPRemoteDidNotReturnPartial, resp.StatusCode)
	}

	expectedSize := endPos - offset + 1
	data := make([]byte, expectedSize)
	n, err := io.ReadFull(resp.Body, data)
	if err != nil && err != io.ErrUnexpectedEOF {
		return nil, err
	}

	return data[:n], nil
}
