package qa

import "context"

type CompactModel interface {
	Compact(context.Context, string, []Turn) (string, error)
}

func Compact(ctx context.Context, store *Conversations, model CompactModel, id string, maximum int) error {
	value, ok := store.Get(id)
	if !ok {
		return nil
	}
	size := len(value.Summary)
	for _, turn := range value.Turns {
		size += len(turn.Text)
	}
	if size < maximum || len(value.Turns) < 6 {
		return nil
	}
	cut := len(value.Turns) - 4
	summary, err := model.Compact(ctx, value.Summary, value.Turns[:cut])
	if err != nil {
		return err
	}
	store.ReplaceSummary(id, summary, value.Turns[cut:])
	return nil
}
