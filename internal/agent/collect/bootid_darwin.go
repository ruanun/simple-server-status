//go:build darwin

package collect

import "golang.org/x/sys/unix"

// bootID 本次开机的唯一标识：系统每次开机生成的 UUID
func bootID() (string, error) {
	return unix.Sysctl("kern.bootsessionuuid")
}
