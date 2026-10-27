package content

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/serviceauth"
)

// 海外网盘备份目标：Google Drive（只见本应用建的文件）、OneDrive（应用专属文件夹）、Dropbox（应用文件夹）。
// 备份目标里保存的是长期授权（加密），每次备份前经身份域换成短期访问令牌；应用密钥只在身份域。

func isDriveKind(kind string) bool {
	return kind == "gdrive" || kind == "onedrive" || kind == "dropbox"
}

// 接口地址固定在代码里。
var (
	googleAPI      = "https://www.googleapis.com/drive/v3"
	googleUpload   = "https://www.googleapis.com/upload/drive/v3"
	graphAPI       = "https://graph.microsoft.com/v1.0"
	dropboxAPI     = "https://api.dropboxapi.com/2"
	dropboxContent = "https://content.dropboxapi.com/2"
)

// driveCall 发一次带访问令牌的请求；状态码不在这里判断。
func driveCall(ctx context.Context, client *http.Client, method, target, token string, body []byte, headers map[string]string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(body))
	if err != nil {
		return nil, platform.NewError(platform.CodeValidation, false, "网盘地址不合法")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "网盘不可达")
	}
	return resp, nil
}

// driveJSON 发请求并把 JSON 响应读进 out（out 为空则只检查状态）；allowed 里的状态码返回 (code, nil)。
func driveJSON(ctx context.Context, client *http.Client, method, target, token string, body []byte, headers map[string]string, what string, out any, allowed ...int) (int, error) {
	resp, err := driveCall(ctx, client, method, target, token, body, headers)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	for _, code := range allowed {
		if resp.StatusCode == code {
			return code, nil
		}
	}
	if resp.StatusCode >= 300 {
		return resp.StatusCode, responseError(resp, what)
	}
	if out != nil {
		if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(out); err != nil {
			return resp.StatusCode, platform.NewError(platform.CodeDependencyUnavailable, true, "网盘返回的内容不能解析")
		}
	}
	return resp.StatusCode, nil
}

// ---- Google Drive ----

type gdriveStore struct {
	client *http.Client
	token  string
	folder []string
}

func newGdriveStore(client *http.Client, token, prefix string) *gdriveStore {
	var folder []string
	if prefix = cleanPrefix(prefix); prefix != "" {
		folder = strings.Split(prefix, "/")
	}
	return &gdriveStore{client: client, token: token, folder: folder}
}

func gdriveQuote(s string) string {
	return "'" + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(s) + "'"
}

func (g *gdriveStore) list(ctx context.Context, query string) ([]struct{ ID, Name string }, error) {
	var out struct {
		Files []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"files"`
	}
	target := googleAPI + "/files?" + url.Values{"q": {query}, "fields": {"files(id,name)"}, "pageSize": {"1000"}, "spaces": {"drive"}}.Encode()
	if _, err := driveJSON(ctx, g.client, http.MethodGet, target, g.token, nil, nil, "列出备份", &out); err != nil {
		return nil, err
	}
	files := make([]struct{ ID, Name string }, 0, len(out.Files))
	for _, f := range out.Files {
		files = append(files, struct{ ID, Name string }{f.ID, f.Name})
	}
	return files, nil
}

// folderID 逐级找到备份文件夹；create 为真时缺的层级现建，否则缺了就返回空串。
func (g *gdriveStore) folderID(ctx context.Context, create bool) (string, error) {
	parent := "root"
	for _, name := range g.folder {
		found, err := g.list(ctx, "mimeType='application/vnd.google-apps.folder' and name="+gdriveQuote(name)+" and "+gdriveQuote(parent)+" in parents and trashed=false")
		if err != nil {
			return "", err
		}
		switch {
		case len(found) > 0:
			parent = found[0].ID
		case !create:
			return "", nil
		default:
			body, _ := json.Marshal(map[string]any{"name": name, "mimeType": "application/vnd.google-apps.folder", "parents": []string{parent}})
			var made struct {
				ID string `json:"id"`
			}
			if _, err := driveJSON(ctx, g.client, http.MethodPost, googleAPI+"/files?fields=id", g.token, body, map[string]string{"Content-Type": "application/json"}, "创建备份文件夹", &made); err != nil {
				return "", err
			}
			parent = made.ID
		}
	}
	return parent, nil
}

// Put 用可续传上传（整个文件一次发完）：先带元数据开一个会话，再把内容发到会话地址。
func (g *gdriveStore) Put(ctx context.Context, name string, data []byte) error {
	folder, err := g.folderID(ctx, true)
	if err != nil {
		return err
	}
	// Drive 允许同名文件并存；这里按“覆盖”处理，和其他存储一致，免得清理旧备份时误删同名的新文件。
	if err := g.Delete(ctx, name); err != nil {
		return err
	}
	meta, _ := json.Marshal(map[string]any{"name": name, "parents": []string{folder}})
	resp, err := driveCall(ctx, g.client, http.MethodPost, googleUpload+"/files?uploadType=resumable", g.token, meta, map[string]string{"Content-Type": "application/json; charset=UTF-8"})
	if err != nil {
		return err
	}
	resp.Body.Close()
	session := resp.Header.Get("Location")
	if resp.StatusCode >= 300 || session == "" {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "开启上传失败（%d）", resp.StatusCode)
	}
	_, err = driveJSON(ctx, g.client, http.MethodPut, session, g.token, data, map[string]string{"Content-Type": "application/octet-stream"}, "写入备份", nil)
	return err
}

func (g *gdriveStore) List(ctx context.Context, prefix string) ([]string, error) {
	folder, err := g.folderID(ctx, false)
	if err != nil || folder == "" {
		return nil, err
	}
	found, err := g.list(ctx, gdriveQuote(folder)+" in parents and name contains "+gdriveQuote(prefix)+" and trashed=false")
	if err != nil {
		return nil, err
	}
	var names []string
	for _, f := range found {
		names = append(names, f.Name)
	}
	return names, nil
}

func (g *gdriveStore) Delete(ctx context.Context, name string) error {
	folder, err := g.folderID(ctx, false)
	if err != nil || folder == "" {
		return err
	}
	found, err := g.list(ctx, gdriveQuote(folder)+" in parents and name="+gdriveQuote(name)+" and trashed=false")
	if err != nil {
		return err
	}
	for _, f := range found {
		if _, err := driveJSON(ctx, g.client, http.MethodDelete, googleAPI+"/files/"+url.PathEscape(f.ID), g.token, nil, nil, "删除旧备份", nil, http.StatusNotFound); err != nil {
			return err
		}
	}
	return nil
}

// ---- OneDrive（Microsoft Graph 应用文件夹） ----

type onedriveStore struct {
	client *http.Client
	token  string
	prefix string
}

func newOnedriveStore(client *http.Client, token, prefix string) *onedriveStore {
	return &onedriveStore{client: client, token: token, prefix: cleanPrefix(prefix)}
}

// itemPath 是应用文件夹下某个路径的 Graph 地址（每段单独转义）。
func (o *onedriveStore) itemPath(name string) string {
	var parts []string
	for _, segment := range strings.Split(joinKey(o.prefix, name), "/") {
		if segment != "" {
			parts = append(parts, url.PathEscape(segment))
		}
	}
	if len(parts) == 0 {
		return graphAPI + "/me/drive/special/approot"
	}
	return graphAPI + "/me/drive/special/approot:/" + strings.Join(parts, "/") + ":"
}

// onedriveChunk 是上传分片大小，必须是 320KiB 的整数倍。
var onedriveChunk = 32 * 320 * 1024

func (o *onedriveStore) Put(ctx context.Context, name string, data []byte) error {
	var session struct {
		UploadURL string `json:"uploadUrl"`
	}
	body := []byte(`{"item":{"@microsoft.graph.conflictBehavior":"replace"}}`)
	if _, err := driveJSON(ctx, o.client, http.MethodPost, o.itemPath(name)+"/createUploadSession", o.token, body, map[string]string{"Content-Type": "application/json"}, "开启上传", &session); err != nil || session.UploadURL == "" {
		if err == nil {
			err = platform.NewError(platform.CodeDependencyUnavailable, true, "开启上传没有返回上传地址")
		}
		return err
	}
	total := len(data)
	for start := 0; start < total; start += onedriveChunk {
		end := min(start+onedriveChunk, total)
		// 上传地址自带凭证，不能再带 Authorization。
		if _, err := driveJSON(ctx, o.client, http.MethodPut, session.UploadURL, "", data[start:end],
			map[string]string{"Content-Range": fmt.Sprintf("bytes %d-%d/%d", start, end-1, total), "Content-Type": "application/octet-stream"}, "写入备份", nil, http.StatusAccepted); err != nil {
			return err
		}
	}
	return nil
}

func (o *onedriveStore) List(ctx context.Context, prefix string) ([]string, error) {
	var out struct {
		Value []struct {
			Name string `json:"name"`
		} `json:"value"`
	}
	code, err := driveJSON(ctx, o.client, http.MethodGet, o.itemPath("")+"/children?$select=name&$top=1000", o.token, nil, nil, "列出备份", &out, http.StatusNotFound)
	if err != nil || code == http.StatusNotFound {
		return nil, err
	}
	var names []string
	for _, item := range out.Value {
		if strings.HasPrefix(item.Name, prefix) {
			names = append(names, item.Name)
		}
	}
	return names, nil
}

func (o *onedriveStore) Delete(ctx context.Context, name string) error {
	_, err := driveJSON(ctx, o.client, http.MethodDelete, o.itemPath(name), o.token, nil, nil, "删除旧备份", nil, http.StatusNotFound)
	return err
}

// ---- Dropbox（应用文件夹） ----

type dropboxStore struct {
	client *http.Client
	token  string
	prefix string
}

func newDropboxStore(client *http.Client, token, prefix string) *dropboxStore {
	return &dropboxStore{client: client, token: token, prefix: cleanPrefix(prefix)}
}

// dropboxArg 把参数写成头里能放的 JSON：非 ASCII 字符一律转成 \u 转义。
func dropboxArg(v any) string {
	raw, _ := json.Marshal(v)
	var out strings.Builder
	for _, r := range string(raw) {
		switch {
		case r < 0x80:
			out.WriteRune(r)
		case r > 0xFFFF:
			r -= 0x10000
			fmt.Fprintf(&out, `\u%04x\u%04x`, 0xD800+(r>>10), 0xDC00+(r&0x3FF))
		default:
			fmt.Fprintf(&out, `\u%04x`, r)
		}
	}
	return out.String()
}

func (d *dropboxStore) path(name string) string {
	if p := strings.Trim(joinKey(d.prefix, name), "/"); p != "" {
		return "/" + p
	}
	return ""
}

// 单次上传的上限是 150MB，更大的文件走上传会话，每片 64MiB。
var dropboxChunk = 64 << 20

func (d *dropboxStore) upload(ctx context.Context, endpoint string, arg any, data []byte, out any) error {
	_, err := driveJSON(ctx, d.client, http.MethodPost, dropboxContent+endpoint, d.token, data,
		map[string]string{"Dropbox-API-Arg": dropboxArg(arg), "Content-Type": "application/octet-stream"}, "写入备份", out)
	return err
}

func (d *dropboxStore) Put(ctx context.Context, name string, data []byte) error {
	commit := map[string]any{"path": d.path(name), "mode": "overwrite", "mute": true}
	if len(data) <= dropboxChunk {
		return d.upload(ctx, "/files/upload", commit, data, nil)
	}
	var started struct {
		SessionID string `json:"session_id"`
	}
	if err := d.upload(ctx, "/files/upload_session/start", map[string]any{"close": false}, data[:dropboxChunk], &started); err != nil {
		return err
	}
	offset := dropboxChunk
	for len(data)-offset > dropboxChunk {
		cursor := map[string]any{"session_id": started.SessionID, "offset": offset}
		if err := d.upload(ctx, "/files/upload_session/append_v2", map[string]any{"cursor": cursor, "close": false}, data[offset:offset+dropboxChunk], nil); err != nil {
			return err
		}
		offset += dropboxChunk
	}
	cursor := map[string]any{"session_id": started.SessionID, "offset": offset}
	return d.upload(ctx, "/files/upload_session/finish", map[string]any{"cursor": cursor, "commit": commit}, data[offset:], nil)
}

func (d *dropboxStore) rpc(ctx context.Context, endpoint string, arg any, what string, out any) (int, error) {
	body, _ := json.Marshal(arg)
	return driveJSON(ctx, d.client, http.MethodPost, dropboxAPI+endpoint, d.token, body, map[string]string{"Content-Type": "application/json"}, what, out, http.StatusConflict)
}

func (d *dropboxStore) List(ctx context.Context, prefix string) ([]string, error) {
	var out struct {
		Entries []struct {
			Tag  string `json:".tag"`
			Name string `json:"name"`
		} `json:"entries"`
	}
	// 文件夹还不存在时服务端回 409，当作空。
	code, err := d.rpc(ctx, "/files/list_folder", map[string]any{"path": d.path(""), "limit": 2000}, "列出备份", &out)
	if err != nil || code == http.StatusConflict {
		return nil, err
	}
	var names []string
	for _, entry := range out.Entries {
		if entry.Tag == "file" && strings.HasPrefix(entry.Name, prefix) {
			names = append(names, entry.Name)
		}
	}
	return names, nil
}

func (d *dropboxStore) Delete(ctx context.Context, name string) error {
	_, err := d.rpc(ctx, "/files/delete_v2", map[string]any{"path": d.path(name)}, "删除旧备份", nil)
	return err
}

// ---- 内容域调身份域：换短期访问令牌 ----

// DriveGateway 是内容域调用身份域的最小客户端：用保存的长期授权换短期访问令牌。
type DriveGateway struct {
	base   string
	client *http.Client
}

func NewDriveGateway(baseURL, issuer string, key platform.SigningKey) *DriveGateway {
	return &DriveGateway{base: strings.TrimRight(baseURL, "/"), client: &http.Client{
		Timeout:       30 * time.Second,
		Transport:     serviceauth.SigningTransport(issuer, "identity", key, nil),
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}
}

// Refresh 返回访问令牌；服务商换发了新的长期授权时第二个返回值非空，调用方要换掉保存的那份。
func (g *DriveGateway) Refresh(ctx context.Context, op OperationContext, provider, refreshToken string) (string, string, error) {
	body, _ := json.Marshal(map[string]string{"provider": provider, "refreshToken": refreshToken})
	req, err := http.NewRequestWithContext(platform.WithPortalProof(platform.WithOperationContext(ctx, op), ""), http.MethodPost, g.base+"/v1/drive-tokens:refresh", bytes.NewReader(body))
	if err != nil {
		return "", "", platform.NewError(platform.CodeValidation, false, "身份服务请求构造失败")
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := g.client.Do(req)
	if err != nil {
		return "", "", platform.NewError(platform.CodeDependencyUnavailable, true, "身份服务不可达")
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 16<<10))
	if resp.StatusCode >= 300 {
		var problem struct {
			Detail string `json:"detail"`
		}
		_ = json.Unmarshal(raw, &problem)
		if problem.Detail == "" {
			problem.Detail = "换取网盘访问令牌失败（" + strconv.Itoa(resp.StatusCode) + "）"
		}
		return "", "", platform.NewError(platform.CodeDependencyUnavailable, true, "%s", problem.Detail)
	}
	var out struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
	}
	if json.Unmarshal(raw, &out) != nil || out.AccessToken == "" {
		return "", "", platform.NewError(platform.CodeIntegrityError, false, "身份服务返回的令牌格式不对")
	}
	return out.AccessToken, out.RefreshToken, nil
}
