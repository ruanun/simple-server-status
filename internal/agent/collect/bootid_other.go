//go:build !linux && !windows && !darwin

package collect

// bootID 没有可靠的开机标识，返回空串（Dashboard 不判断重启）
func bootID() (string, error) { return "", nil }
