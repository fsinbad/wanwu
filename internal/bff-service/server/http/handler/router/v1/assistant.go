package v1

import (
	"net/http"

	v1 "github.com/UnicomAI/wanwu/internal/bff-service/server/http/handler/v1"
	"github.com/UnicomAI/wanwu/internal/bff-service/server/http/middleware"
	mid "github.com/UnicomAI/wanwu/pkg/gin-util/mid-wrap"
	"github.com/gin-gonic/gin"
)

func registerAssistant(apiV1 *gin.RouterGroup) {
	mid.Sub("app.agent").Reg(apiV1, "/assistant/template/list", http.MethodGet, v1.GetAssistantTemplateList, "智能体模板列表")
	mid.Sub("app.agent").Reg(apiV1, "/assistant/template", http.MethodGet, v1.GetAssistantTemplate, "获取智能体模板")
	mid.Sub("app.agent").Reg(apiV1, "/assistant/template", http.MethodPost, v1.AssistantTemplateCreate, "复制智能体模板")

	mid.Sub("app.agent").Reg(apiV1, "/assistant", http.MethodPost, v1.AssistantCreate, "创建智能体")
	mid.Sub("app.agent").Reg(apiV1, "/assistant", http.MethodPut, v1.AssistantUpdate, "修改智能体基本信息")
	mid.Sub("app.agent").Reg(apiV1, "/assistant/config", http.MethodPut, v1.AssistantConfigUpdate, "修改智能体配置信息", middleware.AuthModelByModelId([]string{"modelConfig.modelId", "rerankConfig.modelId", "recommendConfig.modelConfig.modelId"}))
	mid.Sub("app.agent").Reg(apiV1, "/assistant/draft", http.MethodGet, v1.GetDraftAssistantInfo, "查看草稿智能体详情")
	mid.Sub("app.agent").Reg(apiV1, "/assistant", http.MethodGet, v1.GetPublishedAssistantInfo, "查看已发布智能体详情")
	mid.Sub("app.agent").Reg(apiV1, "/assistant/copy", http.MethodPost, v1.AssistantCopy, "智能体复制")

	mid.Sub("app.agent").Reg(apiV1, "/assistant/tool/workflow", http.MethodPost, v1.AssistantWorkFlowCreate, "添加工作流")
	mid.Sub("app.agent").Reg(apiV1, "/assistant/tool/workflow", http.MethodDelete, v1.AssistantWorkFlowDelete, "删除工作流")
	mid.Sub("app.agent").Reg(apiV1, "/assistant/tool/workflow/switch", http.MethodPut, v1.AssistantWorkFlowEnableSwitch, "启用/停用工作流")

	mid.Sub("app.agent").Reg(apiV1, "/assistant/tool/mcp", http.MethodPost, v1.AssistantMCPCreate, "添加mcp工具")
	mid.Sub("app.agent").Reg(apiV1, "/assistant/tool/mcp", http.MethodDelete, v1.AssistantMCPDelete, "删除mcp工具")
	mid.Sub("app.agent").Reg(apiV1, "/assistant/tool/mcp/switch", http.MethodPut, v1.AssistantMCPEnableSwitch, "启用/停用mcp工具")

	mid.Sub("app.agent").Reg(apiV1, "/assistant/tool", http.MethodPost, v1.AssistantToolCreate, "添加智能体工具，包括自定义工具和内置工具")
	mid.Sub("app.agent").Reg(apiV1, "/assistant/tool", http.MethodDelete, v1.AssistantToolDelete, "删除智能体工具，包括自定义工具和内置工具")
	mid.Sub("app.agent").Reg(apiV1, "/assistant/tool/switch", http.MethodPut, v1.AssistantToolEnableSwitch, "智能体启用/停用自定义内置工具")
	mid.Sub("app.agent").Reg(apiV1, "/assistant/tool/config", http.MethodPut, v1.AssistantToolConfig, "配置智能体工具，包括自定义工具和内置工具")

	mid.Sub("app.agent").Reg(apiV1, "/assistant/multi-agent", http.MethodPost, v1.MultiAgentCreate, "添加多智能体配置-子智能体")
	mid.Sub("app.agent").Reg(apiV1, "/assistant/multi-agent", http.MethodDelete, v1.MultiAgentDelete, "删除多智能体配置-子智能体")
	mid.Sub("app.agent").Reg(apiV1, "/assistant/multi-agent/config", http.MethodPut, v1.MultiAgentConfigUpdate, "编辑多智能体配置中子智能体描述")
	mid.Sub("app.agent").Reg(apiV1, "/assistant/multi-agent/switch", http.MethodPut, v1.MultiAgentEnableSwitch, "启用/停用多智能体配置-子智能体")

	mid.Sub("app.agent").Reg(apiV1, "/assistant/stream/draft", http.MethodPost, v1.DraftAssistantConversionStream, "草稿智能体流式问答")
	mid.Sub("app.agent").Reg(apiV1, "/assistant/select", http.MethodGet, v1.GetAssistantSelect, "添加多智能体配置-下拉列表接口")
	mid.Sub("app.agent").Reg(apiV1, "/assistant/question/recommend", http.MethodPost, v1.AssistantQuestionRecommend, "智能体问题推荐接口")
	mid.Sub("app.agent").Reg(apiV1, "/assistant/conversation/draft/detail", http.MethodGet, v1.DraftAssistantConversationDetailList, "草稿智能体对话详情历史列表")
	mid.Sub("app.agent").Reg(apiV1, "/assistant/conversation/draft", http.MethodDelete, v1.DraftAssistantConversationDelete, "草稿智能体对话删除")

}
