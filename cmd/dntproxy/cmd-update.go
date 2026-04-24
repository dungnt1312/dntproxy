package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	appversion "github.com/dungnt/dntproxy/internal/version"
	"github.com/spf13/cobra"
)

const githubRepo = "dungnt1312/dntproxy"

type ghRelease struct {
	TagName string    `json:"tag_name"`
	Assets  []ghAsset `json:"assets"`
}

type ghAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

func buildUpdateCmd() *cobra.Command {
	var forceFlag bool

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update dntproxy to the latest version",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpdate(forceFlag)
		},
	}
	cmd.Flags().BoolVarP(&forceFlag, "force", "f", false, "Force update even if already on latest")
	return cmd
}

func runUpdate(force bool) error {
	fmt.Printf("Current version: %s\n", appversion.Version)
	fmt.Println("Checking for updates...")

	// 1. Fetch latest release info
	release, err := fetchLatestRelease()
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	latestVer := strings.TrimPrefix(release.TagName, "v")
	currentVer := strings.TrimPrefix(appversion.Version, "v")

	if !force && currentVer != "dev" && compareVersions(currentVer, latestVer) >= 0 {
		fmt.Printf("Already on latest version (%s)\n", currentVer)
		return nil
	}

	fmt.Printf("New version available: %s\n", latestVer)

	// 2. Find matching asset
	assetName := getAssetName()
	var downloadURL string
	for _, a := range release.Assets {
		if a.Name == assetName {
			downloadURL = a.BrowserDownloadURL
			break
		}
	}
	if downloadURL == "" {
		return fmt.Errorf("no release asset found for %s", assetName)
	}

	// 3. Download to temp
	fmt.Printf("Downloading %s...\n", assetName)
	tmpDir, err := os.MkdirTemp("", "dntproxy-update-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	archivePath := filepath.Join(tmpDir, assetName)
	if err := downloadFile(downloadURL, archivePath); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	// 4. Extract binary
	binaryName := "dntproxy"
	if runtime.GOOS == "windows" {
		binaryName = "dntproxy.exe"
	}

	extractedPath := filepath.Join(tmpDir, binaryName)
	if strings.HasSuffix(assetName, ".zip") {
		err = extractZip(archivePath, extractedPath, binaryName)
	} else {
		err = extractTarGz(archivePath, extractedPath, binaryName)
	}
	if err != nil {
		return fmt.Errorf("extract failed: %w", err)
	}

	// 5. Replace current binary
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot find current executable: %w", err)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return fmt.Errorf("cannot resolve executable path: %w", err)
	}

	// On Windows, rename old binary first (can't overwrite running exe)
	if runtime.GOOS == "windows" {
		oldPath := exe + ".old"
		os.Remove(oldPath) // clean up previous .old if exists
		if err := os.Rename(exe, oldPath); err != nil {
			return fmt.Errorf("cannot rename current binary: %w", err)
		}
	}

	// Copy new binary
	src, err := os.Open(extractedPath)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.OpenFile(exe, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return fmt.Errorf("cannot write new binary: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("cannot write new binary: %w", err)
	}

	fmt.Printf("Updated to %s\n", latestVer)
	return nil
}

// compareVersions compares two semver strings (e.g. "0.2.0" vs "0.3.0").
// Returns -1 if a < b, 0 if a == b, 1 if a > b.
func compareVersions(a, b string) int {
	aParts := strings.Split(a, ".")
	bParts := strings.Split(b, ".")
	for i := 0; i < 3; i++ {
		var av, bv int
		if i < len(aParts) {
			fmt.Sscanf(aParts[i], "%d", &av)
		}
		if i < len(bParts) {
			fmt.Sscanf(bParts[i], "%d", &bv)
		}
		if av < bv {
			return -1
		}
		if av > bv {
			return 1
		}
	}
	return 0
}

func fetchLatestRelease() (*ghRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", githubRepo)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var release ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}
	return &release, nil
}

func getAssetName() string {
	os := runtime.GOOS
	arch := runtime.GOARCH
	ext := "tar.gz"
	if os == "windows" {
		ext = "zip"
	}
	return fmt.Sprintf("dntproxy-%s-%s.%s", os, arch, ext)
}

func downloadFile(url, dest string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	return err
}

func extractTarGz(archivePath, destPath, binaryName string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if filepath.Base(hdr.Name) == binaryName {
			out, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
			if err != nil {
				return err
			}
			defer out.Close()
			_, err = io.Copy(out, tr)
			return err
		}
	}
	return fmt.Errorf("binary %s not found in archive", binaryName)
}

func extractZip(archivePath, destPath, binaryName string) error {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		if filepath.Base(f.Name) == binaryName {
			rc, err := f.Open()
			if err != nil {
				return err
			}
			defer rc.Close()

			out, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
			if err != nil {
				return err
			}
			defer out.Close()
			_, err = io.Copy(out, rc)
			return err
		}
	}
	return fmt.Errorf("binary %s not found in archive", binaryName)
}
