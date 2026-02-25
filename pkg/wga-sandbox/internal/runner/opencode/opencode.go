// Package opencode 提供 opencode 智能体的运行器实现
package opencode

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"
	"text/template"

	openapi3_util "github.com/UnicomAI/wanwu/pkg/openapi3-util"
	"github.com/UnicomAI/wanwu/pkg/wga-sandbox/internal/runner"
	"github.com/UnicomAI/wanwu/pkg/wga-sandbox/internal/sandbox"
	wga_sandbox_option "github.com/UnicomAI/wanwu/pkg/wga-sandbox/wga-sandbox-option"
	"github.com/google/uuid"
)

// opencode.json 配置文件模板
const configTemplate = `{
  "$schema": "https://opencode.ai/config.json",
  "permission": "allow",
  "provider": {
    "{{.Provider}}": {
      "npm": "@ai-sdk/openai-compatible",
      "name": "{{.ProviderName}}",
      "options": {
        "baseURL": "{{.BaseURL}}",
        "apiKey": "{{.APIKey}}"
      },
      "models": {
        "{{.Model}}": {
          "name": "{{.ModelName}}"
        }
      }
    }
  }
}`

// 任务要求文件模板
const requirementTemplate = `# 任务要求

{{if .Instruction}}---

## 系统提示词

{{.Instruction}}

{{end}}{{if .OverallTask}}---

## 整体任务

{{.OverallTask}}

{{end}}{{if .Messages}}---

## 历史信息

{{range .Messages}}### {{.Role}}

{{.Content}}

{{end}}{{end}}{{if .CurrentTask}}---

## 当前任务

{{.CurrentTask}}

{{end}}`

// runPromptTemplate 是传递给 opencode run 的提示词
// 要求智能体读取 .requirement.md 文件并执行任务
const runPromptTemplate = `读取 .requirement.md 文件，理解其中的系统提示词、整体任务、历史信息和当前任务，然后根据你的角色定义自主完成任务。
要求：
1. 使用语言与 .requirement.md 中的"整体任务"保持一致
2. 充分利用工作目录中的现有内容
3. 优先使用已配置的工具和技能
4. 自行决定最终结果是直接输出还是保存到工作目录
5. 完整执行所有步骤，不要中途停止或等待用户确认
6. 不要在输出中包含或让用户感知到 .requirement.md`

// 确保 Runner 实现 runner.Runner 接口
var _ runner.Runner = (*Runner)(nil)

// Runner 实现 opencode 智能体运行器
type Runner struct {
	sb  sandbox.Sandbox
	req runner.RunRequest
}

// NewRunner 创建 opencode 运行器实例
func NewRunner(sb sandbox.Sandbox, req runner.RunRequest) runner.Runner {
	return &Runner{
		sb:  sb,
		req: req,
	}
}

// BeforeRun 执行前准备工作：
// 1. 创建 opencode 配置文件
// 2. 创建任务要求文件
// 3. 复制 skills 和 tools
// 4. 复制输入文件
// 注意：沙箱环境已在 Manager.Create 时通过 Prepare 初始化，此处不再调用
func (r *Runner) BeforeRun(ctx context.Context) error {
	if err := r.setupConfig(ctx); err != nil {
		return err
	}

	if err := r.createRequirementFile(ctx); err != nil {
		return err
	}

	// 复制 skills 到工作目录
	if len(r.req.Skills) > 0 {
		if _, err := r.sb.ExecuteSync(ctx, "mkdir", "-p", ".opencode/skills"); err != nil {
			return fmt.Errorf("failed to create skills directory: %w", err)
		}
		for _, skill := range r.req.Skills {
			dirName := path.Base(skill.Dir)
			if err := r.sb.CopyToSandbox(ctx, skill.Dir, ".opencode/skills/"+dirName); err != nil {
				return fmt.Errorf("failed to copy skill %s to workspace: %w", dirName, err)
			}
		}
	}

	// 转换 tools 为 skills 并复制到工作目录
	if len(r.req.Tools) > 0 {
		if _, err := r.sb.ExecuteSync(ctx, "mkdir", "-p", ".opencode/tools"); err != nil {
			return fmt.Errorf("failed to create tools directory: %w", err)
		}
		if _, err := r.sb.ExecuteSync(ctx, "mkdir", "-p", ".opencode/skills"); err != nil {
			return fmt.Errorf("failed to create skills directory: %w", err)
		}
		for _, tool := range r.req.Tools {
			// 写入 OpenAPI schema 文件
			schemaData, err := json.Marshal(tool.OpenAPI3Schema)
			if err != nil {
				return fmt.Errorf("failed to marshal tool schema %s: %w", tool.Name, err)
			}
			dstFileName := fmt.Sprintf("%s.%s.json", toSkillName(tool.Name), uuid.New().String()[:8])
			dstPath := ".opencode/tools/" + dstFileName
			if err := writeFileViaBase64(ctx, r.sb, dstPath, string(schemaData)); err != nil {
				return fmt.Errorf("failed to write tool schema %s: %w", tool.Name, err)
			}

			// 使用 openapi-to-skills 转换为 skill
			skillName := toSkillName(tool.Name)
			if _, err := r.sb.ExecuteSync(ctx, "openapi-to-skills", dstPath, "-o", ".opencode/skills", "-n", skillName, "-f"); err != nil {
				return fmt.Errorf("failed to convert tool %s to skill: %w", tool.Name, err)
			}

			// 追加 API 认证信息
			if tool.APIAuth != nil && tool.APIAuth.Type != "none" && tool.APIAuth.Value != "" {
				skillDir := ".opencode/skills/" + skillName
				authLine := formatAuthLine(tool.APIAuth)
				authPath := fmt.Sprintf("%s/references/authentication.md", skillDir)
				encoded := base64.StdEncoding.EncodeToString([]byte(authLine))
				cmd := fmt.Sprintf("echo '%s' | base64 -d >> %s", encoded, authPath)
				if _, err := r.sb.ExecuteSync(ctx, "sh", "-c", cmd); err != nil {
					return fmt.Errorf("failed to update authentication.md for tool %s: %w", tool.Name, err)
				}
			}
		}
	}

	// 复制输入文件到工作目录
	if r.req.InputDir != "" {
		if err := r.sb.CopyToSandbox(ctx, r.req.InputDir+"/."); err != nil {
			return fmt.Errorf("failed to copy input to workspace: %w", err)
		}
	}

	return nil
}

// Run 执行 opencode run 命令，返回输出流
func (r *Runner) Run(ctx context.Context) (<-chan string, error) {
	args := []string{"opencode", "run"}
	if r.req.OutputFormat == wga_sandbox_option.OutputFormatJSON {
		args = append(args, "--format", "json")
	}
	if r.req.EnableThinking {
		args = append(args, "--thinking")
	}
	args = append(args, runPromptTemplate)
	return r.sb.Execute(ctx, args...)
}

// AfterRun 执行后处理：
// 复制输出文件到本地（如果指定了 OutputDir）
// 沙箱清理由外部统一管理，不在此处处理
func (r *Runner) AfterRun(ctx context.Context) error {
	if r.req.OutputDir != "" {
		return r.copyOutput(ctx)
	}
	return nil
}

// setupConfig 创建 opencode 配置文件
func (r *Runner) setupConfig(ctx context.Context) error {
	if _, err := r.sb.ExecuteSync(ctx, "mkdir", "-p", ".opencode"); err != nil {
		return fmt.Errorf("failed to create .opencode directory: %w", err)
	}

	content := renderConfig(r.req.ModelConfig)
	if err := writeFileViaBase64(ctx, r.sb, ".opencode/opencode.json", content); err != nil {
		return fmt.Errorf("failed to create opencode.json: %w", err)
	}

	return nil
}

// createRequirementFile 创建任务要求文件
func (r *Runner) createRequirementFile(ctx context.Context) error {
	content := renderRequirement(r.req.Instruction, r.req.OverallTask, r.req.CurrentTask, r.req.Messages)
	if err := writeFileViaBase64(ctx, r.sb, ".requirement.md", content); err != nil {
		return fmt.Errorf("failed to create requirement file: %w", err)
	}

	return nil
}

// copyOutput 复制输出文件到本地，并移除隐藏文件
func (r *Runner) copyOutput(ctx context.Context) error {
	if err := r.sb.CopyFromSandbox(ctx, r.req.OutputDir); err != nil {
		return fmt.Errorf("failed to copy output from workspace: %w", err)
	}

	// 移除隐藏文件（如 .opencode 目录）
	entries, err := os.ReadDir(r.req.OutputDir)
	if err != nil {
		return fmt.Errorf("failed to read output directory: %w", err)
	}

	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			removePath := r.req.OutputDir + "/" + entry.Name()
			if err := os.RemoveAll(removePath); err != nil {
				return fmt.Errorf("failed to remove hidden file %s: %w", entry.Name(), err)
			}
		}
	}

	return nil
}

// renderConfig 渲染 opencode 配置文件
func renderConfig(config wga_sandbox_option.ModelConfig) string {
	tmpl, err := template.New("config").Parse(configTemplate)
	if err != nil {
		return ""
	}
	var buf strings.Builder
	if err := tmpl.Execute(&buf, config); err != nil {
		return ""
	}
	return buf.String()
}

// renderRequirement 渲染任务要求文件
func renderRequirement(instruction, overallTask, currentTask string, messages []wga_sandbox_option.Message) string {
	tmpl, err := template.New("requirement").Parse(requirementTemplate)
	if err != nil {
		return ""
	}
	data := struct {
		Instruction string
		OverallTask string
		CurrentTask string
		Messages    []wga_sandbox_option.Message
	}{
		Instruction: instruction,
		OverallTask: overallTask,
		CurrentTask: currentTask,
		Messages:    messages,
	}
	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return ""
	}
	return buf.String()
}

// toSkillName 将工具名称转换为 skill 名称
// 替换空格为连字符，移除括号等特殊字符
func toSkillName(name string) string {
	result := strings.ReplaceAll(name, " ", "-")
	result = strings.ReplaceAll(result, "(", "")
	result = strings.ReplaceAll(result, ")", "")
	return result
}

// formatAuthLine 格式化认证信息为 Markdown 格式
func formatAuthLine(auth *openapi3_util.Auth) string {
	if auth.Type == "none" || auth.Value == "" {
		return ""
	}
	var authDesc string
	switch auth.In {
	case "header":
		authDesc = fmt.Sprintf("Header: `%s: %s`", auth.Name, auth.Value)
	case "query":
		authDesc = fmt.Sprintf("Query Parameter: `%s=%s`", auth.Name, auth.Value)
	default:
		authDesc = fmt.Sprintf("Auth Value: `%s`", auth.Value)
	}
	return fmt.Sprintf("\n- **API Key:** %s\n", authDesc)
}

// writeFileViaBase64 通过 base64 编码写入文件，避免特殊字符问题
func writeFileViaBase64(ctx context.Context, sb sandbox.Sandbox, dstPath, content string) error {
	encoded := base64.StdEncoding.EncodeToString([]byte(content))
	cmd := fmt.Sprintf("echo '%s' | base64 -d > %s", encoded, dstPath)
	_, err := sb.ExecuteSync(ctx, "sh", "-c", cmd)
	return err
}
