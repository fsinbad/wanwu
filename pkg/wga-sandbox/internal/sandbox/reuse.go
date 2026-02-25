package sandbox

// 复用容器模式沙箱实现。
//
// 复用模式使用预运行的 Docker 容器（默认：wga-sandbox-wanwu），
// 每次执行在容器内创建独立的工作目录，执行完成后清理工作目录。
//
// 优点：
//   - 启动快（无需创建新容器）
//   - 资源占用少
//
// 适用场景：
//   - 开发环境
//   - 频繁执行的场景

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"sync"

	"github.com/UnicomAI/wanwu/pkg/util"
)

const (
	defaultContainerName = "wga-sandbox-wanwu"
	defaultWorkspaceBase = "/home/root/workspace"
)

var _ Sandbox = (*reuseSandbox)(nil)

type reuseSandbox struct {
	containerName string
	workspaceBase string
	uuid          string
	workDir       string
}

func newReuseSandbox(uuid string) Sandbox {
	return &reuseSandbox{
		containerName: defaultContainerName,
		workspaceBase: defaultWorkspaceBase,
		uuid:          uuid,
	}
}

func (s *reuseSandbox) Prepare(ctx context.Context) error {
	s.workDir = fmt.Sprintf("%s/%s/workspace", s.workspaceBase, s.uuid)
	cmd := exec.CommandContext(ctx, "docker", "exec", s.containerName, "mkdir", "-p", s.workDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create workspace: %w, output: %s", err, string(output))
	}
	return nil
}

func (s *reuseSandbox) Cleanup(ctx context.Context) error {
	workspacePath := fmt.Sprintf("%s/%s", s.workspaceBase, s.uuid)
	cmd := exec.CommandContext(ctx, "docker", "exec", s.containerName, "rm", "-rf", workspacePath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to cleanup workspace: %w, output: %s", err, string(output))
	}
	return nil
}

func (s *reuseSandbox) Execute(ctx context.Context, args ...string) (<-chan string, error) {
	outputCh := make(chan string, 1024)

	execArgs := append([]string{"exec", "-w", s.workDir, s.containerName}, args...)
	cmd := exec.CommandContext(ctx, "docker", execArgs...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start command: %w", err)
	}

	go func() {
		defer util.PrintPanicStack()
		defer close(outputCh)

		var wg sync.WaitGroup

		wg.Add(1)
		go func() {
			defer util.PrintPanicStack()
			defer wg.Done()
			scanner := bufio.NewScanner(stdout)
			for scanner.Scan() {
				select {
				case outputCh <- scanner.Text():
				case <-ctx.Done():
					return
				}
			}
		}()

		wg.Add(1)
		go func() {
			defer util.PrintPanicStack()
			defer wg.Done()
			scanner := bufio.NewScanner(stderr)
			for scanner.Scan() {
				select {
				case outputCh <- fmt.Sprintf("[STDERR] %s", scanner.Text()):
				case <-ctx.Done():
					return
				}
			}
		}()

		if err := cmd.Wait(); err != nil {
			select {
			case outputCh <- fmt.Sprintf("[ERROR] command execution failed: %v", err):
			case <-ctx.Done():
			}
		}

		wg.Wait()
	}()

	return outputCh, nil
}

func (s *reuseSandbox) ExecuteSync(ctx context.Context, args ...string) (string, error) {
	execArgs := append([]string{"exec", "-w", s.workDir, s.containerName}, args...)
	cmd := exec.CommandContext(ctx, "docker", execArgs...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func (s *reuseSandbox) CopyToSandbox(ctx context.Context, localPath string, destPath ...string) error {
	target := s.workDir
	if len(destPath) > 0 {
		target = s.workDir + "/" + destPath[0]
	}
	cmd := exec.CommandContext(ctx, "docker", "cp", localPath, s.containerName+":"+target)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to copy to sandbox: %w, output: %s", err, string(output))
	}
	return nil
}

func (s *reuseSandbox) CopyFromSandbox(ctx context.Context, localPath string) error {
	cmd := exec.CommandContext(ctx, "docker", "cp", s.containerName+":"+s.workDir+"/.", localPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to copy from sandbox: %w, output: %s", err, string(output))
	}
	return nil
}
