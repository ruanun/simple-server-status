//go:build linux

package collect

import (
	"os"
	"strings"
)

// bootID 本次开机的唯一标识：内核每次开机生成的随机 UUID
func bootID() (string, error) {
	b, err := os.ReadFile("/proc/sys/kernel/random/boot_id")
	return strings.TrimSpace(string(b)), err
}
