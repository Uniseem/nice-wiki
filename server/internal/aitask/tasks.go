package aitask

type Tier string

const (
	Fast      Tier = "fast"
	Smart     Tier = "smart"
	Vision    Tier = "vision"
	Embedding Tier = "embedding"
	Rerank    Tier = "rerank"
)

type Task struct {
	Name      string
	Tier      Tier
	Memory    []string
	MaxOutput int
}

var Tasks = map[string]Task{
	"qa-route": {"qa-route", Fast, []string{"project-overview"}, 1024}, "qa-answer": {"qa-answer", Smart, []string{"project-overview", "writing-style"}, 4096},
	"contextualize": {"contextualize", Fast, []string{"project-overview"}, 512}, "describe-image": {"describe-image", Vision, []string{"project-overview"}, 1024},
	"describe-attachment": {"describe-attachment", Vision, []string{"project-overview"}, 2048}, "describe-video": {"describe-video", Vision, []string{"project-overview"}, 1024},
	"translate-blocks": {"translate-blocks", Smart, []string{"project-overview", "terminology"}, 4096}, "extract-terms": {"extract-terms", Fast, []string{"terminology"}, 2048},
	"translate-review": {"translate-review", Smart, []string{"writing-style", "terminology"}, 4096}, "translate-strings": {"translate-strings", Fast, []string{"terminology"}, 2048},
	"site-summary": {"site-summary", Smart, nil, 2048}, "memory-assistant": {"memory-assistant", Smart, nil, 4096},
	"slug": {"slug", Fast, []string{"terminology"}, 64}, "seo-description": {"seo-description", Fast, []string{"project-overview"}, 256},
	"language-suggest": {"language-suggest", Fast, nil, 64}, "compact-conversation": {"compact-conversation", Smart, []string{"project-overview"}, 2048},
	"embed": {"embed", Embedding, nil, 0}, "rerank": {"rerank", Rerank, []string{"project-overview"}, 0},
}

func Lookup(name string) (Task, bool) { value, ok := Tasks[name]; return value, ok }
