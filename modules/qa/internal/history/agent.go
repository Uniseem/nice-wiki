package history

import (
	"context"
	"encoding/json"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/qa/internal/gen"
	qaruntime "nice-wiki/modules/qa/internal/runtime"
)

type AgentTurn struct {
	State     json.RawMessage           `json:"state,omitempty"`
	Messages  []platform.GatewayMessage `json:"messages"`
	Citations []gen.Citation            `json:"citations"`
}

// ForGateway 从独立的服务端记录重放原生工具消息，访客原话永远只作为user文本。
func (c Conversation) ForGateway() []platform.GatewayMessage {
	result := []platform.GatewayMessage{}
	for _, message := range c.Messages {
		if agent, exists := c.AgentTurns[message.MessageId]; exists {
			for _, item := range agent.Messages {
				if item.Role == "tool" {
					// 旧目录不跨轮保留；每轮都有当前全站指南，展开分区时重新读取。
					for _, prior := range agent.Messages {
						for _, call := range prior.ToolCalls {
							if call.ID == item.ToolCallID && call.Name == "read_guide" {
								item.Content = `{"message":"目录导航已由本轮全站AGENTS.md更新，需要分区细节时请重新read_guide。"}`
							}
						}
					}
				}
				result = append(result, item)
			}
			continue
		}
		// 旧式model_text没有完整的资料来源身份，升级后只保留原问答，需要事实时重新查阅。
		result = append(result, platform.GatewayMessage{Role: string(message.Role), Content: message.Text})
	}
	return result
}

func (c Conversation) AgentCitations() []gen.Citation {
	result := []gen.Citation{}
	seen := map[string]bool{}
	for _, message := range c.Messages {
		for _, citation := range c.AgentTurns[message.MessageId].Citations {
			if !seen[citation.CitationId] {
				seen[citation.CitationId] = true
				result = append(result, citation)
			}
		}
	}
	return result
}

// AgentState 只返回最后一轮经发布版本复核的原生检查点。
func (c Conversation) AgentState() string {
	for index := len(c.Messages) - 1; index >= 0; index-- {
		if turn, ok := c.AgentTurns[c.Messages[index].MessageId]; ok && len(turn.State) > 0 {
			return string(turn.State)
		}
	}
	return ""
}

// ForAgentGateway 只补检查点之后尚未进入原生会话的消息，避免再次发送全部原始历史。
func (c Conversation) ForAgentGateway() []platform.GatewayMessage {
	latest := -1
	for index, message := range c.Messages {
		if len(c.AgentTurns[message.MessageId].State) > 0 {
			latest = index
		}
	}
	if latest < 0 {
		return c.ForGateway()
	}
	turn := c.Messages[latest].TurnId
	remainder := c
	remainder.Messages = []gen.Message{}
	for _, message := range c.Messages[latest+1:] {
		if message.TurnId != turn {
			remainder.Messages = append(remainder.Messages, message)
		}
	}
	return remainder.ForGateway()
}

// validateAgentSources 批量核对原生历史材料，版本变化或撤回后不把旧正文交给下一轮模型。
func (c *Conversation) validateAgentSources(ctx context.Context, op OperationContext) error {
	refs := []gen.PublishedRevisionRef{}
	seen := map[gen.PublishedRevisionRef]bool{}
	for _, turn := range c.AgentTurns {
		for _, citation := range turn.Citations {
			if !seen[citation.Document] {
				seen[citation.Document] = true
				refs = append(refs, citation.Document)
			}
		}
	}
	if len(refs) == 0 {
		return nil
	}
	runtime, err := qaruntime.FromContext(ctx)
	if err != nil {
		return err
	}
	if runtime.Content == nil {
		return platform.NewError(platform.CodeDependencyUnavailable, false, "历史材料发布权威未装配")
	}
	valid := map[gen.PublishedRevisionRef]bool{}
	for start := 0; start < len(refs); start += 100 {
		batch := refs[start:min(start+100, len(refs))]
		raw, err := runtime.Content.ValidatePublications(qaruntime.ServiceReadContext(ctx, op), struct {
			Refs []gen.PublishedRevisionRef `json:"refs"`
		}{batch})
		if err != nil {
			return err
		}
		var response struct {
			Validations []struct {
				Reference gen.PublishedRevisionRef `json:"reference"`
				Visible   bool                     `json:"visible"`
				Matches   bool                     `json:"matchesCurrent"`
				Revision  int                      `json:"currentRevision"`
			} `json:"validations"`
		}
		if err := qaruntime.DecodeResponse(runtime, "ContentValidatePublicationsResult", raw, &response); err != nil {
			return err
		}
		if len(response.Validations) != len(batch) {
			return platform.NewError(platform.CodeIntegrityError, false, "历史材料发布核对不完整")
		}
		for index, item := range response.Validations {
			if item.Reference != batch[index] {
				return platform.NewError(platform.CodeIntegrityError, false, "历史材料发布身份不一致")
			}
			valid[item.Reference] = item.Visible && item.Matches && item.Revision == item.Reference.Revision
		}
	}
	for id, turn := range c.AgentTurns {
		for _, citation := range turn.Citations {
			if !valid[citation.Document] {
				delete(c.AgentTurns, id)
				delete(c.ModelTexts, id)
				break
			}
		}
	}
	return nil
}

func SaveAgentTurn(ctx context.Context, op OperationContext, turnID string, turn AgentTurn) error {
	raw, err := json.Marshal(turn)
	if err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "本轮工具消息无法保存")
	}
	tx, err := qaruntime.BeginWrite(ctx, op)
	if err != nil {
		return err
	}
	defer tx.Close()
	if _, err := tx.Tx.ExecContext(tx.Context, `UPDATE messages SET agent_messages_json=? WHERE message_id=?`, raw, "question."+turnID); err != nil {
		return qaruntime.StorageError(err)
	}
	// 只保留最新完整 checkpoint，逐轮原始工具消息仍保留，避免重复保存累计上下文。
	if _, err := tx.Tx.ExecContext(tx.Context, `UPDATE messages SET agent_messages_json=json_remove(agent_messages_json,'$.state') WHERE conversation_id=(SELECT conversation_id FROM messages WHERE message_id=?) AND message_id<>? AND json_valid(agent_messages_json) AND json_type(agent_messages_json,'$.state') IS NOT NULL`, "question."+turnID, "question."+turnID); err != nil {
		return qaruntime.StorageError(err)
	}
	return tx.Commit()
}
