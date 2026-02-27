package conversation

var mainAgent = &MainAgent{}

type MainAgent struct {
}

func init() {
	InitBuilder(mainAgent)
}

func (*MainAgent) EventType() int {
	return MainAgentEventType
}
func (*MainAgent) Build(conversationResp *ConversationResp, conversation, searchResult string, agentChatResp *AgentChatResp) error {
	if conversationResp.SearchList == nil && len(searchResult) > 0 {
		conversationResp.SearchList = &searchResult
	}
	if len(conversation) > 0 {
		//保存对话
		conversationResp.Write(conversation, agentChatResp.Order)
	}
	return nil
}
