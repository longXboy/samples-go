package dsl

import (
	"context"
	"fmt"
	"time"
)

// 用于 worker.RegisterActivity(a) 注册其方法
type Activities struct{}

// 模拟计算/IO 活动
func (a *Activities) DoA(ctx context.Context, x int64) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-time.After(10 * time.Millisecond):
	}
	return fmt.Sprintf("A:%d", x), nil
}

func (a *Activities) DoB(ctx context.Context, y int64) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-time.After(10 * time.Millisecond):
	}
	return fmt.Sprintf("B:%d", y), nil
}

func (a *Activities) DoC(ctx context.Context, aStr, bStr string) (string, error) {
	return fmt.Sprintf("C(%s+%s)", aStr, bStr), nil
}

// 模拟抓取（真实生产里这里做 HTTP/存储等，注意幂等）
func (a *Activities) Fetch(ctx context.Context, url string) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-time.After(5 * time.Millisecond):
	}
	return "content-of-" + url, nil
}

// 模拟审批通过（返回 true）
func (a *Activities) MockApprove(ctx context.Context) (bool, error) {
	return true, nil
}
