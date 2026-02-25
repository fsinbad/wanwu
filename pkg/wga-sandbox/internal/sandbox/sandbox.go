// Package sandbox 提供沙箱环境接口。
package sandbox

import "context"

// Sandbox 沙箱环境接口。
type Sandbox interface {
	Prepare(ctx context.Context) error
	Cleanup(ctx context.Context) error
	Execute(ctx context.Context, args ...string) (<-chan string, error)
	ExecuteSync(ctx context.Context, args ...string) (string, error)
	CopyToSandbox(ctx context.Context, localPath string, destPath ...string) error
	CopyFromSandbox(ctx context.Context, localPath string) error
}
