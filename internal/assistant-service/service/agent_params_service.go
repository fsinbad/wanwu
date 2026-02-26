package service

import (
	"encoding/json"
	"fmt"

	assistant_service "github.com/UnicomAI/wanwu/api/proto/assistant-service"
	"github.com/UnicomAI/wanwu/api/proto/common"
	params_process "github.com/UnicomAI/wanwu/internal/assistant-service/service/params-process"
	"github.com/UnicomAI/wanwu/pkg/log"
	mp "github.com/UnicomAI/wanwu/pkg/model-provider"
	safe_go_util "github.com/UnicomAI/wanwu/pkg/safe-go-util"
	"github.com/UnicomAI/wanwu/pkg/util"
)

const (
	maxHistory = 5
)

type AgentChatParamsBuilder struct {
	postProcessList []params_process.ServiceType
	agent           *params_process.AgentInfo
	userQueryParams *params_process.UserQueryParams
	params          *assistant_service.AgentDetail
	clientInfo      *params_process.ClientInfo
	err             error
}

func NewAgentChatParamsBuilder(agent *params_process.AgentInfo, userQueryParams *params_process.UserQueryParams, clientInfo *params_process.ClientInfo) *AgentChatParamsBuilder {
	return &AgentChatParamsBuilder{
		agent:           agent,
		clientInfo:      clientInfo,
		userQueryParams: userQueryParams,
		params: &assistant_service.AgentDetail{
			ToolParams: &assistant_service.ToolParams{},
		},
	}
}

func (a *AgentChatParamsBuilder) UserInput(input string, stream bool, uploadFile []string) *AgentChatParamsBuilder {
	if a.err != nil {
		return a
	}
	a.params.Input = input
	a.params.Stream = stream
	a.params.UploadFile = uploadFile
	return a
}
func (a *AgentChatParamsBuilder) AgentBaseParams() *AgentChatParamsBuilder {
	if a.err != nil {
		return a
	}
	assistant := a.agent.Assistant
	a.params.AgentBaseParams = &assistant_service.AgentBaseParams{
		Name:        assistant.Name,
		Description: assistant.Desc,
		Instruction: assistant.Instructions,
		AgentId:     assistant.UUID,
		Avatar:      assistant.AvatarPath,
	}
	return a
}
func (a *AgentChatParamsBuilder) ModelParams() *AgentChatParamsBuilder {
	if a.err != nil {
		return a
	}
	assistant := a.agent
	if a.agent.Assistant.ModelConfig == "" {
		a.err = fmt.Errorf("Assistant服务智能体模型配置为空，assistantId: %d", assistant.Assistant.ID)
		return a
	}
	params := &assistant_service.ModelParams{}
	modelConfig := &common.AppModelConfig{}
	if err := json.Unmarshal([]byte(a.agent.Assistant.ModelConfig), modelConfig); err != nil {
		a.err = fmt.Errorf("Assistant服务解析智能体模型配置失败，assistantId: %d, error: %v, modelConfigRaw: %s", assistant.Assistant.ID, err, assistant.Assistant.ModelConfig)
		return a
	}
	params.ModelId = modelConfig.ModelId
	params.MaxHistory = maxHistory
	_, modelParams, _ := mp.ToModelParams(modelConfig.Provider, modelConfig.ModelType, modelConfig.Config)
	buildModelParams(modelParams, params)

	if a.userQueryParams != nil && a.userQueryParams.ConversationId != "" {
		a.postProcessList = append(a.postProcessList, params_process.ConversionHistoryType)
	}

	a.params.ModelParams = params
	return a
}
func (a *AgentChatParamsBuilder) KnowledgeParams() *AgentChatParamsBuilder {
	if a.err != nil {
		return a
	}
	a.postProcessList = append(a.postProcessList, params_process.KnowledgeType)
	return a
}

func (a *AgentChatParamsBuilder) ToolParams() *AgentChatParamsBuilder {
	if a.err != nil {
		return a
	}
	a.postProcessList = append(a.postProcessList, params_process.PluginToolType)
	a.postProcessList = append(a.postProcessList, params_process.WorkflowType)
	a.postProcessList = append(a.postProcessList, params_process.McpType)
	return a
}

func (a *AgentChatParamsBuilder) Build() (detail *assistant_service.AgentDetail, err error) {
	if a.err != nil {
		return nil, a.err
	}
	defer util.PrintPanicStackWithCall(func(panicOccur bool, recoverError error) {
		if recoverError != nil {
			err = recoverError
		}
	})
	if len(a.postProcessList) > 0 {
		//准备参数
		prepareParams := prepareAgentParams(a)
		if prepareParams.Err != nil {
			return nil, prepareParams.Err
		}
		//构建参数
		err1 := buildAgentParams(a, prepareParams)
		if err1 != nil {
			return nil, err1
		}
	}
	return a.params, nil
}

// prepareAgentParams 准备参数
func prepareAgentParams(agent *AgentChatParamsBuilder) *params_process.AgentPrepareParams {
	prepareParams := &params_process.AgentPrepareParams{}
	serviceList := agent.postProcessList
	var fnList []func()
	for _, processService := range serviceList {
		fnList = append(fnList, func() {
			err := params_process.PrepareParams(processService, agent.agent, prepareParams, agent.clientInfo, agent.userQueryParams)
			if err != nil {
				prepareParams.Err = err
			}
			log.Infof("Assistant服务构建智能体准备参数，assistantId: %d,service %s done, err %v", agent.agent.Assistant.ID, processService, err)
		})
	}
	// 并发执行调用
	safe_go_util.SageGoWaitGroup(fnList...)
	return prepareParams
}

// buildAgentParams 构建智能体参数
func buildAgentParams(agent *AgentChatParamsBuilder, prepareParams *params_process.AgentPrepareParams) error {
	serviceList := agent.postProcessList
	for _, processService := range serviceList {
		err := params_process.BuildParams(processService, agent.agent, prepareParams, agent.params)
		if err != nil {
			log.Errorf("Assistant服务构建智能体参数失败，assistantId: %d,service %s error: %v", agent.agent.Assistant.ID, processService, err)
			return err
		}
		log.Infof("Assistant服务构建智能体参数，assistantId: %d,service %s done", agent.agent.Assistant.ID, processService)
	}
	return nil
}

func buildModelParams(params map[string]interface{}, modelParams *assistant_service.ModelParams) *assistant_service.ModelParams {
	if len(params) == 0 {
		return modelParams
	}
	modelParams.Temperature = toDouble(params["temperature"])
	modelParams.TopP = toDouble(params["top_p"])
	modelParams.FrequencyPenalty = toDouble(params["frequency_penalty"])
	modelParams.PresencePenalty = toDouble(params["presence_penalty"])
	return modelParams
}

func toDouble(data interface{}) *float64 {
	if data == nil {
		return nil
	}
	f, ok := data.(float64)
	if !ok {
		return nil
	}
	return &f
}
