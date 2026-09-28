package browser

import (
	"log"
	"os/exec"
	"runtime"
	"time"
)

// automationMarker is a Chromium flag unique to every browser this package
// launches. Sweeps must only kill processes carrying it — never a user's
// normal Chrome.
const automationMarker = "--disable-blink-features=AutomationControlled"

// SweepOrphans force-kills automation Chromium processes left behind by an
// earlier run (possible once leakless is disabled and the proxy is hard-killed
// mid-job). Best-effort and platform-specific; failures are logged only.
func SweepOrphans() {
	started := time.Now()
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		// PowerShell filter on the command line marker (wmic is deprecated).
		ps := `$procs = Get-CimInstance Win32_Process | Where-Object { ($_.Name -like "chrome*" -or $_.Name -like "chromium*") -and $_.CommandLine -like "` + automationMarker + `*" }; foreach ($p in $procs) { try { Stop-Process -Id $p.ProcessId -Force -ErrorAction Stop } catch {} }; Write-Host "KILLED:$($procs.Count)"`
		cmd = exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", ps)
	} else {
		cmd = exec.Command("pkill", "-f", automationMarker)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("[auto-login] orphan sweep: %v (%s)", err, string(out))
		return
	}
	log.Printf("[auto-login] orphan sweep done in %s: %s", time.Since(started).Round(time.Millisecond), string(out))
}
