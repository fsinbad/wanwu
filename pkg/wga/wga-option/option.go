// Package wga_option 提供 wga 包的选项类型定义。
package wga_option

import (
	mp_common "github.com/UnicomAI/wanwu/pkg/model-provider/mp-common"
	"github.com/UnicomAI/wanwu/pkg/util"
)

// ModelConfig 模型配置。
type ModelConfig struct {
	Model       string              // 模型标识
	ApiKey      string              // API 密钥
	EndpointUrl string              // API 端点地址
	Params      mp_common.LLMParams // 模型参数
}

// ToolConfig 工具配置。
type ToolConfig struct {
	Title   string                  // 工具标题（对应 OpenAPI schema 的 info.title）
	APIAuth *util.ApiAuthWebRequest // API 认证配置
}

// WorkspaceConfig 工作空间配置。
type WorkspaceConfig struct {
	InputDir  string // 输入目录路径
	OutputDir string // 输出目录路径
}

// RunSession 执行会话标识。
type RunSession struct {
	ThreadID string // 对话会话 ID
	RunID    string // 执行请求 ID
}
