// Package mcp exposes dntproxy's vision-capable providers as an MCP
// (Model Context Protocol) server with image/video analysis tools.
package mcp

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Media size and time limits (see the design spec).
const (
	MaxImageBytes        = 20 << 20  // 20 MB
	MaxVideoBytes        = 150 << 20 // 150 MB
	imageDownloadTimeout = 30 * time.Second
	videoDownloadTimeout = 2 * time.Minute
)

// FetchImageAsDataURL downloads an image over http/https and returns it as a
// base64 data URL. data: URIs pass through unchanged.
func FetchImageAsDataURL(ctx context.Context, client *http.Client, rawURL string) (string, error) {
	if strings.HasPrefix(rawURL, "data:") {
		return rawURL, nil
	}
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return "", fmt.Errorf("image_url must be http(s) or a data: URI")
	}

	ctx, cancel := context.WithTimeout(ctx, imageDownloadTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("download image: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download image: HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, MaxImageBytes+1))
	if err != nil {
		return "", fmt.Errorf("read image: %w", err)
	}
	if len(data) > MaxImageBytes {
		return "", fmt.Errorf("image exceeds %d MB limit", MaxImageBytes/(1<<20))
	}

	mime := resp.Header.Get("Content-Type")
	if mime == "" || !strings.HasPrefix(mime, "image/") {
		mime = http.DetectContentType(data)
	}
	mime, _, _ = strings.Cut(mime, ";")
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}

// DownloadVideo downloads a video over http/https into a temp file, returning
// its path and a cleanup func that removes the file. The caller MUST call
// cleanup.
func DownloadVideo(ctx context.Context, client *http.Client, rawURL string) (string, func(), error) {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return "", nil, fmt.Errorf("video_url must be http(s)")
	}

	ctx, cancel := context.WithTimeout(ctx, videoDownloadTimeout)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		cancel()
		return "", nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		cancel()
		return "", nil, fmt.Errorf("download video: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		cancel()
		return "", nil, fmt.Errorf("download video: HTTP %d", resp.StatusCode)
	}

	ext := strings.ToLower(filepath.Ext(u.Path))
	if ext == "" || len(ext) > 6 {
		ext = ".mp4"
	}
	tmp, err := os.CreateTemp("", "dntproxy-video-*"+ext)
	if err != nil {
		cancel()
		return "", nil, err
	}

	cleanup := func() {
		cancel()
		tmp.Close()
		os.Remove(tmp.Name())
	}

	if _, err := io.Copy(tmp, io.LimitReader(resp.Body, MaxVideoBytes+1)); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("write video: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return "", nil, err
	}
	if info, err := os.Stat(tmp.Name()); err != nil || info.Size() == 0 {
		cleanup()
		return "", nil, fmt.Errorf("downloaded video is empty")
	}
	// Size check after download; over-limit downloads are rejected outright.
	if info, _ := os.Stat(tmp.Name()); info.Size() > MaxVideoBytes {
		cleanup()
		return "", nil, fmt.Errorf("video exceeds %d MB limit", MaxVideoBytes/(1<<20))
	}
	return tmp.Name(), cleanup, nil
}
