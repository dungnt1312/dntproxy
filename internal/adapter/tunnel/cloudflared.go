package tunnel

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
)

const cloudflaredVersion = "2026.3.0"

// Cloudflared manages the cloudflared binary lifecycle.
type Cloudflared struct {
	state       *StateManager
	cmd         *exec.Cmd
	mu          sync.Mutex
	onURL       func(url string) // callback when URL detected
	urlDetected bool
}

// NewCloudflared creates a new Cloudflared instance.
func NewCloudflared(state *StateManager, onURL func(string)) *Cloudflared {
	return &Cloudflared{
		state: state,
		onURL: onURL,
	}
}

// EnsureBinary downloads cloudflared if not already present.
func (c *Cloudflared) EnsureBinary() error {
	binPath := c.binaryPath()
	if info, err := os.Stat(binPath); err == nil && info.Size() > 1024 {
		return nil
	}

	return c.downloadBinary(binPath)
}

func (c *Cloudflared) binaryPath() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(c.state.BinDir(), "cloudflared.exe")
	}
	return filepath.Join(c.state.BinDir(), "cloudflared")
}

func (c *Cloudflared) downloadBinary(target string) error {
	url := c.downloadURL()
	if url == "" {
		return fmt.Errorf("unsupported platform: %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	fmt.Printf("[tunnel] Downloading cloudflared %s from %s\n", cloudflaredVersion, url)

	client := &http.Client{Timeout: 3 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("download cloudflared: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download cloudflared: HTTP %d", resp.StatusCode)
	}

	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}

	tmp := target + ".tmp"
	_ = os.Remove(tmp)
	defer os.Remove(tmp)

	// Handle tar.gz archives
	if strings.HasSuffix(url, ".tgz") || strings.HasSuffix(url, ".tar.gz") {
		if err := c.extractTarGz(resp.Body, tmp); err != nil {
			return err
		}
	} else {
		f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(f, io.LimitReader(resp.Body, 200<<20))
		closeErr := f.Close()
		if copyErr != nil {
			return fmt.Errorf("write binary: %w", copyErr)
		}
		if closeErr != nil {
			return closeErr
		}
	}

	info, err := os.Stat(tmp)
	if err != nil || info.Size() < 1024 {
		return fmt.Errorf("downloaded cloudflared looks incomplete")
	}
	if err := os.Rename(tmp, target); err != nil {
		return fmt.Errorf("install cloudflared: %w", err)
	}

	fmt.Printf("[tunnel] Downloaded cloudflared to %s\n", target)
	return nil
}

func (c *Cloudflared) extractTarGz(r io.Reader, target string) error {
	gr, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("gzip: %w", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar: %w", err)
		}

		name := filepath.Base(hdr.Name)
		if (runtime.GOOS == "windows" && name == "cloudflared.exe") ||
			(runtime.GOOS != "windows" && name == "cloudflared") {
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return fmt.Errorf("extract binary: %w", err)
			}
			f.Close()
			fmt.Printf("[tunnel] Extracted cloudflared to %s\n", target)
			return nil
		}
	}

	return fmt.Errorf("cloudflared binary not found in archive")
}

func (c *Cloudflared) downloadURL() string {
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	if goarch != "amd64" && goarch != "arm64" {
		return ""
	}

	base := fmt.Sprintf("https://github.com/cloudflare/cloudflared/releases/download/%s", cloudflaredVersion)

	switch goos {
	case "windows":
		// cloudflared-windows-amd64.exe
		return fmt.Sprintf("%s/cloudflared-windows-%s.exe", base, goarch)
	case "darwin":
		// cloudflared-darwin-amd64.tgz
		return fmt.Sprintf("%s/cloudflared-darwin-%s.tgz", base, goarch)
	case "linux":
		// cloudflared-linux-amd64 (direct binary)
		return fmt.Sprintf("%s/cloudflared-linux-%s", base, goarch)
	default:
		return ""
	}
}

// Spawn starts a cloudflared quick tunnel to the given local port.
// Blocks until URL is detected or context cancelled.
func (c *Cloudflared) Spawn(ctx context.Context, localPort int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	binPath := c.binaryPath()
	if _, err := os.Stat(binPath); err != nil {
		return fmt.Errorf("cloudflared binary not found: %w", err)
	}

	// Create temp config
	tmpDir, err := os.MkdirTemp("", "dntproxy-tunnel-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	configPath := filepath.Join(tmpDir, "config.yml")
	config := fmt.Sprintf("tunnel: quick-tunnel\ncredentials-file: %s/creds.json\n", tmpDir)
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		os.RemoveAll(tmpDir)
		return fmt.Errorf("write config: %w", err)
	}

	args := []string{
		"tunnel",
		"--url", fmt.Sprintf("http://localhost:%d", localPort),
		"--config", configPath,
		"--no-autoupdate",
	}

	cmd := exec.CommandContext(ctx, binPath, args...)
	cmd.SysProcAttr = sysProcAttrs()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		os.RemoveAll(tmpDir)
		return fmt.Errorf("start cloudflared: %w", err)
	}

	c.cmd = cmd
	c.urlDetected = false

	// Save PID
	c.state.SavePID(cmd.Process.Pid)

	// Cleanup temp dir on exit
	go func() {
		cmd.Wait()
		os.RemoveAll(tmpDir)
		c.state.ClearPID()
		c.mu.Lock()
		c.cmd = nil
		c.mu.Unlock()
	}()

	// Watch output for URL
	go c.scanOutput(stdout)
	go c.scanOutput(stderr)

	return nil
}

func (c *Cloudflared) scanOutput(r io.Reader) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "trycloudflare.com") && !strings.Contains(line, "api.trycloudflare.com") {
			url := c.extractURL(line)
			if url != "" && !c.urlDetected {
				c.mu.Lock()
				if !c.urlDetected {
					c.urlDetected = true
					c.mu.Unlock()
					if c.onURL != nil {
						c.onURL(url)
					}
				} else {
					c.mu.Unlock()
				}
			}
		}
	}
}

func (c *Cloudflared) extractURL(line string) string {
	// Look for https://xxx.trycloudflare.com
	parts := strings.Fields(line)
	for _, part := range parts {
		if strings.HasPrefix(part, "https://") && strings.Contains(part, "trycloudflare.com") {
			// Clean trailing chars
			part = strings.TrimRight(part, ")]}>'\" ")
			return part
		}
	}
	return ""
}

// Kill stops the cloudflared process forcefully.
func (c *Cloudflared) Kill() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.killInternal()
}

func (c *Cloudflared) killInternal() error {
	if c.cmd != nil && c.cmd.Process != nil {
		c.cmd.Process.Kill()
	}

	// Also kill via PID file as fallback
	pid := c.state.ReadPID()
	if pid > 0 {
		killProcess(pid)
		c.state.ClearPID()
	}

	c.cmd = nil
	return nil
}

// IsRunning checks if cloudflared is currently running.
func (c *Cloudflared) IsRunning() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cmd != nil && c.cmd.Process != nil {
		// Check if process has exited via ProcessState (set after Wait())
		if c.cmd.ProcessState != nil {
			return false // already exited
		}
		// Process hasn't been waited on yet — check via PID
		return isProcessRunning(c.cmd.Process.Pid)
	}

	return false
}

func isProcessRunning(pid int) bool {
	if runtime.GOOS == "windows" {
		cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/NH", "/FO", "CSV")
		out, err := cmd.Output()
		if err != nil {
			return false
		}
		line := strings.TrimSpace(string(out))
		if line == "" || strings.Contains(line, "No tasks") || strings.Contains(line, "INFO:") {
			return false
		}
		return strings.Contains(line, fmt.Sprintf("%d", pid))
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

func killProcess(pid int) {
	if runtime.GOOS == "windows" {
		exec.Command("taskkill", "/F", "/PID", fmt.Sprintf("%d", pid)).Run()
	} else {
		proc, err := os.FindProcess(pid)
		if err == nil {
			proc.Kill()
		}
	}
}

// sysProcAttrs returns platform-specific process attributes.
func sysProcAttrs() *syscall.SysProcAttr {
	return getSysProcAttr()
}
