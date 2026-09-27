package logx

import (
	"bytes"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// failWriter 每次 Write 都失败，用于模拟服务模式下 os.Stdout 不可写的情况
type failWriter struct{ err error }

func (f *failWriter) Write([]byte) (int, error) { return 0, f.err }

func TestSafeMultiWriterToleratesOneFailure(t *testing.T) {
	fw := &failWriter{err: errors.New("stdout 不可写")}
	var buf bytes.Buffer
	w := newSafeMultiWriter(fw, &buf)

	p := []byte("你好")
	n, err := w.Write(p)
	if err != nil {
		t.Fatalf("单个 writer 失败不应影响整体写入，实际返回错误: %v", err)
	}
	if n != len(p) {
		t.Fatalf("Write 应返回 len(p)=%d，实际 %d", len(p), n)
	}
	if buf.String() != "你好" {
		t.Fatalf("第二个 writer 应收到完整内容，实际: %q", buf.String())
	}
}

func TestSafeMultiWriterAllFail(t *testing.T) {
	w := newSafeMultiWriter(&failWriter{err: errors.New("err1")}, &failWriter{err: errors.New("err2")})
	n, err := w.Write([]byte("x"))
	if err == nil {
		t.Fatal("全部 writer 都失败时应返回错误")
	}
	if n != 0 {
		t.Fatalf("全部失败时写入字节数应为 0，得到 %d", n)
	}
}

func TestParseLevel(t *testing.T) {
	cases := map[string]slog.Level{
		"":        slog.LevelInfo,
		"info":    slog.LevelInfo,
		"DEBUG":   slog.LevelDebug,
		"warn":    slog.LevelWarn,
		"warning": slog.LevelWarn,
		"error":   slog.LevelError,
	}
	for in, want := range cases {
		got, err := ParseLevel(in)
		if err != nil || got != want {
			t.Errorf("ParseLevel(%q) = %v, %v；期望 %v", in, got, err, want)
		}
	}
	if _, err := ParseLevel("verbose"); err == nil {
		t.Error("未知级别应报错")
	}
}

func TestNewWritesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.log")
	log, closeFn, err := New("info", path)
	if err != nil {
		t.Fatal(err)
	}
	log.Info("你好")
	if err := closeFn(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "你好") {
		t.Fatalf("日志文件内容不含预期文本: %s", b)
	}
}

func TestNewStdoutOnly(t *testing.T) {
	_, closeFn, err := New("debug", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := closeFn(); err != nil {
		t.Fatal(err)
	}
}
