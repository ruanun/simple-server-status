// Package cliutil 提供命令行辅助函数。
package cliutil

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/pflag"
)

// ApplyEnv 对命令行未显式设置的参数，若存在环境变量 PREFIX_NAME（名称转大写，- 替换为 _），则用其值设置
func ApplyEnv(fs *pflag.FlagSet, prefix string) error {
	var firstErr error
	fs.VisitAll(func(f *pflag.Flag) {
		if f.Changed || firstErr != nil {
			return
		}
		key := prefix + "_" + strings.ToUpper(strings.ReplaceAll(f.Name, "-", "_"))
		if v, ok := os.LookupEnv(key); ok {
			if err := fs.Set(f.Name, v); err != nil {
				firstErr = fmt.Errorf("环境变量 %s 的值无效: %w", key, err)
			}
		}
	})
	return firstErr
}
