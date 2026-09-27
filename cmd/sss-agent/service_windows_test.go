//go:build windows

package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"golang.org/x/sys/windows/svc"
)

// executeAsync 在后台运行服务处理器，返回请求通道与结果通道
func executeAsync(h *serviceHandler) (chan svc.ChangeRequest, chan [2]any) {
	req := make(chan svc.ChangeRequest)
	status := make(chan svc.Status, 16)
	done := make(chan [2]any, 1)
	go func() {
		ssec, code := h.Execute(nil, req, status)
		done <- [2]any{ssec, code}
	}()
	return req, done
}

func TestServiceHandlerStopCancelsRun(t *testing.T) {
	started := make(chan struct{})
	h := &serviceHandler{run: func(ctx context.Context) error {
		close(started)
		<-ctx.Done()
		return nil
	}}
	req, done := executeAsync(h)
	<-started
	req <- svc.ChangeRequest{Cmd: svc.Stop}
	select {
	case r := <-done:
		if r[0].(bool) || r[1].(uint32) != 0 {
			t.Fatalf("停止后应以 0 退出，得到 ssec=%v code=%v", r[0], r[1])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("收到停止请求后服务未退出")
	}
}

func TestServiceHandlerRunReturnsNil(t *testing.T) {
	h := &serviceHandler{run: func(context.Context) error { return nil }}
	_, done := executeAsync(h)
	select {
	case r := <-done:
		if r[0].(bool) || r[1].(uint32) != 0 {
			t.Fatalf("正常结束应以 0 退出，得到 ssec=%v code=%v", r[0], r[1])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("run 结束后服务未退出")
	}
}

func TestServiceHandlerRunError(t *testing.T) {
	h := &serviceHandler{run: func(context.Context) error { return errors.New("boom") }}
	_, done := executeAsync(h)
	select {
	case r := <-done:
		if !r[0].(bool) || r[1].(uint32) != 1 {
			t.Fatalf("出错应以服务专用退出码 1 退出，得到 ssec=%v code=%v", r[0], r[1])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("run 出错后服务未退出")
	}
}
