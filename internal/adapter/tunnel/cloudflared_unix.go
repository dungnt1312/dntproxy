// +build !windows

package tunnel

import "syscall"

// getSysProcAttr returns Unix-specific process attributes.
func getSysProcAttr() *syscall.SysProcAttr {
	return nil
}
