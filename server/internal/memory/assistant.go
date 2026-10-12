package memory

import (
	"context"
	"errors"
)

type AssistantModel interface {
	Propose(context.Context, string, []Item) ([]Proposal, error)
}
type Assistant struct {
	Store   *Store
	Model   AssistantModel
	Pending *Proposals
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
	if err := a.Pending.Put(conversation, values); err != nil {
		return nil, err
	}
	return values, nil
}
func (a *Assistant) Apply(conversation string, accepted map[string]bool) error {
	values := a.Pending.Get(conversation)
	for _, proposal := range values {
		if !accepted[proposal.ID] || proposal.Change == Unchanged {
			continue
		}
		if proposal.Change == Delete {
			if err := a.Store.Delete(proposal.ID); err != nil {
				return err
			}
			continue
		}
		if proposal.After == nil {
			return errors.New("记忆提议缺少目标内容")
		}
		if err := a.Store.Save(*proposal.After); err != nil {
			return err
		}
	}
	a.Pending.Clear(conversation)
	return nil
}
