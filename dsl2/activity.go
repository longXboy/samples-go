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

// 验证输入参数
func (a *Activities) ValidateInput(ctx context.Context) (bool, error) {
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	case <-time.After(5 * time.Millisecond):
	}
	return true, nil
}

// 检查权限
func (a *Activities) CheckPermissions(ctx context.Context) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-time.After(10 * time.Millisecond):
	}
	return "permissions-granted", nil
}

// 加载配置
func (a *Activities) LoadConfig(ctx context.Context) (map[string]interface{}, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(8 * time.Millisecond):
	}
	return map[string]interface{}{
		"database_url": "localhost:5432",
		"api_key":      "demo-key-123",
		"timeout":      30,
	}, nil
}

// 开发模式设置
func (a *Activities) DevModeSetup(ctx context.Context) (map[string]interface{}, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(5 * time.Millisecond):
	}
	return map[string]interface{}{
		"debug":       true,
		"mock_data":   true,
		"log_level":   "debug",
		"environment": "development",
	}, nil
}

// 处理单个项目
func (a *Activities) ProcessItem(ctx context.Context, item interface{}) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-time.After(15 * time.Millisecond):
	}
	return fmt.Sprintf("processed-%v", item), nil
}

// 最终化结果
func (a *Activities) FinalizeResults(ctx context.Context, results []interface{}) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-time.After(12 * time.Millisecond):
	}
	return fmt.Sprintf("finalized-%d-results", len(results)), nil
}
