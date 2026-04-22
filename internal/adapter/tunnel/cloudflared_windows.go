// +build windows

package tunnel

import "syscall"

// getSysProcAttr returns Windows-specific process attributes.
func getSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{HideWindow: true}
}
