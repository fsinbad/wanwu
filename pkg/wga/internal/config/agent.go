package config

import (
	"context"
	"fmt"
	"os"
	"path"
	"slices"

	"github.com/UnicomAI/wanwu/pkg/log"
	openapi3_util "github.com/UnicomAI/wanwu/pkg/openapi3-util"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/spf13/viper"
)

// Agent 智能体配置。
type Agent struct {
	ID             string          `json:"id"`
	Type           AgentType       `json:"type"`
	Name           string          `json:"name"`
	Description    string          `json:"description"`
	Configure      agentConfigure  `json:"configure"`
	Prompt         string          `json:"prompt"`
	ToolCategories []*ToolCategory `json:"tool_categories"`
	SubAgents      []*Agent        `json:"sub_agents"`
}

// LoadAgents 从配置文件加载智能体配置。
// configPath 为配置文件路径，支持 YAML 格式。
func LoadAgents(ctx context.Context, configPath string) ([]*Agent, error) {
	cfg := &all{}
	v := viper.New()
	v.SetConfigFile(configPath)
	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, err
	}

	baseDir := path.Dir(configPath)
	var agents []*Agent
	for _, at := range cfg.Agents {
		agent, err := at.load(ctx, baseDir, "")
		if err != nil {
			return nil, err
		}
		// 一级通用智能体ID不能重复
		for _, a := range agents {
			if a.ID == agent.ID {
				return nil, fmt.Errorf("load agent [%v(%v)] already exist", agent.ID, agent.Type)
			}
		}
		agents = append(agents, agent)
	}
	return agents, nil
}

// ToolCategory 工具类别配置。
type ToolCategory struct {
	Category  ToolCategoryType      `json:"category"`
	Condition ToolCategoryCondition `json:"condition"`
	Tools     []*Tool               `json:"tools"`
}

// Tool 工具配置。
type Tool struct {
	Doc          *openapi3.T     `json:"-"`             // OpenAPI schema
	SchemaPath   string          `json:"-"`             // schema 文件路径
	AuthRequired bool            `json:"auth_required"` // 是否需要认证
	Operations   []toolOperation `json:"operations"`    // 允许的操作
}

// 内部配置结构

type all struct {
	Agents []agentTemplate `json:"agents" mapstructure:"agents"`
}

type agentConfig struct {
	ID          string    `json:"id" mapstructure:"id"`
	Type        AgentType `json:"type" mapstructure:"type"`
	Name        string    `json:"name" mapstructure:"name"`
	Description string    `json:"description" mapstructure:"description"`

	Configure agentConfigure `json:"configure" mapstructure:"configure"`

	PromptRelativePath string `json:"prompt_relative_path" mapstructure:"prompt_relative_path"`

	ToolCategories []toolCategory `json:"tool_categories" mapstructure:"tool_categories"`

	SubAgents []agentTemplate `json:"sub_agents" mapstructure:"sub_agents"`
}

type agentConfigure struct {
	MaxIterations  int  `json:"max_iterations" mapstructure:"max_iterations"`
	EnableThinking bool `json:"enable_thinking" mapstructure:"enable_thinking"`
}

type agentTemplate struct {
	RelativePath string `json:"relative_path" mapstructure:"relative_path"`
}

func (at *agentTemplate) load(ctx context.Context, baseDir, classPrefix string) (*Agent, error) {
	configPath := path.Join(baseDir, at.RelativePath)
	cfg := &agentConfig{}
	v := viper.New()
	v.SetConfigFile(configPath)
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("load agent (%v) err: %v", configPath, err)
	}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("unmarshal agent (%v) err: %v", configPath, err)
	}
	if cfg.ID == "" {
		return nil, fmt.Errorf("load agent (%v) id empty", configPath)
	}
	if cfg.Name == "" {
		return nil, fmt.Errorf("load agent (%v) name empty", configPath)
	}
	agent := &Agent{
		ID:          cfg.ID,
		Type:        cfg.Type,
		Name:        cfg.Name,
		Description: cfg.Description,
		Configure:   cfg.Configure,
	}
	log.Debugf("[WGA][CONFIG] %vload agent [%v(%v)], %v", classPrefix, agent.ID, agent.Type, configPath)
	// prompt
	if cfg.PromptRelativePath != "" {
		promptPath := path.Join(path.Dir(configPath), cfg.PromptRelativePath)
		b, err := os.ReadFile(promptPath)
		if err != nil {
			return nil, fmt.Errorf("load agent (%v) read prompt (%v) err: %v", configPath, promptPath, err)
		}
		agent.Prompt = string(b)
	}
	// tools
	var tools []string        // 当前智能体下所有的tool(title)唯一
	var categories []string   // 当前智能体下的所有category唯一
	var operationIDs []string // 当前智能体下的所有operation唯一
	for _, tc := range cfg.ToolCategories {
		if slices.Contains(categories, string(tc.Category)) {
			return nil, fmt.Errorf("load agent (%v), tool category (%v) already exist", configPath, tc.Category)
		}
		categories = append(categories, string(tc.Category))
		category := &ToolCategory{
			Category:  tc.Category,
			Condition: tc.Condition,
		}
		for _, tt := range tc.Tools {
			tool, err := tt.load(ctx, path.Dir(configPath), classPrefix+"  ")
			if err != nil {
				return nil, fmt.Errorf("load agent (%v) err: %v", configPath, err)
			}
			if slices.Contains(tools, tool.Doc.Info.Title) {
				return nil, fmt.Errorf("load agent (%v), tool (%v) already exist", configPath, tool.Doc.Info.Title)
			}
			tools = append(tools, tool.Doc.Info.Title)
			for _, toolOperation := range tool.Operations {
				if slices.Contains(operationIDs, toolOperation.OperationID) {
					return nil, fmt.Errorf("load agent (%v), tool operation (%v) already exist", configPath, toolOperation.OperationID)
				}
				operationIDs = append(operationIDs, toolOperation.OperationID)
			}
			category.Tools = append(category.Tools, tool)
		}
		agent.ToolCategories = append(agent.ToolCategories, category)
	}
	// sub agents
	for _, at := range cfg.SubAgents {
		subAgent, err := at.load(ctx, path.Dir(configPath), classPrefix+"  ")
		if err != nil {
			return nil, fmt.Errorf("load agent (%v) err: %v", configPath, err)
		}
		for _, sa := range agent.SubAgents {
			if sa.ID == subAgent.ID {
				return nil, fmt.Errorf("load agent (%v), sub agent (%v) already exist", configPath, subAgent.ID)
			}
		}
		agent.SubAgents = append(agent.SubAgents, subAgent)
	}
	return agent, nil
}

type toolCategory struct {
	Category  ToolCategoryType      `json:"category" mapstructure:"category"`
	Condition ToolCategoryCondition `json:"condition" mapstructure:"condition"`
	Tools     []toolTemplate        `json:"tools" mapstructure:"tools"`
}

type toolTemplate struct {
	RelativePath string          `json:"relative_path" mapstructure:"relative_path"`
	AuthRequired bool            `json:"auth_required" mapstructure:"auth_required"`
	Operations   []toolOperation `json:"operations" mapstructure:"operations"`
}

type toolOperation struct {
	OperationID    string `json:"operation_id" mapstructure:"operation_id"`
	ReturnDirectly bool   `json:"return_directly" mapstructure:"return_directly"`
}

func (tt *toolTemplate) load(ctx context.Context, baseDir, classPrefix string) (*Tool, error) {
	configPath := path.Join(baseDir, tt.RelativePath)
	schema, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	doc, err := openapi3_util.LoadFromData(ctx, schema)
	if err != nil {
		return nil, fmt.Errorf("load tool (%v) err: %v", configPath, err)
	}
	// check operations
	if len(tt.Operations) == 0 {
		return nil, fmt.Errorf("load tool (%v) operations empty", configPath)
	}
	var operationIDs []string
	for _, toolOperation := range tt.Operations {
		if slices.Contains(operationIDs, toolOperation.OperationID) {
			return nil, fmt.Errorf("load tool (%v) operation (%v) duplicate", configPath, toolOperation.OperationID)
		}
		operationIDs = append(operationIDs, toolOperation.OperationID)
		var exist bool
		for _, pathItem := range doc.Paths {
			for _, operation := range pathItem.Operations() {
				if operation.OperationID == toolOperation.OperationID {
					exist = true
				}
				if exist {
					break
				}
			}
			if exist {
				break
			}
		}
		if !exist {
			return nil, fmt.Errorf("load tool (%v) operation (%v) not exist", configPath, toolOperation.OperationID)
		}
	}
	log.Debugf("[WGA][CONFIG] %vload tool %v, %v", classPrefix, tt.Operations, configPath)
	return &Tool{
		Doc:          doc,
		SchemaPath:   configPath,
		AuthRequired: tt.AuthRequired,
		Operations:   tt.Operations,
	}, nil
}
