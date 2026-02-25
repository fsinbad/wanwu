package sandbox

// 一次性容器模式沙箱实现（未完成）。
//
// 一次性模式为每次执行创建新的 Docker 容器，执行完成后销毁容器。
//
// 优点：
//   - 完全隔离
//   - 安全性高
//
// 适用场景：
//   - 生产环境
//   - 多租户场景
//
// TODO: 实现以下功能：
//   - Prepare: 创建新容器
//   - Cleanup: 销毁容器
//   - Execute/ExecuteSync: 在容器内执行命令
//   - CopyToSandbox/CopyFromSandbox: 文件复制

import (
	"context"
)

const (
	defaultImageName            = "wga-sandbox-wanwu"
	defaultImageTag             = "latest"
	defaultOneshotWorkspaceBase = "/home/root/workspace"
)

var _ Sandbox = (*oneshotSandbox)(nil)

type oneshotSandbox struct {
	imageName     string
	imageTag      string
	workspaceBase string
	uuid          string
}

func newOneshotSandbox(uuid string) Sandbox {
	return &oneshotSandbox{
		imageName:     defaultImageName,
		imageTag:      defaultImageTag,
		workspaceBase: defaultOneshotWorkspaceBase,
		uuid:          uuid,
	}
}

func (s *oneshotSandbox) Prepare(ctx context.Context) error {
	panic("not implemented: oneshotSandbox.Prepare")
}

func (s *oneshotSandbox) Cleanup(ctx context.Context) error {
	panic("not implemented: oneshotSandbox.Cleanup")
}

func (s *oneshotSandbox) Execute(ctx context.Context, args ...string) (<-chan string, error) {
	panic("not implemented: oneshotSandbox.Execute")
}

func (s *oneshotSandbox) ExecuteSync(ctx context.Context, args ...string) (string, error) {
	panic("not implemented: oneshotSandbox.ExecuteSync")
}

func (s *oneshotSandbox) CopyToSandbox(ctx context.Context, localPath string, destPath ...string) error {
	panic("not implemented: oneshotSandbox.CopyToSandbox")
}

func (s *oneshotSandbox) CopyFromSandbox(ctx context.Context, localPath string) error {
	panic("not implemented: oneshotSandbox.CopyFromSandbox")
}
