//go:build windows

package collect

import (
	"strconv"

	"golang.org/x/sys/windows/registry"
)

// bootID 本次开机的唯一标识：系统每次开机加一的启动计数
func bootID() (string, error) {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE,
		`SYSTEM\CurrentControlSet\Control\Session Manager\Memory Management\PrefetchParameters`, registry.QUERY_VALUE)
	if err != nil {
		return "", err
	}
	defer k.Close()
	n, _, err := k.GetIntegerValue("BootId")
	if err != nil {
		return "", err
	}
	return strconv.FormatUint(n, 10), nil
}
