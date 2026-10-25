package turn

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/modelagent"
	"nice-wiki/modules/qa/internal/gen"
	"nice-wiki/modules/qa/internal/stream"
)

// turnProgress 投影每次真实活动的生命周期；只展示公开说明与只读工具的必要参数，不转发内部上下文。
type turnProgress struct {
	op            OperationContext
	turnID, lease string
	started       map[string]time.Time
	round         int
}

func (p *turnProgress) emit(ctx context.Context, id string, value gen.AnswerStep) {
	started, exists := p.started[id]
	if !exists {
		started = time.Now()
		p.started[id] = started
	}
	startedAt := started.UTC()
	value.Id, value.StartedAt = &id, &startedAt
	if value.Status != gen.AnswerStepStatusRunning {
		ms := int(time.Since(started).Milliseconds())
		value.DurationMs = &ms
	}
	if value.Detail != nil {
		detail := clip(*value.Detail, 2000)
		value.Detail = &detail
	}
	if value.Items != nil {
		items := append([]string{}, (*value.Items)[:min(20, len(*value.Items))]...)
		for index := range items {
			items[index] = clip(items[index], 300)
		}
		value.Items = &items
	}
	_, _ = stream.AppendAnswerEvent(ctx, p.op, stream.AppendAnswerEventInput{TurnID: p.turnID, LeaseID: p.lease, Event: gen.AnswerEvent{Event: "step", Step: &value}})
}

func (p *turnProgress) stage(ctx context.Context, key gen.AnswerStepKey, status gen.AnswerStepStatus, title, detail string, items []string) {
	value := gen.AnswerStep{Key: key, Status: status, Title: title}
	if detail != "" {
		value.Detail = &detail
	}
	if len(items) > 0 {
		value.Items = &items
	}
	p.emit(ctx, string(key), value)
}

func (p *turnProgress) model(ctx context.Context, round int, output *platform.ModelOutput) {
	p.round = round
	value := gen.AnswerStep{Key: gen.Answer, Status: gen.AnswerStepStatusRunning, Title: "分析资料", Round: &round}
	if output != nil {
		value.Status = gen.AnswerStepStatusDone
		if len(output.ToolCalls) == 0 {
			value.Title = "整理回答"
		} else if output.Text != nil && *output.Text != "" {
			// 这是原本会作为公开正文流出的过程说明；工具回合结束后单独保留，不把它混入最终答案。
			value.Detail = output.Text
		}
	}
	p.emit(ctx, "model."+strconv.Itoa(round), value)
}

func (p *turnProgress) tool(ctx context.Context, call platform.GatewayToolCall, result *modelagent.ToolResult) {
	name := gen.AnswerStepTool(call.Name)
	value := gen.AnswerStep{Key: gen.Search, Status: gen.AnswerStepStatusRunning, Tool: &name, Round: &p.round}
	var arguments struct {
		Query          string `json:"query"`
		RootDocumentID string `json:"rootDocumentId"`
		Offset         int    `json:"offset"`
	}
	_ = json.Unmarshal([]byte(call.Arguments), &arguments)
	switch call.Name {
	case "report_abuse":
		value.Title = "上报滥用信号"
	case "read_guide":
		value.Title = "阅读全站指南"
		if arguments.RootDocumentID != "" {
			value.Title = "阅读分区指南"
		}
	case "search_documents":
		value.Title = "搜索文档"
		if arguments.Query != "" {
			query := clip(arguments.Query, 500)
			value.Query = &query
		}
	case "read_document":
		value.Title = "阅读文档"
		value.Offset = &arguments.Offset
		if arguments.Offset > 0 {
			value.Title = "继续阅读"
		}
	}
	if result != nil {
		value.Status = gen.AnswerStepStatusDone
		var returned struct {
			Offset    *int           `json:"offset"`
			Next      *int           `json:"nextOffset"`
			Total     *int           `json:"totalCharacters"`
			Citations []gen.Citation `json:"citations"`
			Matches   []struct {
				Title string `json:"title"`
			} `json:"matches"`
			Error   string `json:"error"`
			Message string `json:"message"`
		}
		_ = json.Unmarshal([]byte(result.Content), &returned)
		if result.IsError || returned.Error != "" {
			value.Status = gen.AnswerStepStatus("failed")
			if returned.Error == "TOOL_LIMIT" {
				value.Status = gen.AnswerStepStatusSkipped
			}
			if returned.Message != "" {
				value.Detail = &returned.Message
			}
		} else {
			value.NextOffset, value.TotalCharacters = returned.Next, returned.Total
			if returned.Offset != nil {
				value.Offset = returned.Offset
			}
		}
		items := citedTitles(returned.Citations)
		for _, match := range returned.Matches {
			if match.Title != "" && len(items) < 8 {
				items = append(items, match.Title)
			}
		}
		if len(items) == 0 && result.Title != "" && call.Name == "read_document" {
			items = []string{result.Title}
		}
		if len(items) > 0 {
			value.Items = &items
		}
	}
	p.emit(ctx, "tool."+platform.HashBytes([]byte(call.ID)), value)
}
