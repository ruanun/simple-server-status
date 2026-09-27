// Package logx 初始化 slog 日志，可选同时写入按大小轮转的文件。
package logx

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"gopkg.in/natefinch/lumberjack.v2"
)

// ParseLevel 解析日志级别，空字符串视为 info
func ParseLevel(s string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	}
	return slog.LevelInfo, fmt.Errorf("未知日志级别: %q", s)
}

// New 创建日志器；file 为空时只输出到标准输出。返回的关闭函数用于关闭日志文件
func New(level, file string) (*slog.Logger, func() error, error) {
	lv, err := ParseLevel(level)
	if err != nil {
		return nil, nil, err
	}
	var w io.Writer = os.Stdout
	closeFn := func() error { return nil }
	if file != "" {
		lj := &lumberjack.Logger{Filename: file, MaxSize: 10, MaxBackups: 5, MaxAge: 30, Compress: true}
		// 以 Windows 服务方式运行时没有标准输出句柄，写 os.Stdout 会报错；
		// 若直接用 io.MultiWriter，其中一个 writer 失败会导致整体失败，
		// 日志文件也收不到任何内容，因此这里用容错的多路 writer
		w = newSafeMultiWriter(os.Stdout, lj)
		closeFn = lj.Close
	}
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: lv})), closeFn, nil
}

// safeMultiWriter 容错的多路 writer：依次写入每个 writer，单个失败不影响其余；
// 只要有一个成功就返回 len(p), nil，全部失败才返回错误
type safeMultiWriter struct {
	writers []io.Writer
}

// newSafeMultiWriter 创建容错的多路 writer
func newSafeMultiWriter(writers ...io.Writer) io.Writer {
	return &safeMultiWriter{writers: writers}
}

func (w *safeMultiWriter) Write(p []byte) (int, error) {
	var errs []error
	ok := false
	for _, sub := range w.writers {
		if _, err := sub.Write(p); err != nil {
			errs = append(errs, err)
			continue
		}
		ok = true
	}
	if !ok {
		return 0, errors.Join(errs...)
	}
	return len(p), nil
}
