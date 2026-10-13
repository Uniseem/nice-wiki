package ingest

import (
	"context"
	"errors"
	"nice-wiki/internal/doc"
	"nice-wiki/internal/doc/nodes"
)

type Media struct {
	ID, Kind, Hash, PosterHash, Name, Extension, Title, Description string
}

func MediaItems(document doc.Document) []Media {
	result := []Media{}
	var walk func(doc.Node)
	walk = func(node doc.Node) {
		id := nodes.String(node, "id")
		switch node.Type {
		case "image":
			result = append(result, Media{ID: id, Kind: "image", Hash: nodes.String(node, "hash"), Extension: nodes.String(node, "extension"), Title: nodes.String(node, "alt"), Description: nodes.String(node, "caption")})
		case "drawing":
			hash := nodes.String(node, "sceneHash")
			if hash == "" {
				hash = nodes.String(node, "lightHash")
			}
			result = append(result, Media{ID: id, Kind: "drawing", Hash: hash, Title: nodes.String(node, "alt"), Description: nodes.String(node, "caption")})
		case "attachment":
			result = append(result, Media{ID: id, Kind: "attachment", Hash: nodes.String(node, "hash"), Extension: nodes.String(node, "extension"), Name: nodes.String(node, "name"), Description: nodes.String(node, "description")})
		case "video":
			hash := nodes.String(node, "hash")
			if hash == "" {
				hash = nodes.String(node, "posterHash")
			}
			result = append(result, Media{ID: id, Kind: "video", Hash: hash, PosterHash: nodes.String(node, "posterHash"), Name: nodes.String(node, "provider") + ":" + nodes.String(node, "providerId"), Title: nodes.String(node, "title"), Description: nodes.String(node, "description")})
		}
		for _, child := range node.Content {
			walk(child)
		}
	}
	walk(document.Content)
	return result
}

type DescriptionStore interface {
	GetDescription(context.Context, string, string) (string, bool, error)
	PutDescription(context.Context, string, string, string) error
}
type DescriptionModel interface {
	Describe(context.Context, string, string) (string, error)
}

func Describe(ctx context.Context, store DescriptionStore, model DescriptionModel, hash, kind string) (string, error) {
	if hash == "" {
		return "", errors.New("媒体缺少内容哈希")
	}
	if value, ok, err := store.GetDescription(ctx, hash, kind); err != nil {
		return "", err
	} else if ok {
		return value, nil
	}
	if model == nil {
		return "", errors.New("视觉描述模型尚未配置")
	}
	value, err := model.Describe(ctx, hash, kind)
	if err != nil {
		return "", err
	}
	if err := store.PutDescription(ctx, hash, kind, value); err != nil {
		return "", err
	}
	return value, nil
}
