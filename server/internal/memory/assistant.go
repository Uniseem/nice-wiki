package memory

import (
	"context"
	"errors"
	"os"
	"reflect"
)

type AssistantModel interface {
	Propose(context.Context, string, []Item) ([]Proposal, error)
}
type Assistant struct {
	Store   *Store
	Model   AssistantModel
	Pending *Proposals
	Mapping *Mapping
}

func (a *Assistant) Ask(ctx context.Context, conversation, message string) ([]Proposal, error) {
	if a.Store == nil || a.Model == nil || a.Pending == nil {
		return nil, errors.New("记忆助手尚未配置")
	}
	items, err := a.Store.List()
	if err != nil {
		return nil, err
	}
	values, err := a.Model.Propose(ctx, message, items)
	if err != nil {
		return nil, err
	}
	current := map[string]Item{}
	for _, item := range items {
		current[item.ID] = item
	}
	for index := range values {
		proposal := &values[index]
		item, exists := current[proposal.ID]
		switch proposal.Change {
		case Add:
			if exists || proposal.After == nil || proposal.After.ID != proposal.ID {
				return nil, errors.New("新增记忆提议无效")
			}
			proposal.After.Source = SourceAssistant
			proposal.After.ReadOnly = false
			proposal.Before = nil
		case Update:
			if !exists || item.ReadOnly || proposal.After == nil || proposal.After.ID != proposal.ID {
				return nil, errors.New("更新记忆提议无效")
			}
			before := item
			proposal.Before = &before
			proposal.After.Source = SourceAssistant
			proposal.After.ReadOnly = false
		case Delete:
			if !exists || item.ReadOnly {
				return nil, errors.New("删除记忆提议无效")
			}
			before := item
			proposal.Before = &before
			proposal.After = nil
		case Unchanged:
			if exists {
				before := item
				proposal.Before = &before
			}
		default:
			return nil, errors.New("记忆提议类型无效")
		}
	}
	if err := a.Pending.Put(conversation, values); err != nil {
		return nil, err
	}
	return values, nil
}
func (a *Assistant) Apply(conversation string, accepted map[string]bool) error {
	values := a.Pending.Get(conversation)
	if len(values) == 0 {
		return errors.New("记忆提议不存在或已过期")
	}
	overrides := map[string]Item{}
	for _, proposal := range values {
		if !accepted[proposal.ID] || proposal.Change == Unchanged {
			continue
		}
		current, err := a.Store.Get(proposal.ID)
		switch proposal.Change {
		case Add:
			if !errors.Is(err, os.ErrNotExist) || proposal.After == nil {
				return errors.New("新增记忆提议已过期")
			}
			overrides[proposal.ID] = *proposal.After
		case Update, Delete:
			if err != nil || proposal.Before == nil || !reflect.DeepEqual(current, *proposal.Before) {
				return errors.New("记忆内容已改变，请重新生成提议")
			}
			if proposal.Change == Update {
				if proposal.After == nil {
					return errors.New("记忆提议缺少目标内容")
				}
				overrides[proposal.ID] = *proposal.After
			}
		}
	}
	if a.Mapping != nil {
		for _, proposal := range values {
			if !accepted[proposal.ID] || proposal.Change != Delete {
				continue
			}
			for _, ids := range a.Mapping.All() {
				for _, id := range ids {
					if id == proposal.ID {
						return errors.New("记忆仍被任务映射使用，不能删除")
					}
				}
			}
		}
		if err := ValidateMappings(a.Store, a.Mapping, overrides); err != nil {
			return err
		}
	}
	for _, proposal := range values {
		if !accepted[proposal.ID] || proposal.Change == Unchanged {
			continue
		}
		if proposal.Change == Delete {
			if err := a.Store.Delete(proposal.ID); err != nil {
				return err
			}
		} else if err := a.Store.Save(*proposal.After); err != nil {
			return err
		}
	}
	a.Pending.Clear(conversation)
	return nil
}
