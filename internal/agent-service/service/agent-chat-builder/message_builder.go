package agent_chat_builder

import (
	"github.com/UnicomAI/wanwu/internal/agent-service/model/request"
	"github.com/UnicomAI/wanwu/internal/agent-service/model/response"
	"github.com/cloudwego/eino/schema"
)

type MessageType int
type AgentEventType int
type SubEventStatus int

const (
	SingleAgentMessage MessageType = 0 //单智能体消息
	MultiAgentMessage  MessageType = 1 //多智能体消息
)

type AgentMessageBuilder interface {
	MessageType() MessageType //消息类型
	FilterMessage(respContext *response.AgentChatRespContext, chatMessage *schema.Message) bool
	BuildContent(req *request.AgentChatContext, respContext *response.AgentChatRespContext, chatMessage *schema.Message) (*AgentMessageContent, error)
}

type AgentMessageContent struct {
	ContentList  []string
	SubEventData *response.SubEventData
	NotStop      bool
}

func BuildChatMessage(req *request.AgentChatContext, respContext *response.AgentChatRespContext, chatMessage *schema.Message) ([]string, error) {
	//创建构造器
	builder := createBuilder(req)
	//过滤数据
	if builder.FilterMessage(respContext, chatMessage) {
		return make([]string, 0), nil
	}
	//构造内容
	content, err := builder.BuildContent(req, respContext, chatMessage)
	if err != nil {
		return nil, err
	}
	//返回对端resp
	return response.BuildAgentChatResp(req, chatMessage, content.ContentList, content.SubEventData, content.NotStop, respContext.Order)
}

func createBuilder(req *request.AgentChatContext) AgentMessageBuilder {
	var builder AgentMessageBuilder
	if req.AgentChatReq.MultiAgent {
		builder = NewMultiBuilder()
	} else {
		builder = NewSingleBuilder()
	}
	return builder
}

// filterMessage 过滤非法消息
func filterMessage(respContext *response.AgentChatRespContext, chatMessage *schema.Message) bool {
	if string(chatMessage.Role) == "" && chatMessage.Content == "" {
		return true
	}
	messageTool := CreateMessageTool(chatMessage, respContext)
	if messageTool.ToolStart() && !messageTool.ToolEnd() && !messageTool.ToolParamsEnd() {
		//在工具输出内，但是此消息没有同时包含参数输出结束和工具调用结束
		//此时，如果content 也还是空，则完全不需要处理
		tool := chatMessage.ToolCalls[0]
		function := tool.Function
		if len(function.Name) == 0 && len(function.Arguments) == 0 && len(chatMessage.Content) == 0 {
			return true
		}
	}
	return false
}

func buildSkipMessage() *AgentMessageContent {
	return &AgentMessageContent{}
}

func buildMessageContent(contentList []string, subEventData *response.SubEventData) *AgentMessageContent {
	return &AgentMessageContent{
		ContentList:  contentList,
		SubEventData: subEventData,
	}
}
