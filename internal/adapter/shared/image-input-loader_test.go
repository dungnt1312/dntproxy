package shared

import (
	"context"
	"encoding/base64"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

var onePixelPNG = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
	0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
	0x89, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x44, 0x41,
	0x54, 0x08, 0xd7, 0x63, 0xf8, 0xcf, 0xc0, 0xf0,
	0x1f, 0x00, 0x05, 0x00, 0x01, 0xff, 0x89, 0x99,
	0x3d, 0x1d, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45,
	0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
}

func TestValidateRemoteImageURLRejectsUnsafeRedirectTargets(t *testing.T) {
	for _, raw := range []string{
		"file:///tmp/image.png",
		"https://user:password@example.test/image.png",
		"https:///missing-host.png",
	} {
		parsed, _ := url.Parse(raw)
		if err := validateRemoteImageURL(parsed); err == nil {
			t.Fatalf("%s should be rejected", raw)
		}
	}
}

func TestReadRemoteImageResponseEnforcesSizeAndMIME(t *testing.T) {
	tooLarge := &http.Response{
		StatusCode:    http.StatusOK,
		ContentLength: int64(len(onePixelPNG)),
		Header:        http.Header{"Content-Type": []string{"image/png"}},
		Body:          io.NopCloser(strings.NewReader(string(onePixelPNG))),
	}
	if _, _, err := readRemoteImageResponse(tooLarge, int64(len(onePixelPNG)-1)); err == nil || !strings.Contains(err.Error(), "size limit") {
		t.Fatalf("size error = %v", err)
	}
	mismatch := &http.Response{
		StatusCode:    http.StatusOK,
		ContentLength: int64(len(onePixelPNG)),
		Header:        http.Header{"Content-Type": []string{"image/jpeg"}},
		Body:          io.NopCloser(strings.NewReader(string(onePixelPNG))),
	}
	if _, _, err := readRemoteImageResponse(mismatch, DefaultMaxImageInputBytes); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("MIME error = %v", err)
	}
}

func TestLoadImageInputDataURI(t *testing.T) {
	source := "data:image/png;base64," + base64.StdEncoding.EncodeToString(onePixelPNG)
	data, mimeType, err := LoadImageInput(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	if mimeType != "image/png" || len(data) != len(onePixelPNG) {
		t.Fatalf("mime=%q bytes=%d", mimeType, len(data))
	}
}

func TestLoadImageInputRejectsNonImageData(t *testing.T) {
	source := "data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte("not an image"))
	_, _, err := LoadImageInput(context.Background(), source)
	if err == nil || !strings.Contains(err.Error(), "not a supported image") {
		t.Fatalf("err=%v", err)
	}
}

func TestLoadRemoteImageHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, err := loadRemoteImage(ctx, "http://1.1.1.1:81/image.png", DefaultMaxImageInputBytes)
	if err == nil || !strings.Contains(err.Error(), "download image failed") {
		t.Fatalf("err=%v", err)
	}
}

func TestLoadRemoteImageHonorsContextDeadline(t *testing.T) {
	const timeout = time.Nanosecond
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	<-ctx.Done()

	started := time.Now()
	_, _, err := loadRemoteImage(ctx, "http://1.1.1.1:81/image.png", DefaultMaxImageInputBytes)
	elapsed := time.Since(started)
	if err == nil || !strings.Contains(err.Error(), "download image failed") {
		t.Fatalf("err=%v", err)
	}
	if elapsed > time.Second {
		t.Fatalf("remote image load ignored context deadline: elapsed=%v timeout=%v", elapsed, timeout)
	}
	if ctx.Err() != context.DeadlineExceeded {
		t.Fatalf("context error = %v, want deadline exceeded", ctx.Err())
	}
}

func TestPublicImageIP(t *testing.T) {
	for _, raw := range []string{
		"0.1.2.3", "10.0.0.1", "100.64.0.1", "127.0.0.1",
		"169.254.169.254", "192.0.2.1", "198.18.0.1", "198.51.100.1",
		"203.0.113.1", "240.0.0.1", "::1", "fc00::1", "2001:db8::1", "fec0::1",
	} {
		if isPublicImageIP(net.ParseIP(raw)) {
			t.Fatalf("%s should be blocked", raw)
		}
	}
	if !isPublicImageIP(net.ParseIP("1.1.1.1")) {
		t.Fatal("public address should be allowed")
	}
}
