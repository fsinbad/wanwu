package response

import (
	"github.com/UnicomAI/wanwu/internal/agent-service/model/request"
	"github.com/cloudwego/eino/schema"
)

type AgentEventType int
type SubEventStatus int

const (
	MainAgentEventType = 0 //单智能体事件/多智能体主智能体
	SubAgentEventType  = 1 //子智能体事件

	EventStartStatus   SubEventStatus = 1 //开始事件
	EventProcessStatus SubEventStatus = 2 //输出中
	EventEndStatus     SubEventStatus = 3 //结束事件
	EventFailStatus    SubEventStatus = 4 //子智能体失败
)

type SubEventData struct {
	Status   SubEventStatus `json:"status"`
	Id       string         `json:"id"`
	Name     string         `json:"name"`
	Profile  string         `json:"profile"`
	TimeCost string         `json:"timeCost"`
	ParentId string         `json:"parentId"`
	Order    int            `json:"order"`
}

func BuildStartSubAgent(respContext *AgentChatRespContext) *SubEventData {
	return StartSubAgent(respContext.CurrentAgent, respContext.Order)
}

func BuildProcessSubAgent(respContext *AgentChatRespContext) *SubEventData {
	return ProcessSubAgent(respContext.CurrentAgent, respContext.Order)
}

func BuildEndSubAgent(respContext *AgentChatRespContext, timeCost string) *SubEventData {
	return EndSubAgent(respContext.CurrentAgent, timeCost, respContext.Order)
}

func StartSubAgent(agentInfo *AgentInfo, order int) *SubEventData {
	return &SubEventData{
		Status:  EventStartStatus,
		Id:      agentInfo.Id,
		Name:    agentInfo.Name,
		Profile: agentInfo.Avatar,
		Order:   order,
	}
}

func ProcessSubAgent(agentInfo *AgentInfo, order int) *SubEventData {
	if agentInfo == nil || len(agentInfo.Id) == 0 || len(agentInfo.Name) == 0 {
		return nil
	}
	return &SubEventData{
		Status:  EventProcessStatus,
		Id:      agentInfo.Id,
		Name:    agentInfo.Name,
		Profile: agentInfo.Avatar,
		Order:   order,
	}
}

func EndSubAgent(agentInfo *AgentInfo, timeCost string, order int) *SubEventData {
	return &SubEventData{
		Status:   EventEndStatus,
		Id:       agentInfo.Id,
		Name:     agentInfo.Name,
		Profile:  agentInfo.Avatar,
		TimeCost: timeCost,
		Order:    order,
	}
}

func buildSubAgentEventInfo(respContext *request.AgentChatContext, chatMessage *schema.Message, subAgentEventData *SubEventData, order int) ([]string, error) {
	var outputList = make([]string, 0)
	var agentChatResp = &AgentChatResp{
		Code:           agentSuccessCode,
		Message:        "success",
		Response:       "",
		Order:          order,
		EventType:      buildEventType(subAgentEventData),
		EventData:      subAgentEventData,
		GenFileUrlList: []interface{}{},
		History:        []interface{}{},
		SearchList:     buildSubAgentSearchList(subAgentEventData, respContext),
		Finish:         buildFinish(chatMessage, true),
		Usage:          buildUsage(chatMessage),
	}
	respString, err := buildRespString(agentChatResp)
	if err != nil {
		return nil, err
	}
	outputList = append(outputList, respString)
	return outputList, nil
}

// buildEventType 事件类型构造
func buildEventType(subEvent *SubEventData) AgentEventType {
	if subEvent == nil {
		return MainAgentEventType
	}
	return SubAgentEventType
}
