// Package opencode 提供 opencode 智能体的运行器实现
package opencode

import "encoding/json"

// OpencodeEventType opencode 输出事件类型
//
// opencode 使用 JSON Lines 格式输出事件流，每个事件包含 type 字段标识类型。
// 目前处理的事件类型：
//   - text: 文本输出
//   - tool_use: 工具调用
//   - reasoning: 推理/思考过程
//
// 忽略的事件类型：
//   - step_start, step_finish: 步骤开始/结束
//   - snapshot, patch: 状态快照/补丁
//   - file, agent, retry, subtask, compaction: 其他事件
type OpencodeEventType string

const (
	OpencodeEventTypeStepStart  OpencodeEventType = "step_start"
	OpencodeEventTypeStepFinish OpencodeEventType = "step_finish"
	OpencodeEventTypeText       OpencodeEventType = "text"      // 文本输出
	OpencodeEventTypeToolUse    OpencodeEventType = "tool_use"  // 工具调用
	OpencodeEventTypeReasoning  OpencodeEventType = "reasoning" // 推理/思考过程
	OpencodeEventTypeSnapshot   OpencodeEventType = "snapshot"
	OpencodeEventTypePatch      OpencodeEventType = "patch"
	OpencodeEventTypeFile       OpencodeEventType = "file"
	OpencodeEventTypeAgent      OpencodeEventType = "agent"
	OpencodeEventTypeRetry      OpencodeEventType = "retry"
	OpencodeEventTypeSubtask    OpencodeEventType = "subtask"
	OpencodeEventTypeCompaction OpencodeEventType = "compaction"
)

// OpencodeEvent opencode 输出事件结构
//
// opencode 使用 JSON Lines 格式，每行一个事件：
//
//	{"type":"text","timestamp":1234567890,"sessionID":"xxx","part":{"text":"Hello"}}
//	{"type":"tool_use","timestamp":1234567891,"sessionID":"xxx","part":{"tool":"bash","state":{...}}}
type OpencodeEvent struct {
	Type      OpencodeEventType `json:"type"`      // 事件类型
	Timestamp int64             `json:"timestamp"` // 时间戳（毫秒）
	SessionID string            `json:"sessionID"` // 会话 ID
	Part      json.RawMessage   `json:"part"`      // 事件内容，具体结构取决于 Type
}
