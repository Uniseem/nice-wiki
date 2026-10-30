package content

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-billy/v5/util"
	"github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/client"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/memory"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/netguard"
	"nice-wiki/modules/content/internal/gen"
)

// GitRemote 是接口返回的 Git 同步配置（不含令牌）。
type GitRemote struct {
	URL           string `json:"url"`
	Branch        string `json:"branch"`
	Username      string `json:"username"`
	HasToken      bool   `json:"hasToken"`
	PushOnPublish bool   `json:"pushOnPublish"`
	LastPushAt    string `json:"lastPushAt"`
	LastPullAt    string `json:"lastPullAt"`
	LastStatus    string `json:"lastStatus"`
	LastMessage   string `json:"lastMessage"`
	LastCommit    string `json:"lastCommit"`
	Configured    bool   `json:"configured"`
}

// GitRemoteInput 是保存 Git 同步配置的输入；Token 为空表示不改令牌。
type GitRemoteInput struct {
	URL           string  `json:"url"`
	Branch        string  `json:"branch"`
	Username      string  `json:"username"`
	Token         *string `json:"token"`
	PushOnPublish bool    `json:"pushOnPublish"`
}

// GitSyncResult 是一次推送或拉取的结果。
type GitSyncResult struct {
	Commit  string        `json:"commit"`
	Changed bool          `json:"changed"`
	Files   int           `json:"files"`
	Message string        `json:"message"`
	Import  *ImportResult `json:"import,omitempty"`
}

var gitMu sync.Mutex
var gitInstall sync.Once

// installGitTransport 让 go-git 的 http(s) 出站也走防 SSRF 的客户端（进程内只在本模块用 go-git）。
func installGitTransport(allowLocal bool) {
	gitInstall.Do(func() {
		guarded := githttp.NewClient(remoteClient(allowLocal))
		client.InstallProtocol("https", guarded)
		client.InstallProtocol("http", guarded)
	})
}

func (d SiteData) ReadGitRemote(ctx context.Context, op OperationContext) (GitRemote, error) {
	var g GitRemote
	var hasToken bool
	err := d.db().QueryRowContext(ctx, `SELECT url,branch,username,token_cipher IS NOT NULL,push_on_publish<>0,last_push_at,last_pull_at,last_status,last_message,last_commit FROM git_remotes WHERE site_id=?`, op.Actor.SiteID).
		Scan(&g.URL, &g.Branch, &g.Username, &hasToken, &g.PushOnPublish, &g.LastPushAt, &g.LastPullAt, &g.LastStatus, &g.LastMessage, &g.LastCommit)
	if errors.Is(err, sql.ErrNoRows) {
		return GitRemote{Branch: "main"}, nil
	}
	if err != nil {
		return g, storageFailure()
	}
	g.HasToken, g.Configured = hasToken, true
	return g, nil
}

func (d SiteData) gitURL(ctx context.Context, raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "https" && u.Scheme != "http") {
		return nil, platform.NewError(platform.CodeValidation, false, "仓库地址必须是 https:// 开头的网址（不含账号口令）")
	}
	if u.Scheme == "http" && !d.AllowLocal {
		return nil, platform.NewError(platform.CodeValidation, false, "仓库地址必须是 https:// 开头的网址（不含账号口令）")
	}
	if err := netguard.CheckHost(ctx, u.Hostname(), d.AllowLocal); err != nil {
		return nil, platform.NewError(platform.CodeValidation, false, "这个地址不允许使用（私网、本机或无法解析）")
	}
	return u, nil
}

func (d SiteData) PutGitRemote(ctx context.Context, op OperationContext, in GitRemoteInput) (GitRemote, error) {
	u, err := d.gitURL(ctx, in.URL)
	if err != nil {
		return GitRemote{}, err
	}
	branch := strings.TrimSpace(in.Branch)
	if branch == "" {
		branch = "main"
	}
	if len(branch) > 100 || strings.ContainsAny(branch, " ~^:?*[\\") || strings.Contains(branch, "..") || strings.HasPrefix(branch, "-") {
		return GitRemote{}, platform.NewError(platform.CodeValidation, false, "分支名不合法")
	}
	var nonce, cipher []byte
	if in.Token != nil && *in.Token != "" {
		if d.Box == nil {
			return GitRemote{}, platform.NewError(platform.CodeDependencyUnavailable, false, "部署没有配置主密钥，不能保存令牌")
		}
		if nonce, cipher, err = d.Box.Seal([]byte(*in.Token), "git-token:"+op.Actor.SiteID); err != nil {
			return GitRemote{}, platform.NewError(platform.CodeInternal, false, "令牌加密失败")
		}
	}
	db := d.db()
	var exists int
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM git_remotes WHERE site_id=?`, op.Actor.SiteID).Scan(&exists)
	// 换了仓库或分支：上次同步的提交和指纹都不再适用。
	_, _ = db.ExecContext(ctx, `UPDATE git_remotes SET last_commit='',last_hash='' WHERE site_id=? AND (url<>? OR branch<>?)`, op.Actor.SiteID, u.String(), branch)
	push := 0
	if in.PushOnPublish {
		push = 1
	}
	switch {
	case exists == 0:
		if nonce == nil {
			return GitRemote{}, platform.NewError(platform.CodeValidation, false, "首次配置需要填写访问令牌")
		}
		_, err = db.ExecContext(ctx, `INSERT INTO git_remotes(site_id,url,branch,username,token_nonce,token_cipher,push_on_publish) VALUES(?,?,?,?,?,?,?)`, op.Actor.SiteID, u.String(), branch, strings.TrimSpace(in.Username), nonce, cipher, push)
	case nonce != nil:
		_, err = db.ExecContext(ctx, `UPDATE git_remotes SET url=?,branch=?,username=?,token_nonce=?,token_cipher=?,push_on_publish=? WHERE site_id=?`, u.String(), branch, strings.TrimSpace(in.Username), nonce, cipher, push, op.Actor.SiteID)
	default:
		_, err = db.ExecContext(ctx, `UPDATE git_remotes SET url=?,branch=?,username=?,push_on_publish=? WHERE site_id=?`, u.String(), branch, strings.TrimSpace(in.Username), push, op.Actor.SiteID)
	}
	if err != nil {
		return GitRemote{}, storageFailure()
	}
	return d.ReadGitRemote(ctx, op)
}

func (d SiteData) DeleteGitRemote(ctx context.Context, op OperationContext) error {
	if _, err := d.db().ExecContext(ctx, `DELETE FROM git_remotes WHERE site_id=?`, op.Actor.SiteID); err != nil {
		return storageFailure()
	}
	return nil
}

type gitSettings struct {
	url, branch, username, token string
	lastCommit                   string // 上次推送或拉取时仓库分支的提交
}

func (d SiteData) openGitSettings(ctx context.Context, site string) (gitSettings, error) {
	var g gitSettings
	var nonce, cipher []byte
	err := d.db().QueryRowContext(ctx, `SELECT url,branch,username,token_nonce,token_cipher,last_commit FROM git_remotes WHERE site_id=?`, site).Scan(&g.url, &g.branch, &g.username, &nonce, &cipher, &g.lastCommit)
	if errors.Is(err, sql.ErrNoRows) {
		return g, platform.NewError(platform.CodeNotFound, false, "还没有配置 Git 仓库")
	}
	if err != nil {
		return g, storageFailure()
	}
	if d.Box == nil {
		return g, platform.NewError(platform.CodeDependencyUnavailable, false, "部署没有配置主密钥")
	}
	token, err := d.Box.Open(nonce, cipher, "git-token:"+site)
	if err != nil {
		return g, platform.NewError(platform.CodeIntegrityError, false, "令牌无法解密，请重新填写")
	}
	g.token = string(token)
	if _, err := d.gitURL(ctx, g.url); err != nil {
		return g, err
	}
	return g, nil
}

func (g gitSettings) auth() *githttp.BasicAuth {
	user := g.username
	if user == "" {
		user = "git"
	}
	return &githttp.BasicAuth{Username: user, Password: g.token}
}

func gitFailure(err error, what string) error {
	text := err.Error()
	if strings.Contains(text, "authentication") || strings.Contains(text, "authorization") || strings.Contains(text, "401") || strings.Contains(text, "403") {
		return platform.NewError(platform.CodeForbidden, false, "%s失败：令牌无效或没有权限", what)
	}
	return platform.NewError(platform.CodeDependencyUnavailable, true, "%s失败：%s", what, firstN(strings.Join(strings.Fields(text), " "), 200))
}

// managedPaths 是同步会整体替换的目录/文件；仓库里的其他文件（README、许可证等）不动。
var managedPaths = []string{"docs", "blocks", "media", ExportManifest}

func (d SiteData) record(ctx context.Context, site, field, status, message, commit, hash string) {
	db := d.db()
	now := time.Now().UTC().Format(time.RFC3339)
	switch field {
	case "push":
		_, _ = db.ExecContext(ctx, `UPDATE git_remotes SET last_push_at=?,last_status=?,last_message=?,last_commit=CASE WHEN ?<>'' THEN ? ELSE last_commit END,last_hash=CASE WHEN ?<>'' THEN ? ELSE last_hash END WHERE site_id=?`, now, status, firstN(message, 300), commit, commit, hash, hash, site)
	default:
		_, _ = db.ExecContext(ctx, `UPDATE git_remotes SET last_pull_at=?,last_status=?,last_message=?,last_commit=CASE WHEN ?<>'' THEN ? ELSE last_commit END WHERE site_id=?`, now, status, firstN(message, 300), commit, commit, site)
	}
}

// siteStateHash 是整站公开内容的指纹：文档、公开版本、标题与层级变了它就变，用来判断“发布后要不要推送”。
func (d SiteData) siteStateHash(ctx context.Context, op OperationContext) (string, error) {
	docs, err := d.activeDocuments(d.Context(ctx), op)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	for _, doc := range docs {
		ref := ""
		if doc.PublishedReference != nil {
			ref = doc.PublishedReference.PublishedRevisionId
		}
		parent := ""
		if doc.ParentId != nil {
			parent = *doc.ParentId
		}
		fmt.Fprintf(h, "%s|%s|%s|%s|%s|%s\n", doc.DocumentId, doc.State, ref, doc.Title, parent, doc.OrderKey)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// PushGit 把整站已发布内容（Markdown、块 JSON、媒体）提交并推送到用户的仓库。
func (d SiteData) PushGit(ctx context.Context, op OperationContext) (GitSyncResult, error) {
	gitMu.Lock()
	defer gitMu.Unlock()
	site := op.Actor.SiteID
	settings, err := d.openGitSettings(ctx, site)
	if err != nil {
		return GitSyncResult{}, err
	}
	installGitTransport(d.AllowLocal)
	svc := ServiceOperation(site, op.DataEpoch)
	scoped := platform.WithOperationContext(ctx, svc)
	hash, _ := d.siteStateHash(scoped, svc)
	bundle, err := d.BuildExport(scoped, svc, true)
	if err != nil {
		d.record(ctx, site, "push", "failed", "导出失败："+errorText(err), "", "")
		return GitSyncResult{}, err
	}
	result, err := pushBundle(ctx, settings, bundle)
	if err != nil {
		d.record(ctx, site, "push", "failed", errorText(err), "", "")
		return GitSyncResult{}, err
	}
	d.record(ctx, site, "push", "ok", result.Message, result.Commit, hash)
	return result, nil
}

func cloneOrInit(ctx context.Context, settings gitSettings) (*git.Repository, billy.Filesystem, error) {
	fs := memfs.New()
	storage := memory.NewStorage()
	branchRef := plumbing.NewBranchReferenceName(settings.branch)
	repo, err := git.CloneContext(ctx, storage, fs, &git.CloneOptions{URL: settings.url, Auth: settings.auth(), ReferenceName: branchRef, SingleBranch: true})
	switch {
	case err == nil:
		return repo, fs, nil
	case errors.Is(err, transport.ErrEmptyRemoteRepository):
		fs, storage = memfs.New(), memory.NewStorage()
		repo, err = git.Init(storage, fs)
		if err != nil {
			return nil, nil, gitFailure(err, "初始化仓库")
		}
		if _, err = repo.CreateRemote(&gitconfig.RemoteConfig{Name: "origin", URLs: []string{settings.url}}); err != nil {
			return nil, nil, gitFailure(err, "初始化仓库")
		}
		if err = repo.Storer.SetReference(plumbing.NewSymbolicReference(plumbing.HEAD, branchRef)); err != nil {
			return nil, nil, gitFailure(err, "初始化仓库")
		}
		return repo, fs, nil
	case errors.Is(err, plumbing.ErrReferenceNotFound):
		// 仓库有内容但没有这个分支：从默认分支拉下来，再开新分支。
		fs, storage = memfs.New(), memory.NewStorage()
		repo, err = git.CloneContext(ctx, storage, fs, &git.CloneOptions{URL: settings.url, Auth: settings.auth()})
		if err != nil {
			return nil, nil, gitFailure(err, "拉取仓库")
		}
		wt, err := repo.Worktree()
		if err == nil {
			err = wt.Checkout(&git.CheckoutOptions{Branch: branchRef, Create: true})
		}
		if err != nil {
			return nil, nil, gitFailure(err, "创建分支")
		}
		return repo, fs, nil
	default:
		return nil, nil, gitFailure(err, "拉取仓库")
	}
}

func pushBundle(ctx context.Context, settings gitSettings, bundle ExportBundle) (GitSyncResult, error) {
	repo, fs, err := cloneOrInit(ctx, settings)
	if err != nil {
		return GitSyncResult{}, err
	}
	// 仓库里有站点没见过的提交（比如有人改了 Markdown）时不覆盖，先拉取。
	if head, err := repo.Head(); err == nil && settings.lastCommit != "" && head.Hash().String() != settings.lastCommit {
		return GitSyncResult{}, platform.NewError(platform.CodeConflict, false, "仓库里有还没拉取的新提交，先拉取再推送")
	}
	for _, p := range managedPaths {
		_ = util.RemoveAll(fs, p)
	}
	for _, file := range bundle.Files {
		if err := util.WriteFile(fs, file.Name, file.Data, 0o644); err != nil {
			return GitSyncResult{}, gitFailure(err, "写入仓库文件")
		}
	}
	wt, err := repo.Worktree()
	if err != nil {
		return GitSyncResult{}, gitFailure(err, "写入仓库文件")
	}
	if err := wt.AddWithOptions(&git.AddOptions{All: true}); err != nil {
		return GitSyncResult{}, gitFailure(err, "暂存文件")
	}
	status, err := wt.Status()
	if err != nil {
		return GitSyncResult{}, gitFailure(err, "比较文件")
	}
	if status.IsClean() {
		head, _ := repo.Head()
		commit := ""
		if head != nil {
			commit = head.Hash().String()
		}
		return GitSyncResult{Commit: commit, Changed: false, Files: len(bundle.Files), Message: "仓库已经是最新，没有新提交"}, nil
	}
	hash, err := wt.Commit(fmt.Sprintf("同步站点内容（%d 个文件）", len(bundle.Files)), &git.CommitOptions{Author: &object.Signature{Name: "NiceWiki", Email: "sync@nicewiki.invalid", When: time.Now()}})
	if err != nil {
		return GitSyncResult{}, gitFailure(err, "提交")
	}
	branch := settings.branch
	err = repo.PushContext(ctx, &git.PushOptions{RemoteName: "origin", Auth: settings.auth(), RefSpecs: []gitconfig.RefSpec{gitconfig.RefSpec("refs/heads/" + branch + ":refs/heads/" + branch)}})
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return GitSyncResult{}, gitFailure(err, "推送")
	}
	return GitSyncResult{Commit: hash.String(), Changed: true, Files: len(bundle.Files), Message: "已推送 " + hash.String()[:8]}, nil
}

// PullGit 从仓库拉取最新内容，生成草稿，确认后再发布。
// 本站推送出去的仓库按同步处理（见 syncPull）；其他仓库整体导入成新文档。
func (d SiteData) PullGit(ctx context.Context, op OperationContext) (GitSyncResult, error) {
	gitMu.Lock()
	defer gitMu.Unlock()
	site := op.Actor.SiteID
	settings, err := d.openGitSettings(ctx, site)
	if err != nil {
		return GitSyncResult{}, err
	}
	installGitTransport(d.AllowLocal)
	fail := func(err error) (GitSyncResult, error) {
		d.record(ctx, site, "pull", "failed", errorText(err), "", "")
		return GitSyncResult{}, err
	}
	fs := memfs.New()
	// 不做浅克隆：要和上次同步的提交比较改了哪些文件。
	repo, err := git.CloneContext(ctx, memory.NewStorage(), fs, &git.CloneOptions{URL: settings.url, Auth: settings.auth(), ReferenceName: plumbing.NewBranchReferenceName(settings.branch), SingleBranch: true})
	if errors.Is(err, transport.ErrEmptyRemoteRepository) || errors.Is(err, plumbing.ErrReferenceNotFound) {
		return fail(platform.NewError(platform.CodeNotFound, false, "仓库里还没有这个分支或没有内容"))
	}
	if err != nil {
		return fail(gitFailure(err, "拉取仓库"))
	}
	head, err := repo.Head()
	if err != nil {
		return fail(gitFailure(err, "拉取仓库"))
	}
	commit := head.Hash().String()
	if commit == settings.lastCommit {
		message := "仓库没有新的提交"
		d.record(ctx, site, "pull", "ok", message, "", "")
		return GitSyncResult{Commit: commit, Message: message}, nil
	}
	archive, count, err := zipWorktree(fs)
	if err != nil {
		return fail(err)
	}
	a, err := readArchive(archive)
	if err != nil {
		return fail(err)
	}
	var imported ImportResult
	var message string
	var manifest ExportManifestFile
	raw, ok := a.files[ExportManifest]
	if ok && json.Unmarshal(raw, &manifest) == nil && manifest.Format == ExportFormat && manifest.SiteID == site {
		var updated int
		imported, updated, err = d.syncPull(ctx, op, a, manifest, changedFiles(ctx, repo, settings.lastCommit, head.Hash()))
		message = fmt.Sprintf("已拉取 %s：更新 %d 篇、新增 %d 篇（草稿）", firstN(commit, 8), updated, imported.Documents-updated)
	} else {
		imported, err = d.importFrom(ctx, op, a, ImportOptions{Format: "auto"})
		message = fmt.Sprintf("已拉取 %s：导入 %d 篇文档（草稿）", firstN(commit, 8), imported.Documents)
	}
	if err != nil {
		return fail(err)
	}
	d.record(ctx, site, "pull", "ok", message, commit, "")
	return GitSyncResult{Commit: commit, Changed: imported.Documents > 0, Files: count, Message: message, Import: &imported}, nil
}

// changedFiles 列出 base 之后改动或新增的文件；没有 base 或它不在历史里时返回 nil，表示逐个比对全部文件。
func changedFiles(ctx context.Context, repo *git.Repository, base string, head plumbing.Hash) map[string]bool {
	if base == "" {
		return nil
	}
	from, err := repo.CommitObject(plumbing.NewHash(base))
	if err != nil {
		return nil
	}
	to, err := repo.CommitObject(head)
	if err != nil {
		return nil
	}
	fromTree, err := from.Tree()
	if err != nil {
		return nil
	}
	toTree, err := to.Tree()
	if err != nil {
		return nil
	}
	changes, err := fromTree.DiffContext(ctx, toTree)
	if err != nil {
		return nil
	}
	out := map[string]bool{}
	for _, change := range changes {
		if change.To.Name != "" {
			out[change.To.Name] = true
		}
	}
	return out
}

// syncPull 处理本站推送出去的仓库：只拉回上次同步之后在仓库里改过的 Markdown。
// 改过的文档存成它的新草稿（站内有未发布修改的不覆盖），docs/ 下新加的 Markdown 建成新的草稿文档；
// 文档结构、改名和删除仍以站点为准。返回的 Documents 是更新与新增的总数，另返回更新的篇数。
func (d SiteData) syncPull(ctx context.Context, op OperationContext, a *importArchive, manifest ExportManifestFile, changed map[string]bool) (ImportResult, int, error) {
	ctx = d.Context(ctx)
	docs, err := d.activeDocuments(ctx, op)
	if err != nil {
		return ImportResult{}, 0, err
	}
	current := map[string]gen.DocumentMetadata{}
	for _, doc := range docs {
		current[doc.DocumentId] = doc
	}
	result := ImportResult{Format: "nicewiki", Warnings: append([]string{}, a.warnings...), Failures: []string{}}
	im := &importer{d: d, op: op, svc: ServiceOperation(op.Actor.SiteID, op.DataEpoch), archive: a, result: &result, uploaded: map[string]map[string]any{}}
	files := map[string]string{}  // 文档 ID → 推送时的 Markdown 路径
	byFile := map[string]string{} // 小写 Markdown 路径 → 站内文档 ID
	dirs := map[string]string{}   // 子文档所在目录 → 站内文档 ID
	tracked := map[string]bool{}
	for _, doc := range manifest.Documents {
		files[doc.ID] = doc.File
		tracked[strings.ToLower(doc.File)] = true
		if _, ok := current[doc.ID]; ok {
			byFile[strings.ToLower(doc.File)] = doc.ID
			dirs[strings.TrimSuffix(doc.File, ".md")] = doc.ID
		}
	}
	mediaPaths := map[string]string{}
	for _, m := range manifest.Media {
		mediaPaths[fmt.Sprint(m.AssetID, ":", m.Version)] = m.File
	}
	bodies := make([][]any, len(manifest.Documents))
	for i, doc := range manifest.Documents {
		raw, _ := a.get(doc.Blocks)
		var generic struct {
			Content []any `json:"content"`
		}
		_ = json.Unmarshal(raw, &generic)
		bodies[i] = generic.Content
		// 仓库里已有的媒体文件对应站内现成的资源，Markdown 引用它们时不重复上传。
		walkAssetRefs(generic.Content, func(ref map[string]any) map[string]any {
			if file, ok := mediaPaths[fmt.Sprint(ref["assetId"], ":", ref["version"])]; ok {
				im.uploaded[file] = ref
			}
			return nil
		})
	}
	type target struct {
		id, file, title string
		old             []any // 推送时的正文：内容没变的块沿用原块 ID
	}
	var edits, fresh []target
	for i, doc := range manifest.Documents {
		if changed != nil && !changed[doc.File] {
			continue
		}
		// 按推送时的布局重新渲染块 JSON，和仓库里的 Markdown 一样就是没人改过。
		raw, ok := a.get(doc.File)
		if !ok || bytes.Equal(raw, exportMarkdown(doc, bodies[i], files, mediaPaths)) {
			continue
		}
		meta, ok := current[doc.ID]
		if !ok {
			result.Warnings = append(result.Warnings, "《"+doc.Title+"》已在站内删除，仓库里的修改没有拉回")
			continue
		}
		draft, err := ReadDraft(ctx, step(op), ReadDraftInput{DocumentId: doc.ID})
		if err != nil {
			result.Failures = append(result.Failures, "《"+meta.Title+"》读取草稿失败："+errorText(err))
			continue
		}
		if meta.PublishedReference == nil || draft.Draft.BodyHash != meta.PublishedReference.ContentHash {
			result.Warnings = append(result.Warnings, "《"+meta.Title+"》站内有未发布的修改，没有用仓库里的版本覆盖")
			continue
		}
		edits = append(edits, target{doc.ID, doc.File, meta.Title, bodies[i]})
	}
	// a.names 已排序，“docs/A.md”排在“docs/A/新.md”前面，上级文档先建好。
	for _, name := range a.names {
		if !strings.HasPrefix(name, "docs/") || !isMarkdown(name) || tracked[strings.ToLower(name)] || changed != nil && !changed[name] {
			continue
		}
		var parent *Identifier
		if id, ok := dirs[path.Dir(name)]; ok {
			parent = &id
		}
		title := titleOf(a, name, false)
		created, err := CreateDocument(ctx, step(op), CreateDocumentInput{ParentId: parent, Title: title})
		if err != nil {
			result.Failures = append(result.Failures, "《"+title+"》创建失败："+errorText(err))
			continue
		}
		id := created.Metadata.DocumentId
		byFile[strings.ToLower(name)] = id
		dirs[strings.TrimSuffix(name, ".md")] = id
		fresh = append(fresh, target{id: id, file: name, title: title})
	}
	for _, t := range append(edits, fresh...) {
		parsed := im.parseMarkdown(ctx, t.file, byFile)
		for _, w := range parsed.Warnings {
			result.Warnings = append(result.Warnings, "《"+t.title+"》："+w)
		}
		if parsed.Title != "" && parsed.Title != t.title {
			result.Warnings = append(result.Warnings, "《"+t.title+"》的标题改动没有同步，请在站内改名")
		}
		if err := d.saveImported(ctx, op, t.id, keepBlocks(t.old, parsed.Blocks)); err != nil {
			result.Failures = append(result.Failures, "《"+t.title+"》正文没能保存："+errorText(err))
		}
	}
	result.Documents = len(edits) + len(fresh)
	return result, len(edits), nil
}

var blockIDField = regexp.MustCompile(`"blockId":"[^"]*"`)

// keepBlocks 把解析出的块里和原正文内容相同（只差块 ID）的顶层块换回原块：Markdown 不带块 ID，
// 不换回的话没改的段落也会换新 ID，指向段落的链接锚点和批注都会失效。
func keepBlocks(old, parsed []any) []any {
	key := func(block any) string {
		raw, _ := json.Marshal(block)
		return blockIDField.ReplaceAllString(string(raw), `"blockId":""`)
	}
	same := map[string][]any{}
	for _, block := range old {
		k := key(block)
		same[k] = append(same[k], block)
	}
	for i, block := range parsed {
		if list := same[key(block)]; len(list) > 0 {
			parsed[i] = list[0]
			same[key(block)] = list[1:]
		}
	}
	return parsed
}

// zipWorktree 把检出的仓库文件打成 zip（跳过 .git），交给统一的导入流程。
func zipWorktree(fs billy.Filesystem) ([]byte, int, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	count := 0
	var total int64
	var walk func(dir string) error
	walk = func(dir string) error {
		entries, err := fs.ReadDir(dir)
		if err != nil {
			return err
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
		for _, entry := range entries {
			name := path.Join(dir, entry.Name())
			if entry.Name() == ".git" {
				continue
			}
			if entry.IsDir() {
				if err := walk(name); err != nil {
					return err
				}
				continue
			}
			if count >= maxImportEntries || total > maxImportBytes {
				return platform.NewError(platform.CodeValidation, false, "仓库内容太大（文件超过 5000 个或总量超过 256MB）")
			}
			file, err := fs.Open(name)
			if err != nil {
				return err
			}
			raw, err := io.ReadAll(io.LimitReader(file, maxImportFile+1))
			file.Close()
			if err != nil || len(raw) > maxImportFile {
				continue
			}
			total += int64(len(raw))
			w, err := zw.Create(strings.TrimPrefix(name, "/"))
			if err != nil {
				return err
			}
			if _, err = w.Write(raw); err != nil {
				return err
			}
			count++
		}
		return nil
	}
	if err := walk("/"); err != nil && !errors.Is(err, os.ErrNotExist) {
		var failure *platform.Error
		if errors.As(err, &failure) {
			return nil, 0, err
		}
		return nil, 0, gitFailure(err, "读取仓库文件")
	}
	if err := zw.Close(); err != nil {
		return nil, 0, gitFailure(err, "打包仓库文件")
	}
	return buf.Bytes(), count, nil
}

// RunGitAutoPush 给后台循环调用：逐站点检查“发布后自动推送”，内容指纹变了才推（同一站点至少隔一分钟）。
func (d SiteData) RunGitAutoPush(ctx context.Context, sites []Identifier) {
	for _, site := range sites {
		op := ServiceOperation(site, d.Epoch)
		scoped := platform.WithOperationContext(platform.WithPortalProof(ctx, ""), op)
		var push int
		var last, lastHash, lastStatus string
		err := d.db().QueryRowContext(scoped, `SELECT push_on_publish,last_push_at,last_hash,last_status FROM git_remotes WHERE site_id=?`, site).Scan(&push, &last, &lastHash, &lastStatus)
		if err != nil || push == 0 {
			continue
		}
		wait := time.Minute
		if lastStatus == "failed" {
			wait = 10 * time.Minute
		}
		if at, err := time.Parse(time.RFC3339, last); err == nil && time.Since(at) < wait {
			continue
		}
		hash, err := d.siteStateHash(scoped, op)
		if err != nil || hash == lastHash {
			continue
		}
		_, _ = d.PushGit(scoped, op)
	}
}
