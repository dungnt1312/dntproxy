package logger

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/dungnt/dntproxy/internal/adapter/shared"
)

// devMode controls whether to use verbose multi-line format.
// Set via DNTPROXY_DEV=1 environment variable.
var (
	devMode     bool
	devModeOnce sync.Once
)

// IsDevMode returns true when the proxy is running in development mode.
func IsDevMode() bool {
	devModeOnce.Do(func() {
		v := os.Getenv("DNTPROXY_DEV")
		devMode = v == "1" || v == "true"
	})
	return devMode
}

// ANSI color codes for terminal output.
const (
	colorReset   = "\033[0m"
	colorGray    = "\033[90m"
	colorRed     = "\033[91m"
	colorGreen   = "\033[92m"
	colorYellow  = "\033[93m"
	colorBlue    = "\033[94m"
	colorMagenta = "\033[95m"
	colorCyan    = "\033[96m"
	colorBold    = "\033[1m"
	colorDim     = "\033[2m"
)

// terminalPrint outputs the request log to terminal.
// In dev mode: detailed multi-line block.
// In prod mode: single-line summary.
func terminalPrint(r *RequestLog) {
	if IsDevMode() {
		printDevBlock(r)
	} else {
		printProdLine(r)
	}
}

// printProdLine outputs a compact single-line summary.
// Format: 15:30:42 REQ abc12 | sonnet-4 → kiro/sonnet-4 via MyAccount | 200 | 3.2s | in=1234 out=567 | $0.012
func printProdLine(r *RequestLog) {
	ts := r.StartTime.Format("15:04:05")
	shortID := r.ID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}

	var parts []string

	// Model info
	if r.RequestModel != "" {
		modelPart := r.RequestModel
		if r.ResolvedProvider != "" && r.ResolvedModel != "" {
			resolved := r.ResolvedProvider + "/" + r.ResolvedModel
			if r.RequestModel != resolved {
				modelPart = r.RequestModel + " → " + resolved
			}
		}
		if r.ConnectionName != "" {
			modelPart += " via " + r.ConnectionName
		}
		parts = append(parts, modelPart)
	}

	// Status
	statusStr := fmt.Sprintf("%d", r.StatusCode)
	parts = append(parts, statusStr)

	// Duration
	totalDuration := time.Since(r.StartTime)
	parts = append(parts, formatDuration(totalDuration))

	// Usage
	if r.TotalTokens > 0 {
		parts = append(parts, fmt.Sprintf("in=%d out=%d", r.InputTokens, r.OutputTokens))
	}

	// Cost
	if r.CostTotal > 0 {
		parts = append(parts, fmt.Sprintf("$%.4f", r.CostTotal))
	}

	// Error
	if r.Error != "" {
		errShort := r.Error
		if len(errShort) > 80 {
			errShort = errShort[:80] + "..."
		}
		parts = append(parts, "err="+errShort)
	}

	statusColor := colorGreen
	if r.StatusCode >= 400 {
		statusColor = colorRed
	}

	fmt.Printf("%s %sREQ%s %s | %s\n",
		colorGray+ts+colorReset,
		statusColor, colorReset,
		shortID,
		strings.Join(parts, " | "),
	)
}

// printDevBlock outputs a detailed multi-line block for development.
func printDevBlock(r *RequestLog) {
	totalDuration := time.Since(r.StartTime)
	shortID := r.ID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}

	statusEmoji := "✓"
	statusColor := colorGreen
	if r.StatusCode >= 400 || r.Error != "" {
		statusEmoji = "✗"
		statusColor = colorRed
	}

	hr := colorDim + strings.Repeat("─", 60) + colorReset

	fmt.Println(hr)
	fmt.Printf("%s %s[REQ]%s %s%s%s\n",
		colorGray+r.StartTime.Format("15:04:05")+colorReset,
		colorCyan, colorReset,
		colorBold, shortID, colorReset,
	)

	// Inbound
	fmt.Printf("  %s←%s %s %s  model=%s  body=%s\n",
		colorBlue, colorReset,
		r.ClientMethod, r.ClientPath,
		colorYellow+r.RequestModel+colorReset,
		formatBytes(r.BodySize),
	)

	// Routing
	if r.ResolvedProvider != "" {
		resolved := r.ResolvedProvider + "/" + r.ResolvedModel
		if r.IsCombo {
			fmt.Printf("  %s→%s Combo: %s → %s\n",
				colorMagenta, colorReset,
				colorYellow+r.ComboName+colorReset,
				colorCyan+resolved+colorReset,
			)
		} else if r.RequestModel != resolved {
			fmt.Printf("  %s→%s Resolved: %s\n",
				colorMagenta, colorReset,
				colorCyan+resolved+colorReset,
			)
		}
	}

	// Account
	if r.ConnectionName != "" {
		attempt := ""
		if r.AttemptCount > 1 {
			attempt = fmt.Sprintf(" (attempt #%d)", r.AttemptCount)
		}
		fmt.Printf("  %s→%s Account: %s%s%s\n",
			colorMagenta, colorReset,
			colorYellow+`"`+r.ConnectionName+`"`+colorReset,
			attempt,
			func() string {
				if r.ConnectionID != "" {
					return colorGray + " (id=" + r.ConnectionID[:8] + ")" + colorReset
				}
				return ""
			}(),
		)
	}

	// Upstream
	if r.UpstreamURL != "" {
		method := r.UpstreamMethod
		if method == "" {
			method = "POST"
		}
		fmt.Printf("  %s→%s Upstream: %s %s\n",
			colorMagenta, colorReset,
			method, colorDim+r.UpstreamURL+colorReset,
		)
	}

	// Response
	if r.UpstreamStatus > 0 {
		upstreamColor := colorGreen
		if r.UpstreamStatus >= 400 {
			upstreamColor = colorRed
		}
		streamFlag := ""
		if r.Streaming {
			streamFlag = "  stream=true"
		}
		fmt.Printf("  %s←%s %s%d%s  ttfb=%s%s\n",
			colorBlue, colorReset,
			upstreamColor, r.UpstreamStatus, colorReset,
			r.UpstreamDuration.Round(time.Millisecond),
			streamFlag,
		)
	}

	// Error
	if r.Error != "" {
		errDisplay := r.Error
		if len(errDisplay) > 200 {
			errDisplay = errDisplay[:200] + "..."
		}
		fmt.Printf("  %s✗ Error: %s%s\n", colorRed, errDisplay, colorReset)
	}

	// Usage
	if r.TotalTokens > 0 {
		fmt.Printf("  %s←%s Usage: input=%s%d%s  output=%s%d%s  total=%s%d%s  source=%s\n",
			colorBlue, colorReset,
			colorCyan, r.InputTokens, colorReset,
			colorCyan, r.OutputTokens, colorReset,
			colorBold, r.TotalTokens, colorReset,
			r.UsageSource,
		)
	}

	// Cost
	if r.CostTotal > 0 {
		fmt.Printf("  %s←%s Cost: %s$%.4f%s (input=$%.4f + output=$%.4f)\n",
			colorBlue, colorReset,
			colorGreen, r.CostTotal, colorReset,
			r.CostInput, r.CostOutput,
		)
	}

	// Request body preview in dev mode
	if r.RequestBody != "" && shared.ShouldLogRawBodies() {
		preview := r.RequestBody
		if len(preview) > 200 {
			preview = preview[:200] + "..."
		}
		fmt.Printf("  %s→%s ReqBody: %s%s%s\n",
			colorMagenta, colorReset,
			colorDim, preview, colorReset,
		)
	}

	// Done
	fmt.Printf("  %s%s Done%s  total=%s\n",
		statusColor, statusEmoji, colorReset,
		formatDuration(totalDuration),
	)
	fmt.Println(hr)
}

// formatDuration formats a duration for log output.
func formatDuration(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%dµs", d.Microseconds())
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}

// formatBytes formats a byte count for log output.
func formatBytes(n int) string {
	if n < 1024 {
		return fmt.Sprintf("%dB", n)
	}
	return fmt.Sprintf("%.1fKB", float64(n)/1024)
}
