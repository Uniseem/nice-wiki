package content

import (
	"archive/zip"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/content/internal/gen"
	"nice-wiki/modules/content/internal/mdconv"
)

// 站点导出格式（同时是备份格式和 Git 仓库里的目录结构）：
//
//	nicewiki-export.json   清单：文档树、状态、块文件与媒体文件的位置
//	docs/**.md             每篇文档的 Markdown（给人读，Git 里直接可见）
//	blocks/<文档ID>.json   正文块 JSON（权威，导入时优先用它）
//	media/<资源ID>-<文件名> 引用到的媒体原文件
const (
	ExportFormat   = "nicewiki-site"
	ExportManifest = "nicewiki-export.json"
)

type ExportDoc struct {
	ID          string `json:"id"`
	ParentID    string `json:"parentId"`
	Title       string `json:"title"`
	State       string `json:"state"`
	PathSegment string `json:"pathSegment"`
	File        string `json:"file"`
	Blocks      string `json:"blocks"`
}

type ExportMedia struct {
	AssetID      string `json:"assetId"`
	Version      int    `json:"version"`
	SHA256       string `json:"sha256"`
	MediaType    string `json:"mediaType"`
	ByteSize     int    `json:"byteSize"`
	OriginalName string `json:"originalName"`
	File         string `json:"file"`
}

type ExportManifestFile struct {
	Format     string        `json:"format"`
	Version    int           `json:"version"`
	ExportedAt string        `json:"exportedAt"`
	SiteID     string        `json:"siteId"`
	Documents  []ExportDoc   `json:"documents"`
	Media      []ExportMedia `json:"media"`
}

// exportFile 是导出里的一个文件。
type exportFile struct {
	Name string
	Data []byte
}

// ExportBundle 是整站导出的全部文件（已按固定顺序排列），Zip 和 Git 同步共用。
type ExportBundle struct {
	Manifest ExportManifestFile
	Files    []exportFile
	Warnings []string
}

// activeDocuments 读出整站有效（草稿或已发布）的文档，按树前序排列。
func (d SiteData) activeDocuments(ctx context.Context, op OperationContext) ([]gen.DocumentMetadata, error) {
	tx, err := beginTreeRead(ctx, op, d.Tree)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	_, documents, err := readTreeSnapshot(ctx, tx, op, d.Tree)
	if err != nil {
		return nil, err
	}
	return treePreorder(documents, false), nil
}

// safeName 把标题变成适合当文件名的片段。
func safeName(title string) string {
	var out []rune
	lastDash := false
	for _, r := range strings.TrimSpace(title) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '.':
			out = append(out, r)
			lastDash = false
		default:
			if !lastDash && len(out) > 0 {
				out = append(out, '-')
				lastDash = true
			}
		}
	}
	name := strings.Trim(string(out), "-.")
	if runes := []rune(name); len(runes) > 60 {
		name = string(runes[:60])
	}
	return name
}

// layoutPaths 给每篇文档定 Markdown 文件路径：子文档放在“父文档同名目录”里，同级重名加序号。
func layoutPaths(docs []gen.DocumentMetadata) map[string]string {
	files := map[string]string{}
	dirs := map[string]string{}
	used := map[string]map[string]bool{}
	for _, doc := range docs {
		parentDir := "docs"
		if doc.ParentId != nil {
			if dir, ok := dirs[*doc.ParentId]; ok {
				parentDir = dir
			}
		}
		name := safeName(doc.Title)
		if name == "" {
			name = "doc-" + doc.DocumentId[:6]
		}
		if used[parentDir] == nil {
			used[parentDir] = map[string]bool{}
		}
		unique := name
		for n := 2; used[parentDir][strings.ToLower(unique)]; n++ {
			unique = fmt.Sprintf("%s-%d", name, n)
		}
		used[parentDir][strings.ToLower(unique)] = true
		files[doc.DocumentId] = parentDir + "/" + unique + ".md"
		dirs[doc.DocumentId] = parentDir + "/" + unique
	}
	return files
}

func relativePath(from, to string) string {
	rel, err := filepath.Rel(path.Dir(from), to)
	if err != nil {
		return to
	}
	return filepath.ToSlash(rel)
}

// readExportBody 读一篇文档的正文：published 为真时读当前公开的版本（未发布的文档返回 nil），否则读草稿最新版。
func (d SiteData) readExportBody(ctx context.Context, op OperationContext, doc gen.DocumentMetadata, published bool) (*ReadRevisionBodyResult, error) {
	storage := d.Drafts.Store.ConfigureRevisionStorage(RevisionStorageConfig{})
	if published {
		if doc.State != "published" {
			return nil, nil
		}
		tx, err := d.Tree.Reference.Store.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
		if err != nil {
			return nil, contentStorageError()
		}
		current, stored, err := loadVisiblePublication(ctx, tx, op, d.Tree, doc.DocumentId)
		tx.Rollback()
		if err != nil || current.PublishedReference == nil {
			return nil, nil
		}
		body, err := ReadRevisionBody(WithRevisionFileStore(ctx, storage), op, ReadRevisionBodyInput{DocumentID: doc.DocumentId, RevisionID: stored.draftRevisionID, ExpectedHash: &current.PublishedReference.ContentHash})
		return &body, err
	}
	_, draft, err := d.Drafts.Store.readDraftSnapshot(ctx, op, doc.DocumentId)
	if err != nil {
		return nil, err
	}
	body, err := ReadRevisionBody(WithRevisionFileStore(ctx, storage), op, ReadRevisionBodyInput{DocumentID: doc.DocumentId, RevisionID: draft.RevisionId, ExpectedHash: &draft.BodyHash})
	return &body, err
}

// BuildExport 读取整站文档与引用的媒体，组织成导出文件集；published 为真时只导出已发布文档的公开版本（Git 同步用）。
func (d SiteData) BuildExport(ctx context.Context, op OperationContext, published bool) (ExportBundle, error) {
	ctx = d.Context(ctx)
	docs, err := d.activeDocuments(ctx, op)
	if err != nil {
		return ExportBundle{}, err
	}
	if published {
		kept := docs[:0:0]
		for _, doc := range docs {
			if doc.State == "published" {
				kept = append(kept, doc)
			}
		}
		// 上级没发布时挂到最近的已发布祖先下。
		byID := map[string]gen.DocumentMetadata{}
		for _, doc := range docs {
			byID[doc.DocumentId] = doc
		}
		for i := range kept {
			parent := kept[i].ParentId
			for parent != nil && byID[*parent].State != "published" {
				parent = byID[*parent].ParentId
			}
			kept[i].ParentId = parent
		}
		docs = kept
	}
	files := layoutPaths(docs)
	bundle := ExportBundle{Manifest: ExportManifestFile{Format: ExportFormat, Version: 1, ExportedAt: time.Now().UTC().Format(time.RFC3339), SiteID: op.Actor.SiteID, Documents: []ExportDoc{}, Media: []ExportMedia{}}}
	bodies := map[string][]any{}
	assets := map[string]map[string]any{}
	order := []string{}
	for _, doc := range docs {
		body, err := d.readExportBody(ctx, op, doc, published)
		if err != nil || body == nil {
			bundle.Warnings = append(bundle.Warnings, "《"+doc.Title+"》正文读取失败，已跳过")
			continue
		}
		raw, _ := json.Marshal(body.Body)
		var generic struct {
			Content []any `json:"content"`
		}
		_ = json.Unmarshal(raw, &generic)
		bodies[doc.DocumentId] = generic.Content
		walkAssetRefs(generic.Content, func(ref map[string]any) map[string]any {
			key := fmt.Sprint(ref["assetId"], ":", ref["version"])
			if _, ok := assets[key]; !ok {
				assets[key] = ref
				order = append(order, key)
			}
			return nil
		})
		parent := ""
		if doc.ParentId != nil {
			parent = *doc.ParentId
		}
		slug := ""
		if doc.PathSegment != nil {
			slug = *doc.PathSegment
		}
		bundle.Manifest.Documents = append(bundle.Manifest.Documents, ExportDoc{ID: doc.DocumentId, ParentID: parent, Title: doc.Title, State: string(doc.State), PathSegment: slug, File: files[doc.DocumentId], Blocks: "blocks/" + doc.DocumentId + ".json"})
		bundle.Files = append(bundle.Files, exportFile{Name: "blocks/" + doc.DocumentId + ".json", Data: raw})
	}
	mediaPaths := map[string]string{}
	if d.Media != nil {
		svc := ServiceOperation(op.Actor.SiteID, op.DataEpoch)
		for _, key := range order {
			ref := assets[key]
			id := fmt.Sprint(ref["assetId"])
			version, _ := ref["version"].(float64)
			info, err := d.Media.ReadAsset(ctx, svc, id)
			if err != nil {
				bundle.Warnings = append(bundle.Warnings, "资源 "+id+" 的信息读取失败，已跳过")
				continue
			}
			data, err := d.Media.ReadBytes(ctx, svc, id, int(version))
			if err != nil {
				bundle.Warnings = append(bundle.Warnings, "资源 "+id+" 的文件读取失败，已跳过")
				continue
			}
			name := safeName(strings.TrimSuffix(info.OriginalName, path.Ext(info.OriginalName)))
			if name == "" {
				name = "file"
			}
			file := "media/" + id + "-" + name + strings.ToLower(path.Ext(info.OriginalName))
			mediaPaths[key] = file
			bundle.Manifest.Media = append(bundle.Manifest.Media, ExportMedia{AssetID: id, Version: int(version), SHA256: fmt.Sprint(ref["sha256"]), MediaType: info.MediaType, ByteSize: len(data), OriginalName: info.OriginalName, File: file})
			bundle.Files = append(bundle.Files, exportFile{Name: file, Data: data})
		}
	}
	// Markdown：站内链接指向对方文件的相对路径，图片指向媒体文件。
	for _, doc := range bundle.Manifest.Documents {
		blocks := bodies[doc.ID]
		md := mdconv.Render(blocks, mdconv.RenderOptions{
			Link: func(id string) (string, bool) {
				if target, ok := files[id]; ok {
					return relativePath(doc.File, target), true
				}
				return "", false
			},
			Asset: func(ref map[string]any) (string, bool) {
				if file, ok := mediaPaths[fmt.Sprint(ref["assetId"], ":", ref["version"])]; ok {
					return relativePath(doc.File, file), true
				}
				return "", false
			},
		})
		bundle.Files = append(bundle.Files, exportFile{Name: doc.File, Data: []byte("# " + doc.Title + "\n\n" + md)})
	}
	manifest, _ := json.MarshalIndent(bundle.Manifest, "", "  ")
	bundle.Files = append([]exportFile{{Name: ExportManifest, Data: manifest}}, bundle.Files...)
	sort.SliceStable(bundle.Files[1:], func(i, j int) bool { return bundle.Files[1+i].Name < bundle.Files[1+j].Name })
	return bundle, nil
}

// WriteZip 把导出文件集写成 zip。
func (b ExportBundle) WriteZip(w io.Writer) error {
	zw := zip.NewWriter(w)
	for _, file := range b.Files {
		entry, err := zw.CreateHeader(&zip.FileHeader{Name: file.Name, Method: zip.Deflate, Modified: time.Now()})
		if err != nil {
			return platform.NewError(platform.CodeInternal, false, "写导出压缩包失败")
		}
		if _, err = entry.Write(file.Data); err != nil {
			return platform.NewError(platform.CodeInternal, false, "写导出压缩包失败")
		}
	}
	if err := zw.Close(); err != nil {
		return platform.NewError(platform.CodeInternal, false, "写导出压缩包失败")
	}
	return nil
}
