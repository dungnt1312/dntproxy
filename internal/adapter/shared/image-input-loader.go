package shared

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	DefaultMaxImageInputBytes int64 = 10 << 20
	maxImageRedirects               = 3
)

var blockedImageCIDRs = mustImageCIDRs(
	"0.0.0.0/8",
	"100.64.0.0/10",
	"192.0.0.0/24",
	"192.0.2.0/24",
	"198.18.0.0/15",
	"198.51.100.0/24",
	"203.0.113.0/24",
	"240.0.0.0/4",
	"2001:db8::/32",
	"fec0::/10",
)

// LoadImageInput resolves an image data URI or public HTTP(S) URL. Remote
// access is protected against SSRF and bounded by time, redirects, and size.
func LoadImageInput(ctx context.Context, source string) ([]byte, string, error) {
	source = strings.TrimSpace(source)
	if strings.HasPrefix(strings.ToLower(source), "data:") {
		return loadImageDataURI(source, DefaultMaxImageInputBytes)
	}
	return loadRemoteImage(ctx, source, DefaultMaxImageInputBytes)
}

func loadImageDataURI(source string, maxBytes int64) ([]byte, string, error) {
	comma := strings.IndexByte(source, ',')
	if comma < 0 {
		return nil, "", errors.New("invalid image data URI")
	}
	meta := source[5:comma]
	parts := strings.Split(meta, ";")
	mimeType := strings.ToLower(strings.TrimSpace(parts[0]))
	if len(parts) < 2 || !strings.EqualFold(parts[len(parts)-1], "base64") {
		return nil, "", errors.New("image data URI must be base64 encoded")
	}
	if !supportedImageMIME(mimeType) {
		return nil, "", errors.New("unsupported image media type")
	}
	encoded := source[comma+1:]
	if int64(base64.StdEncoding.DecodedLen(len(encoded))) > maxBytes {
		return nil, "", errors.New("image exceeds decoded size limit")
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, "", errors.New("invalid base64 image data")
	}
	return validateImageBytes(data, mimeType, maxBytes)
}

func loadRemoteImage(ctx context.Context, source string, maxBytes int64) ([]byte, string, error) {
	parsed, err := url.Parse(source)
	if err != nil {
		return nil, "", errors.New("invalid image URL")
	}
	if err := validateRemoteImageURL(parsed); err != nil {
		return nil, "", err
	}

	transport := &http.Transport{
		Proxy:                 nil,
		ForceAttemptHTTP2:     true,
		ResponseHeaderTimeout: 10 * time.Second,
		DialContext:           dialPublicAddress,
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   15 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxImageRedirects {
				return errors.New("too many image redirects")
			}
			return validateRemoteImageURL(req.URL)
		},
	}
	defer transport.CloseIdleConnections()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, "", errors.New("create image request")
	}
	req.Header.Set("Accept", "image/png,image/jpeg,image/webp,image/gif")
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", errors.New("download image failed")
	}
	defer resp.Body.Close()
	return readRemoteImageResponse(resp, maxBytes)
}

func validateRemoteImageURL(parsed *url.URL) error {
	if parsed == nil || parsed.Hostname() == "" {
		return errors.New("invalid image URL")
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return errors.New("image URL must use HTTP or HTTPS")
	}
	if parsed.User != nil {
		return errors.New("image URL must not contain credentials")
	}
	return nil
}

func readRemoteImageResponse(resp *http.Response, maxBytes int64) ([]byte, string, error) {
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("download image returned HTTP %d", resp.StatusCode)
	}
	if resp.ContentLength > maxBytes {
		return nil, "", errors.New("image exceeds download size limit")
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, "", errors.New("read image failed")
	}
	if int64(len(data)) > maxBytes {
		return nil, "", errors.New("image exceeds download size limit")
	}
	return validateImageBytes(data, resp.Header.Get("Content-Type"), maxBytes)
}

func dialPublicAddress(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, errors.New("invalid image host")
	}
	if strings.EqualFold(strings.TrimSuffix(host, "."), "localhost") {
		return nil, errors.New("image host is not public")
	}
	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
	if err != nil || len(ips) == 0 {
		return nil, errors.New("resolve image host failed")
	}
	dialer := net.Dialer{Timeout: 10 * time.Second}
	for _, ip := range ips {
		if !isPublicImageIP(ip) {
			return nil, errors.New("image host resolves to a non-public address")
		}
	}
	var lastErr error
	for _, ip := range ips {
		conn, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if dialErr == nil {
			return conn, nil
		}
		lastErr = dialErr
	}
	if lastErr != nil {
		return nil, errors.New("connect to image host failed")
	}
	return nil, errors.New("image host has no reachable address")
}

func isPublicImageIP(ip net.IP) bool {
	if ip == nil || !ip.IsGlobalUnicast() || ip.IsUnspecified() || ip.IsLoopback() || ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() {
		return false
	}
	for _, network := range blockedImageCIDRs {
		if network.Contains(ip) {
			return false
		}
	}
	return true
}

func mustImageCIDRs(values ...string) []*net.IPNet {
	result := make([]*net.IPNet, 0, len(values))
	for _, value := range values {
		_, network, err := net.ParseCIDR(value)
		if err != nil {
			panic("invalid blocked image CIDR: " + value)
		}
		result = append(result, network)
	}
	return result
}

func validateImageBytes(data []byte, declaredMIME string, maxBytes int64) ([]byte, string, error) {
	if len(data) == 0 {
		return nil, "", errors.New("image is empty")
	}
	if int64(len(data)) > maxBytes {
		return nil, "", errors.New("image exceeds decoded size limit")
	}
	detected := strings.ToLower(strings.SplitN(http.DetectContentType(data), ";", 2)[0])
	if !supportedImageMIME(detected) {
		return nil, "", errors.New("downloaded content is not a supported image")
	}
	declared := strings.ToLower(strings.SplitN(declaredMIME, ";", 2)[0])
	if declared != "" && declared != "application/octet-stream" && declared != detected {
		return nil, "", errors.New("image media type does not match its content")
	}
	return data, detected, nil
}

func supportedImageMIME(mimeType string) bool {
	switch strings.ToLower(mimeType) {
	case "image/png", "image/jpeg", "image/webp", "image/gif":
		return true
	default:
		return false
	}
}
