package wga

import (
	"fmt"

	"github.com/UnicomAI/wanwu/pkg/wga/internal/option"
	wga_option "github.com/UnicomAI/wanwu/pkg/wga/wga-option"
)

type Option = option.Option

// WithModelConfig 设置模型配置。
func WithModelConfig(model wga_option.ModelConfig) Option {
	return option.OptionFunc(func(opts *option.Options) error {
		opts.Model = model
		return nil
	})
}

// WithToolConfig 添加工具配置，工具标题不能重复。
func WithToolConfig(tool wga_option.ToolConfig) Option {
	return option.OptionFunc(func(opts *option.Options) error {
		if tool.APIAuth != nil {
			if err := tool.APIAuth.Check(); err != nil {
				return fmt.Errorf("tool (%v) check auth err: %v", tool.Title, err)
			}
		}
		for _, toolOpt := range opts.Tools {
			if toolOpt.Title == tool.Title {
				return fmt.Errorf("tool (%v) already exist", tool.Title)
			}
		}
		opts.Tools = append(opts.Tools, tool)
		return nil
	})
}

// WithWorkspaceConfig 设置工作空间配置。
func WithWorkspaceConfig(workspace wga_option.WorkspaceConfig) Option {
	return option.OptionFunc(func(opts *option.Options) error {
		opts.Workspace = workspace
		return nil
	})
}

// WithRunSession 设置执行会话标识（ThreadID 和 RunID）。
func WithRunSession(session wga_option.RunSession) Option {
	return option.OptionFunc(func(opts *option.Options) error {
		opts.RunSession = session
		return nil
	})
}
