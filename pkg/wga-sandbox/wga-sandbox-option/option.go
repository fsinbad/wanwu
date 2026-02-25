// Package wga_sandbox_option 提供 wga_sandbox 的选项配置。
package wga_sandbox_option

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	openapi3_util "github.com/UnicomAI/wanwu/pkg/openapi3-util"
	"github.com/getkin/kin-openapi/openapi3"
)

// ModelConfig 模型配置。
type ModelConfig struct {
	Provider     string // 提供商标识
	ProviderName string // 提供商显示名称
	BaseURL      string // API 基础地址
	APIKey       string // API 密钥
	Model        string // 模型标识
	ModelName    string // 模型显示名称
}

// Message 消息结构。
type Message struct {
	Role    string // 角色：user, assistant, system
	Content string // 消息内容
}

// Tool 工具配置。
type Tool struct {
	OpenAPI3Schema *openapi3.T // OpenAPI 3.0 schema 文档
	OperationIDs   []string    // 允许的 operations，为空则全部允许
	APIAuth        *openapi3_util.Auth
	Name           string // 工具名称，从 schema 的 info.title 自动读取
}

// Skill 技能配置。
type Skill struct {
	Dir string // skill 目录路径
}

// SandboxType 沙箱类型。
type SandboxType string

const (
	SandboxTypeReuse   SandboxType = "reuse"   // 复用容器模式（默认）
	SandboxTypeOneshot SandboxType = "oneshot" // 一次性容器模式
)

// RunnerType 运行器类型。
type RunnerType string

const (
	RunnerTypeOpencode RunnerType = "opencode" // opencode 智能体（默认）
)

// OutputFormat 输出格式。
type OutputFormat string

const (
	OutputFormatText OutputFormat = "text" // 文本格式（默认）
	OutputFormatJSON OutputFormat = "json" // JSON 事件流格式
)

// RunSession 执行会话标识。
type RunSession struct {
	ThreadID string // 对话会话 ID
	RunID    string // 执行请求 ID
}

type Option interface {
	apply(*RunOption) error
}

type OptionFunc func(*RunOption) error

func (f OptionFunc) apply(opts *RunOption) error {
	return f(opts)
}

// RunOption 运行选项。
type RunOption struct {
	RunSession     RunSession
	ModelConfig    ModelConfig
	Instruction    string
	OverallTask    string
	CurrentTask    string
	InputDir       string
	OutputDir      string
	Messages       []Message
	Skills         []Skill
	Tools          []Tool
	SandboxType    SandboxType
	RunnerType     RunnerType
	OutputFormat   OutputFormat
	EnableThinking bool
	SkipCleanup    bool
	AgentName      string
}

func (o *RunOption) Apply(opts ...Option) error {
	for _, opt := range opts {
		if err := opt.apply(o); err != nil {
			return err
		}
	}
	if o.RunSession.ThreadID == "" {
		o.RunSession.ThreadID = uuid.New().String()
	}
	if o.RunSession.RunID == "" {
		o.RunSession.RunID = uuid.New().String()
	}
	return nil
}

// WithModelConfig 设置模型配置（必须）。
func WithModelConfig(config ModelConfig) Option {
	return OptionFunc(func(opts *RunOption) error {
		opts.ModelConfig = config
		return nil
	})
}

// WithRunSession 设置执行会话标识。
func WithRunSession(session RunSession) Option {
	return OptionFunc(func(opts *RunOption) error {
		opts.RunSession = session
		return nil
	})
}

// WithInstruction 设置系统提示词。
func WithInstruction(instruction string) Option {
	return OptionFunc(func(opts *RunOption) error {
		opts.Instruction = instruction
		return nil
	})
}

// WithOverallTask 设置整体任务（用于多智能体场景）。
func WithOverallTask(overallTask string) Option {
	return OptionFunc(func(opts *RunOption) error {
		opts.OverallTask = overallTask
		return nil
	})
}

// WithCurrentTask 设置当前任务。
func WithCurrentTask(currentTask string) Option {
	return OptionFunc(func(opts *RunOption) error {
		opts.CurrentTask = currentTask
		return nil
	})
}

// WithInputDir 设置输入目录。
func WithInputDir(inputDir string) Option {
	return OptionFunc(func(opts *RunOption) error {
		opts.InputDir = inputDir
		return nil
	})
}

// WithOutputDir 设置输出目录。
func WithOutputDir(outputDir string) Option {
	return OptionFunc(func(opts *RunOption) error {
		opts.OutputDir = outputDir
		return nil
	})
}

// WithMessages 设置历史消息列表。
func WithMessages(messages []Message) Option {
	return OptionFunc(func(opts *RunOption) error {
		opts.Messages = messages
		return nil
	})
}

// WithSkills 设置技能列表。
func WithSkills(skills []Skill) Option {
	return OptionFunc(func(opts *RunOption) error {
		opts.Skills = skills
		return nil
	})
}

// WithTools 设置工具列表。
func WithTools(tools []Tool) Option {
	return OptionFunc(func(opts *RunOption) error {
		ctx := context.Background()
		for i := range tools {
			if tools[i].OpenAPI3Schema == nil {
				return fmt.Errorf("tool schema is required")
			}
			if err := openapi3_util.ValidateDoc(ctx, tools[i].OpenAPI3Schema); err != nil {
				return fmt.Errorf("invalid tool schema: %w", err)
			}
			if tools[i].APIAuth != nil && tools[i].APIAuth.Type != "none" && tools[i].APIAuth.Value == "" {
				return fmt.Errorf("tool [%s] auth value is empty", tools[i].Name)
			}
			if tools[i].Name == "" {
				tools[i].Name = tools[i].OpenAPI3Schema.Info.Title
			}
			if len(tools[i].OperationIDs) > 0 {
				tools[i].OpenAPI3Schema = openapi3_util.FilterDocOperations(tools[i].OpenAPI3Schema, tools[i].OperationIDs)
			}
			opts.Tools = append(opts.Tools, tools[i])
		}
		return nil
	})
}

// WithSandboxType 设置沙箱类型。
func WithSandboxType(sandboxType SandboxType) Option {
	return OptionFunc(func(opts *RunOption) error {
		opts.SandboxType = sandboxType
		return nil
	})
}

// WithRunnerType 设置运行器类型。
func WithRunnerType(runnerType RunnerType) Option {
	return OptionFunc(func(opts *RunOption) error {
		opts.RunnerType = runnerType
		return nil
	})
}

// WithOutputFormat 设置输出格式。
func WithOutputFormat(format OutputFormat) Option {
	return OptionFunc(func(opts *RunOption) error {
		opts.OutputFormat = format
		return nil
	})
}

// WithEnableThinking 启用思考模式。
func WithEnableThinking(enable bool) Option {
	return OptionFunc(func(opts *RunOption) error {
		opts.EnableThinking = enable
		return nil
	})
}

// WithSkipCleanup 跳过沙箱清理。
func WithSkipCleanup(skip bool) Option {
	return OptionFunc(func(opts *RunOption) error {
		opts.SkipCleanup = skip
		return nil
	})
}

// WithAgentName 设置智能体名称（用于日志标识）。
func WithAgentName(name string) Option {
	return OptionFunc(func(opts *RunOption) error {
		opts.AgentName = name
		return nil
	})
}
