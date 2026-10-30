package content

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strings"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/content/internal/mdconv"
)

// ImportOptions 是导入请求的选项。
type ImportOptions struct {
	Format   string // auto | nicewiki | markdown | gitbook | notion
	ParentID string // 导入到这篇文档下面；空表示站点根
}

// ImportResult 是一次导入的结果。导入的文档都是草稿，确认后由用户自己发布。
type ImportResult struct {
	Format    string   `json:"format"`
	Documents int      `json:"documents"`
	Images    int      `json:"images"`
	Warnings  []string `json:"warnings"`
	Failures  []string `json:"failures"`
}

const (
	maxImportEntries = 5000
	maxImportBytes   = 256 << 20
	maxImportFile    = 64 << 20
	maxImportImages  = 500
)

// importArchive 是读进内存的压缩包内容，路径已规范化（斜杠、无前导目录、无 ..）。
type importArchive struct {
	files    map[string][]byte
	lower    map[string]string
	names    []string
	warnings []string
}

func cleanArchiveName(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	name = path.Clean("/" + name)[1:]
	return name
}

func readArchive(data []byte) (*importArchive, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, platform.NewError(platform.CodeValidation, false, "不是有效的压缩包")
	}
	if len(zr.File) > maxImportEntries {
		return nil, platform.NewError(platform.CodeValidation, false, "压缩包里的文件太多（上限 5000 个）")
	}
	a := &importArchive{files: map[string][]byte{}, lower: map[string]string{}}
	var total int64
	for _, f := range zr.File {
		name := cleanArchiveName(f.Name)
		if f.FileInfo().IsDir() || name == "" || strings.HasPrefix(name, "__MACOSX/") || strings.HasSuffix(name, ".DS_Store") || strings.HasPrefix(name, ".git/") || strings.Contains(name, "/.git/") {
			continue
		}
		if f.UncompressedSize64 > maxImportFile {
			a.warnings = append(a.warnings, "文件过大已跳过："+name)
			continue
		}
		total += int64(f.UncompressedSize64)
		if total > maxImportBytes {
			return nil, platform.NewError(platform.CodeValidation, false, "压缩包解开后超过 256MB")
		}
		rc, err := f.Open()
		if err != nil {
			continue
		}
		raw, err := io.ReadAll(io.LimitReader(rc, maxImportFile+1))
		rc.Close()
		if err != nil || len(raw) > maxImportFile {
			continue
		}
		a.files[name] = raw
	}
	// GitHub 下载的压缩包外面包一层目录：所有文件共用同一个顶层目录时去掉它。
	prefix := ""
	for name := range a.files {
		top, _, found := strings.Cut(name, "/")
		if !found {
			prefix = ""
			break
		}
		if prefix == "" {
			prefix = top + "/"
		} else if prefix != top+"/" {
			prefix = ""
			break
		}
	}
	if prefix != "" {
		stripped := make(map[string][]byte, len(a.files))
		for name, raw := range a.files {
			stripped[strings.TrimPrefix(name, prefix)] = raw
		}
		a.files = stripped
	}
	for name := range a.files {
		a.names = append(a.names, name)
		a.lower[strings.ToLower(name)] = name
	}
	sort.Strings(a.names)
	return a, nil
}

func (a *importArchive) get(name string) ([]byte, bool) {
	if raw, ok := a.files[name]; ok {
		return raw, true
	}
	if real, ok := a.lower[strings.ToLower(name)]; ok {
		return a.files[real], true
	}
	return nil, false
}

var notionSuffix = regexp.MustCompile(`\s+[0-9a-f]{32}$`)
var notionFile = regexp.MustCompile(`\s[0-9a-f]{32}\.md$`)
var numberPrefix = regexp.MustCompile(`^[0-9]{1,3}[-_.\s]+`)

func detectFormat(a *importArchive) string {
	if _, ok := a.files[ExportManifest]; ok {
		return "nicewiki"
	}
	if _, ok := a.lower["summary.md"]; ok {
		return "gitbook"
	}
	for _, name := range a.names {
		if notionFile.MatchString(name) {
			return "notion"
		}
	}
	return "markdown"
}

// planDoc 是导入计划里的一篇文档（按创建顺序排列，父在子之前）。
type planDoc struct {
	Title  string
	Parent int    // 计划里的下标，-1 表示站点根
	File   string // 压缩包里的 Markdown 路径；空表示文件夹文档
	OldID  string // 自家格式里的原文档 ID
	Blocks []byte // 自家格式里的块 JSON
	ID     string // 创建后的文档 ID
	failed bool
}

func humanize(name string, stripHex bool) string {
	name = strings.TrimSuffix(strings.TrimSuffix(name, ".md"), ".MD")
	if stripHex {
		name = notionSuffix.ReplaceAllString(name, "")
	}
	name = numberPrefix.ReplaceAllString(name, "")
	name = strings.TrimSpace(strings.NewReplacer("-", " ", "_", " ").Replace(name))
	if name == "" {
		return "未命名"
	}
	return name
}

func clipTitle(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return "未命名"
	}
	if runes := []rune(title); len(runes) > 300 {
		return string(runes[:300])
	}
	return title
}

// titleOf 取 Markdown 文件的标题：第一个一级标题，其次 YAML 头，最后用文件名。
func titleOf(a *importArchive, file string, notion bool) string {
	raw, _ := a.get(file)
	if title := mdconv.Parse(raw, mdconv.Options{}).Title; title != "" {
		return clipTitle(title)
	}
	base := path.Base(file)
	if lower := strings.ToLower(strings.TrimSuffix(base, path.Ext(base))); (lower == "readme" || lower == "index") && path.Dir(file) != "." {
		base = path.Base(path.Dir(file))
	}
	return clipTitle(humanize(base, notion))
}

func isMarkdown(name string) bool { return strings.HasSuffix(strings.ToLower(name), ".md") }

// planMarkdownTree 按目录结构生成计划（普通 Markdown 目录与 Notion 导出共用）：
// README/index 是所在目录的页面；与目录同名的 .md 是该目录的页面（Notion 风格）；其余目录补一篇文件夹文档。
func planMarkdownTree(a *importArchive, notion bool) []planDoc {
	var pages []string
	for _, name := range a.names {
		if isMarkdown(name) {
			pages = append(pages, name)
		}
	}
	dirOf := func(p string) string {
		d := path.Dir(p)
		if d == "." {
			return ""
		}
		return d
	}
	hasDir := map[string]bool{}
	for _, p := range pages {
		for d := dirOf(p); d != ""; d = dirOf(d) {
			hasDir[d] = true
		}
	}
	type node struct {
		title string
		file  string
		dir   string // 这篇文档“拥有”的目录（子文档放在里面）
		key   string
	}
	var nodes []*node
	owner := map[string]*node{}
	pageNode := map[string]*node{}
	for _, p := range pages {
		base := strings.TrimSuffix(path.Base(p), path.Ext(p))
		lower := strings.ToLower(base)
		n := &node{file: p, title: titleOf(a, p, notion)}
		switch {
		case (lower == "readme" || lower == "index") && dirOf(p) != "":
			n.dir = dirOf(p)
			n.key = n.dir + "\x00"
		case dirOf(p) == "" && (lower == "readme" || lower == "index"):
			n.key = "\x00" + p
		default:
			own := path.Join(dirOf(p), base)
			if dirOf(p) == "" {
				own = base
			}
			if hasDir[own] {
				n.dir = own
			}
			n.key = strings.ToLower(p)
		}
		if n.dir != "" {
			if _, taken := owner[n.dir]; !taken {
				owner[n.dir] = n
			} else {
				n.dir = ""
			}
		}
		pageNode[p] = n
		nodes = append(nodes, n)
	}
	// 没有页面认领的目录补一篇文件夹文档，浅的先建。
	var dirs []string
	for d := range hasDir {
		if _, ok := owner[d]; !ok {
			dirs = append(dirs, d)
		}
	}
	sort.Slice(dirs, func(i, j int) bool {
		return strings.Count(dirs[i], "/") < strings.Count(dirs[j], "/") || (strings.Count(dirs[i], "/") == strings.Count(dirs[j], "/") && dirs[i] < dirs[j])
	})
	for _, d := range dirs {
		n := &node{title: clipTitle(humanize(path.Base(d), notion)), dir: d, key: d + "\x00"}
		owner[d] = n
		nodes = append(nodes, n)
	}
	// 父子关系：文档所在目录的认领者；认领目录 D 的页面，它自己的位置是 D 的上一级。
	children := map[*node][]*node{}
	var roots []*node
	for _, n := range nodes {
		var holder string
		switch {
		case n.dir != "" && (n.file == "" || n.key == n.dir+"\x00"):
			holder = dirOf(n.dir)
		case n.file != "":
			holder = dirOf(n.file)
		}
		if parent, ok := owner[holder]; ok && holder != "" && parent != n {
			children[parent] = append(children[parent], n)
		} else {
			roots = append(roots, n)
		}
	}
	less := func(list []*node) {
		sort.SliceStable(list, func(i, j int) bool { return list[i].key < list[j].key })
	}
	less(roots)
	var plan []planDoc
	var emit func(n *node, parent int)
	emit = func(n *node, parent int) {
		plan = append(plan, planDoc{Title: n.title, Parent: parent, File: n.file})
		me := len(plan) - 1
		less(children[n])
		for _, child := range children[n] {
			emit(child, me)
		}
	}
	for _, n := range roots {
		emit(n, -1)
	}
	return plan
}

var summaryLine = regexp.MustCompile(`^(\s*)[*+-]\s+\[([^\]]*)\]\(([^)]*)\)`)

// planGitbook 按 SUMMARY.md 的层级生成计划；没写进目录的 .md 追加在根上。
func planGitbook(a *importArchive) []planDoc {
	raw, _ := a.get("SUMMARY.md")
	var plan []planDoc
	type level struct {
		indent int
		index  int
	}
	var stack []level
	listed := map[string]bool{}
	for _, line := range strings.Split(strings.ReplaceAll(string(raw), "\t", "    "), "\n") {
		m := summaryLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		indent := len(m[1])
		for len(stack) > 0 && stack[len(stack)-1].indent >= indent {
			stack = stack[:len(stack)-1]
		}
		parent := -1
		if len(stack) > 0 {
			parent = stack[len(stack)-1].index
		}
		file := ""
		if target := strings.TrimSpace(m[3]); target != "" && isMarkdown(strings.SplitN(target, "#", 2)[0]) && !strings.Contains(target, "://") {
			if decoded, err := url.PathUnescape(strings.SplitN(target, "#", 2)[0]); err == nil {
				file = cleanArchiveName(decoded)
			}
			if _, ok := a.get(file); !ok {
				file = ""
			}
		}
		title := strings.TrimSpace(m[2])
		if title == "" && file != "" {
			title = titleOf(a, file, false)
		}
		plan = append(plan, planDoc{Title: clipTitle(title), Parent: parent, File: file})
		if file != "" {
			listed[strings.ToLower(file)] = true
		}
		stack = append(stack, level{indent: indent, index: len(plan) - 1})
	}
	for _, name := range a.names {
		if isMarkdown(name) && strings.ToLower(name) != "summary.md" && !listed[strings.ToLower(name)] {
			plan = append(plan, planDoc{Title: titleOf(a, name, false), Parent: -1, File: name})
		}
	}
	return plan
}

func planNicewiki(a *importArchive) ([]planDoc, ExportManifestFile, error) {
	raw, _ := a.get(ExportManifest)
	var manifest ExportManifestFile
	if json.Unmarshal(raw, &manifest) != nil || manifest.Format != ExportFormat || manifest.Version != 1 {
		return nil, manifest, platform.NewError(platform.CodeValidation, false, "导出清单格式不对或版本不支持")
	}
	index := map[string]int{}
	var plan []planDoc
	for _, doc := range manifest.Documents {
		parent := -1
		if p, ok := index[doc.ParentID]; ok {
			parent = p
		}
		blocks, _ := a.get(doc.Blocks)
		index[doc.ID] = len(plan)
		plan = append(plan, planDoc{Title: clipTitle(doc.Title), Parent: parent, OldID: doc.ID, Blocks: blocks, File: doc.File})
	}
	return plan, manifest, nil
}

// importer 保存一次导入过程中的状态。
type importer struct {
	d        SiteData
	op       OperationContext
	svc      OperationContext
	archive  *importArchive
	result   *ImportResult
	uploaded map[string]map[string]any // 压缩包路径 → 新资源引用
	images   int
}

// uploadFile 把压缩包里的文件交给媒体服务；同一路径只传一次。
func (im *importer) uploadFile(ctx context.Context, file string) (map[string]any, bool) {
	if ref, ok := im.uploaded[file]; ok {
		return ref, ref != nil
	}
	im.uploaded[file] = nil
	data, ok := im.archive.get(file)
	if !ok || im.d.Media == nil {
		return nil, false
	}
	if im.images >= maxImportImages {
		im.result.Warnings = append(im.result.Warnings, "图片数量超过上限，后面的没有导入")
		return nil, false
	}
	ref, err := im.d.Media.Import(ctx, im.svc, path.Base(file), data, im.op.Actor.SubjectID)
	if err != nil {
		im.result.Warnings = append(im.result.Warnings, "文件没能导入："+file)
		return nil, false
	}
	im.images++
	im.result.Images++
	im.uploaded[file] = ref
	return ref, true
}

// resolveFile 把 Markdown 里相对某个文件的引用解析成压缩包里的路径。
func resolveFile(a *importArchive, from, target string) (string, bool) {
	if i := strings.IndexAny(target, "#?"); i >= 0 {
		target = target[:i]
	}
	if decoded, err := url.PathUnescape(target); err == nil {
		target = decoded
	}
	if target == "" || strings.Contains(target, "://") {
		return "", false
	}
	var joined string
	if strings.HasPrefix(target, "/") {
		joined = cleanArchiveName(target)
	} else {
		joined = cleanArchiveName(path.Join(path.Dir(from), target))
	}
	if _, ok := a.get(joined); ok {
		if real, ok := a.lower[strings.ToLower(joined)]; ok {
			return real, true
		}
		return joined, true
	}
	return joined, false
}

// parseMarkdown 把压缩包里的一篇 Markdown 转成块：站内链接按 byFile（小写路径 → 文档 ID）解析，图片交给媒体服务。
func (im *importer) parseMarkdown(ctx context.Context, file string, byFile map[string]string) mdconv.Result {
	a := im.archive
	raw, _ := a.get(file)
	return mdconv.Parse(raw, mdconv.Options{
		Link: func(target string) (string, bool) {
			resolved, _ := resolveFile(a, file, target)
			for _, candidate := range []string{resolved, resolved + ".md", resolved + "/README.md", resolved + "/readme.md", resolved + "/index.md"} {
				if id, ok := byFile[strings.ToLower(candidate)]; ok {
					return id, true
				}
			}
			return "", false
		},
		Image: func(src string) (map[string]any, bool) {
			resolved, ok := resolveFile(a, file, src)
			if !ok {
				return nil, false
			}
			return im.uploadFile(ctx, resolved)
		},
	})
}

// ImportSite 把压缩包导入成当前站点的草稿文档。
func (d SiteData) ImportSite(ctx context.Context, op OperationContext, data []byte, opt ImportOptions) (ImportResult, error) {
	a, err := readArchive(data)
	if err != nil {
		return ImportResult{}, err
	}
	return d.importFrom(ctx, op, a, opt)
}

func (d SiteData) importFrom(ctx context.Context, op OperationContext, a *importArchive, opt ImportOptions) (ImportResult, error) {
	var err error
	format := opt.Format
	if format == "" || format == "auto" {
		format = detectFormat(a)
	}
	result := ImportResult{Format: format, Warnings: append([]string{}, a.warnings...), Failures: []string{}}
	var plan []planDoc
	var manifest ExportManifestFile
	switch format {
	case "nicewiki":
		plan, manifest, err = planNicewiki(a)
		if err != nil {
			return result, err
		}
	case "gitbook":
		plan = planGitbook(a)
	case "notion":
		plan = planMarkdownTree(a, true)
	default:
		result.Format = "markdown"
		plan = planMarkdownTree(a, false)
	}
	if len(plan) == 0 {
		return result, platform.NewError(platform.CodeValidation, false, "压缩包里没有可导入的文档")
	}
	if len(plan) > 2000 {
		return result, platform.NewError(platform.CodeValidation, false, "一次最多导入 2000 篇文档")
	}
	ctx = d.Context(ctx)
	im := &importer{d: d, op: op, svc: ServiceOperation(op.Actor.SiteID, op.DataEpoch), archive: a, result: &result, uploaded: map[string]map[string]any{}}
	// 先把文档建好拿到 ID：正文里的站内链接要指向它们。
	for i := range plan {
		var parent *Identifier
		switch {
		case plan[i].Parent >= 0:
			if plan[plan[i].Parent].failed {
				plan[i].failed = true
				result.Failures = append(result.Failures, "《"+plan[i].Title+"》的上级文档没有建成，已跳过")
				continue
			}
			id := plan[plan[i].Parent].ID
			parent = &id
		case opt.ParentID != "":
			id := opt.ParentID
			parent = &id
		}
		created, err := CreateDocument(ctx, step(op), CreateDocumentInput{ParentId: parent, Title: plan[i].Title})
		if err != nil {
			plan[i].failed = true
			result.Failures = append(result.Failures, "《"+plan[i].Title+"》创建失败："+errorText(err))
			continue
		}
		plan[i].ID = created.Metadata.DocumentId
		result.Documents++
	}
	docIDs := map[string]string{} // 自家格式：旧文档 ID → 新文档 ID
	byFile := map[string]string{} // Markdown 路径 → 新文档 ID
	for _, doc := range plan {
		if doc.failed {
			continue
		}
		if doc.OldID != "" {
			docIDs[doc.OldID] = doc.ID
		}
		if doc.File != "" {
			byFile[strings.ToLower(doc.File)] = doc.ID
		}
	}
	blockMap := map[string]string{}
	bodies := make([][]any, len(plan))
	assetByOld := map[string]map[string]any{}
	for i := range plan {
		if plan[i].failed {
			continue
		}
		switch {
		case format == "nicewiki":
			var parsed struct {
				Content []any `json:"content"`
			}
			if json.Unmarshal(plan[i].Blocks, &parsed) != nil {
				result.Failures = append(result.Failures, "《"+plan[i].Title+"》的正文文件损坏，已留空")
				continue
			}
			regenerateIDs(parsed.Content, blockMap)
			bodies[i] = parsed.Content
		case plan[i].File != "":
			parsed := im.parseMarkdown(ctx, plan[i].File, byFile)
			for _, w := range parsed.Warnings {
				result.Warnings = append(result.Warnings, "《"+plan[i].Title+"》："+w)
			}
			bodies[i] = parsed.Blocks
		}
	}
	if format == "nicewiki" {
		mediaByOld := map[string]ExportMedia{}
		for _, m := range manifest.Media {
			mediaByOld[m.AssetID] = m
		}
		for i := range bodies {
			if bodies[i] == nil {
				continue
			}
			bodies[i] = remapLinks(bodies[i], docIDs, blockMap).([]any)
			walkAssetRefs(bodies[i], func(ref map[string]any) map[string]any {
				old := fmt.Sprint(ref["assetId"])
				if fresh, ok := assetByOld[old]; ok {
					return fresh
				}
				m, ok := mediaByOld[old]
				if !ok {
					return nil
				}
				fresh, ok := im.uploadFile(ctx, m.File)
				if ok {
					assetByOld[old] = fresh
					return fresh
				}
				return nil
			})
		}
	}
	for i := range plan {
		if plan[i].failed || bodies[i] == nil {
			continue
		}
		if err := d.saveImported(ctx, op, plan[i].ID, bodies[i]); err != nil {
			result.Failures = append(result.Failures, "《"+plan[i].Title+"》正文没能保存（已建为空草稿）："+errorText(err))
		}
	}
	return result, nil
}

func errorText(err error) string {
	var failure *platform.Error
	if errors.As(err, &failure) {
		return failure.Message
	}
	return err.Error()
}

// saveImported 给新建的草稿写入正文：取编辑锁、保存、释放。
func (d SiteData) saveImported(ctx context.Context, op OperationContext, documentID string, blocks []any) error {
	raw, err := json.Marshal(map[string]any{"schemaVersion": 1, "type": "doc", "content": blocks})
	if err != nil {
		return err
	}
	var body DocumentBody
	if err := json.Unmarshal(raw, &body); err != nil {
		return platform.NewError(platform.CodeValidation, false, "正文结构不合法")
	}
	draft, err := ReadDraft(ctx, step(op), ReadDraftInput{DocumentId: documentID})
	if err != nil {
		return err
	}
	lock, err := AcquireEditLock(ctx, step(op), AcquireEditLockInput{DocumentId: documentID, TabId: platform.NewIdentifier()})
	if err != nil {
		return err
	}
	_, saveErr := SaveDraft(ctx, step(op), SaveDraftInput{DocumentId: documentID, Body: body, ExpectedDraftVersion: draft.Draft.DraftVersion, ExpectedDraftRevisionId: draft.Draft.RevisionId,
		LeaseId: lock.Lease.LeaseId, FencingToken: lock.Lease.FencingToken, ServerBootId: lock.Lease.ServerBootId})
	_, _ = ReleaseEditLock(ctx, step(op), ReleaseEditLockInput{DocumentId: documentID, LeaseId: lock.Lease.LeaseId, ServerBootId: lock.Lease.ServerBootId})
	return saveErr
}
