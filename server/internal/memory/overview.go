package memory

import "context"

type OverviewModel interface {
	Summarize(context.Context, string) (string, error)
}

func UpdateOverview(ctx context.Context, store *Store, model OverviewModel, published string) error {
	current, err := store.Get("project-overview")
	if err != nil {
		return err
	}
	if current.Source == SourceManual {
		return nil
	}
	summary, err := model.Summarize(ctx, published)
	if err != nil {
		return err
	}
	current.Content = summary
	current.Source = SourceAutomatic
	return store.Save(current)
}
