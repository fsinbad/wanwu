// Package option 提供智能体运行选项的内部实现。
package option

import (
	"github.com/google/uuid"

	"github.com/UnicomAI/wanwu/pkg/wga/internal/config"
	wga_option "github.com/UnicomAI/wanwu/pkg/wga/wga-option"
)

// Option 选项接口。
type Option interface {
	apply(*Options) error
}

// OptionFunc 选项函数类型。
type OptionFunc func(*Options) error

func (f OptionFunc) apply(opts *Options) error {
	return f(opts)
}

// Options 智能体运行选项。
type Options struct {
	Model      wga_option.ModelConfig     // 模型配置
	Tools      []wga_option.ToolConfig    // 工具配置列表
	Workspace  wga_option.WorkspaceConfig // 工作空间配置
	RunSession wga_option.RunSession      // 执行会话标识
}

// Apply 应用选项。
// 如果 ThreadID 或 RunID 为空，自动生成 UUID。
func (options *Options) Apply(opts ...Option) error {
	for _, opt := range opts {
		if err := opt.apply(options); err != nil {
			return err
		}
	}
	if options.RunSession.ThreadID == "" {
		options.RunSession.ThreadID = uuid.New().String()
	}
	if options.RunSession.RunID == "" {
		options.RunSession.RunID = uuid.New().String()
	}
	return nil
}

// CheckCondition 检查智能体运行条件是否满足。
func (options *Options) CheckCondition(cfg *config.Agent) (*wga_option.CheckResult, error) {
	// model
	model := wga_option.CheckModel{
		Meet: true,
	}
	if err := options.checkModel(); err != nil {
		model.Meet = false
	}
	// tool
	conditions, err := options.checkToolsCondition(cfg.ToolCategories)
	if err != nil {
		return nil, err
	}
	return &wga_option.CheckResult{
		Model:          model,
		ToolCategories: conditions,
	}, nil
}
