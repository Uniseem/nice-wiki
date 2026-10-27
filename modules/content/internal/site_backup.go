package content

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/netguard"
)

// BackupTarget 是接口返回的备份目标（不含密钥）。
type BackupTarget struct {
	TargetID    string `json:"targetId"`
	Kind        string `json:"kind"`
	Name        string `json:"name"`
	Endpoint    string `json:"endpoint"`
	Bucket      string `json:"bucket"`
	Region      string `json:"region"`
	PathPrefix  string `json:"pathPrefix"`
	Username    string `json:"username"`
	HasSecret   bool   `json:"hasSecret"`
	Schedule    string `json:"schedule"`
	Keep        int    `json:"keep"`
	LastRunAt   string `json:"lastRunAt"`
	LastStatus  string `json:"lastStatus"`
	LastMessage string `json:"lastMessage"`
}

// BackupTargetInput 是新建或修改备份目标的输入；Secret 为空表示不改密钥。
type BackupTargetInput struct {
	Kind       string  `json:"kind"`
	Name       string  `json:"name"`
	Endpoint   string  `json:"endpoint"`
	Bucket     string  `json:"bucket"`
	Region     string  `json:"region"`
	PathPrefix string  `json:"pathPrefix"`
	Username   string  `json:"username"`
	Secret     *string `json:"secret"`
	Schedule   string  `json:"schedule"`
	Keep       int     `json:"keep"`
}

// BackupRun 是一次备份的记录。
type BackupRun struct {
	RunID      string `json:"runId"`
	TargetID   string `json:"targetId"`
	StartedAt  string `json:"startedAt"`
	FinishedAt string `json:"finishedAt"`
	Status     string `json:"status"`
	ObjectName string `json:"objectName"`
	Bytes      int64  `json:"bytes"`
	Message    string `json:"message"`
}

var backupRunMu sync.Mutex

func (d SiteData) db() *sql.DB { return d.Drafts.Store.db }

const targetColumns = `target_id,kind,name,endpoint,bucket,region,path_prefix,username,secret_cipher IS NOT NULL,schedule,keep,last_run_at,last_status,last_message`

func scanTarget(row interface{ Scan(...any) error }) (BackupTarget, error) {
	var t BackupTarget
	err := row.Scan(&t.TargetID, &t.Kind, &t.Name, &t.Endpoint, &t.Bucket, &t.Region, &t.PathPrefix, &t.Username, &t.HasSecret, &t.Schedule, &t.Keep, &t.LastRunAt, &t.LastStatus, &t.LastMessage)
	return t, err
}

func storageFailure() error {
	return platform.NewError(platform.CodeDependencyUnavailable, true, "备份配置存储不可用")
}

func (d SiteData) ListBackupTargets(ctx context.Context, op OperationContext) ([]BackupTarget, error) {
	rows, err := d.db().QueryContext(ctx, `SELECT `+targetColumns+` FROM backup_targets WHERE site_id=? ORDER BY created_at`, op.Actor.SiteID)
	if err != nil {
		return nil, storageFailure()
	}
	defer rows.Close()
	out := []BackupTarget{}
	for rows.Next() {
		t, err := scanTarget(rows)
		if err != nil {
			return nil, storageFailure()
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (d SiteData) validateEndpoint(ctx context.Context, in BackupTargetInput) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(in.Endpoint))
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "https" && u.Scheme != "http") {
		return nil, platform.NewError(platform.CodeValidation, false, "地址必须是 https:// 开头的网址")
	}
	if u.Scheme == "http" && !d.AllowLocal {
		return nil, platform.NewError(platform.CodeValidation, false, "地址必须是 https:// 开头的网址")
	}
	if err := netguard.CheckHost(ctx, u.Hostname(), d.AllowLocal); err != nil {
		return nil, platform.NewError(platform.CodeValidation, false, "这个地址不允许使用（私网、本机或无法解析）")
	}
	return u, nil
}

// PutBackupTarget 新建（targetID 为空）或修改备份目标。
func (d SiteData) PutBackupTarget(ctx context.Context, op OperationContext, targetID string, in BackupTargetInput) (BackupTarget, error) {
	if in.Kind != "s3" && in.Kind != "webdav" && !isDriveKind(in.Kind) {
		return BackupTarget{}, platform.NewError(platform.CodeValidation, false, "目标类型只支持 s3、webdav、gdrive、onedrive、dropbox")
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" || len([]rune(in.Name)) > 80 {
		return BackupTarget{}, platform.NewError(platform.CodeValidation, false, "名称不能为空，且不超过 80 个字")
	}
	if in.Keep < 1 || in.Keep > 100 {
		return BackupTarget{}, platform.NewError(platform.CodeValidation, false, "保留份数需在 1 至 100 之间")
	}
	if in.Schedule != "off" && in.Schedule != "daily" && in.Schedule != "weekly" {
		return BackupTarget{}, platform.NewError(platform.CodeValidation, false, "计划只支持 off、daily、weekly")
	}
	if in.Kind == "s3" && (strings.TrimSpace(in.Bucket) == "" || strings.TrimSpace(in.Username) == "") {
		return BackupTarget{}, platform.NewError(platform.CodeValidation, false, "S3 需要填桶名和访问密钥 ID")
	}
	// 网盘目标没有地址、桶、账号和手填密钥：长期授权由身份域在授权回调后交来。
	endpointURL := ""
	if isDriveKind(in.Kind) {
		in.Secret, in.Bucket, in.Username = nil, "", ""
	} else {
		endpoint, err := d.validateEndpoint(ctx, in)
		if err != nil {
			return BackupTarget{}, err
		}
		endpointURL = endpoint.String()
	}
	var err error
	region := strings.TrimSpace(in.Region)
	if in.Kind == "s3" && region == "" {
		region = "us-east-1"
	}
	var nonce, cipher []byte
	if in.Secret != nil && *in.Secret != "" {
		if d.Box == nil {
			return BackupTarget{}, platform.NewError(platform.CodeDependencyUnavailable, false, "部署没有配置主密钥，不能保存备份凭据")
		}
		id := targetID
		if id == "" {
			id = platform.NewIdentifier()
			targetID = id
		}
		if nonce, cipher, err = d.Box.Seal([]byte(*in.Secret), "backup-target:"+id); err != nil {
			return BackupTarget{}, platform.NewError(platform.CodeInternal, false, "凭据加密失败")
		}
	}
	db := d.db()
	var oldKind string
	exists := targetID != "" && db.QueryRowContext(ctx, `SELECT kind FROM backup_targets WHERE site_id=? AND target_id=?`, op.Actor.SiteID, targetID).Scan(&oldKind) == nil
	if exists && oldKind != in.Kind && (isDriveKind(oldKind) || isDriveKind(in.Kind)) {
		return BackupTarget{}, platform.NewError(platform.CodeValidation, false, "网盘备份目标的类型不能修改，请新建一个")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	switch {
	case !exists:
		if nonce == nil && !isDriveKind(in.Kind) {
			return BackupTarget{}, platform.NewError(platform.CodeValidation, false, "新建备份目标需要填写密钥或密码")
		}
		if targetID == "" {
			targetID = platform.NewIdentifier()
		}
		if _, err = db.ExecContext(ctx, `INSERT INTO backup_targets(target_id,site_id,kind,name,endpoint,bucket,region,path_prefix,username,secret_nonce,secret_cipher,schedule,keep,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			targetID, op.Actor.SiteID, in.Kind, in.Name, endpointURL, strings.TrimSpace(in.Bucket), region, cleanPrefix(in.PathPrefix), strings.TrimSpace(in.Username), nonce, cipher, in.Schedule, in.Keep, now); err != nil {
			return BackupTarget{}, storageFailure()
		}
	default:
		if nonce != nil {
			_, err = db.ExecContext(ctx, `UPDATE backup_targets SET kind=?,name=?,endpoint=?,bucket=?,region=?,path_prefix=?,username=?,secret_nonce=?,secret_cipher=?,schedule=?,keep=? WHERE site_id=? AND target_id=?`,
				in.Kind, in.Name, endpointURL, strings.TrimSpace(in.Bucket), region, cleanPrefix(in.PathPrefix), strings.TrimSpace(in.Username), nonce, cipher, in.Schedule, in.Keep, op.Actor.SiteID, targetID)
		} else {
			_, err = db.ExecContext(ctx, `UPDATE backup_targets SET kind=?,name=?,endpoint=?,bucket=?,region=?,path_prefix=?,username=?,schedule=?,keep=? WHERE site_id=? AND target_id=?`,
				in.Kind, in.Name, endpointURL, strings.TrimSpace(in.Bucket), region, cleanPrefix(in.PathPrefix), strings.TrimSpace(in.Username), in.Schedule, in.Keep, op.Actor.SiteID, targetID)
		}
		if err != nil {
			return BackupTarget{}, storageFailure()
		}
	}
	return scanTarget(db.QueryRowContext(ctx, `SELECT `+targetColumns+` FROM backup_targets WHERE site_id=? AND target_id=?`, op.Actor.SiteID, targetID))
}

func (d SiteData) DeleteBackupTarget(ctx context.Context, op OperationContext, targetID string) error {
	if _, err := d.db().ExecContext(ctx, `DELETE FROM backup_targets WHERE site_id=? AND target_id=?`, op.Actor.SiteID, targetID); err != nil {
		return storageFailure()
	}
	return nil
}

func (d SiteData) ListBackupRuns(ctx context.Context, op OperationContext) ([]BackupRun, error) {
	rows, err := d.db().QueryContext(ctx, `SELECT run_id,target_id,started_at,finished_at,status,object_name,bytes,message FROM backup_runs WHERE site_id=? ORDER BY started_at DESC LIMIT 50`, op.Actor.SiteID)
	if err != nil {
		return nil, storageFailure()
	}
	defer rows.Close()
	out := []BackupRun{}
	for rows.Next() {
		var r BackupRun
		if err := rows.Scan(&r.RunID, &r.TargetID, &r.StartedAt, &r.FinishedAt, &r.Status, &r.ObjectName, &r.Bytes, &r.Message); err != nil {
			return nil, storageFailure()
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// openRemote 读出目标配置与解密后的凭据，建出对应的远程存储。
func (d SiteData) openRemote(ctx context.Context, site, targetID string) (remoteStore, BackupTarget, error) {
	var t BackupTarget
	var nonce, cipher []byte
	err := d.db().QueryRowContext(ctx, `SELECT target_id,kind,name,endpoint,bucket,region,path_prefix,username,schedule,keep,secret_nonce,secret_cipher FROM backup_targets WHERE site_id=? AND target_id=?`, site, targetID).
		Scan(&t.TargetID, &t.Kind, &t.Name, &t.Endpoint, &t.Bucket, &t.Region, &t.PathPrefix, &t.Username, &t.Schedule, &t.Keep, &nonce, &cipher)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, t, platform.NewError(platform.CodeNotFound, false, "备份目标不存在")
	}
	if err != nil {
		return nil, t, storageFailure()
	}
	if d.Box == nil {
		return nil, t, platform.NewError(platform.CodeDependencyUnavailable, false, "部署没有配置主密钥")
	}
	if len(cipher) == 0 {
		return nil, t, platform.NewError(platform.CodeValidation, false, "这个备份目标还没有授权网盘，请先点“授权”")
	}
	secret, err := d.Box.Open(nonce, cipher, "backup-target:"+targetID)
	if err != nil {
		return nil, t, platform.NewError(platform.CodeIntegrityError, false, "备份凭据无法解密，请重新填写")
	}
	client := remoteClient(d.AllowLocal)
	if isDriveKind(t.Kind) {
		store, err := d.openDrive(ctx, client, site, t, string(secret))
		return store, t, err
	}
	endpoint, err := d.validateEndpoint(ctx, BackupTargetInput{Endpoint: t.Endpoint})
	if err != nil {
		return nil, t, err
	}
	switch t.Kind {
	case "s3":
		return newS3Store(client, endpoint, t.Bucket, t.Region, t.PathPrefix, t.Username, string(secret)), t, nil
	default:
		return newDavStore(client, endpoint, t.Username, string(secret), t.PathPrefix), t, nil
	}
}

func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

// RunBackup 导出整站并写到备份目标，再按保留份数清理旧备份；结果记入备份记录。
func (d SiteData) RunBackup(ctx context.Context, op OperationContext, targetID string) (BackupRun, error) {
	backupRunMu.Lock()
	defer backupRunMu.Unlock()
	site := op.Actor.SiteID
	run := BackupRun{RunID: platform.NewIdentifier(), TargetID: targetID, StartedAt: time.Now().UTC().Format(time.RFC3339), Status: "running"}
	store, target, err := d.openRemote(ctx, site, targetID)
	if err != nil {
		return run, err
	}
	record := func(status, object string, size int64, message string) BackupRun {
		run.Status, run.ObjectName, run.Bytes, run.Message = status, object, size, firstN(message, 300)
		run.FinishedAt = time.Now().UTC().Format(time.RFC3339)
		db := d.db()
		_, _ = db.ExecContext(ctx, `INSERT INTO backup_runs(run_id,site_id,target_id,started_at,finished_at,status,object_name,bytes,message) VALUES(?,?,?,?,?,?,?,?,?)`, run.RunID, site, targetID, run.StartedAt, run.FinishedAt, status, object, size, run.Message)
		_, _ = db.ExecContext(ctx, `UPDATE backup_targets SET last_run_at=?,last_status=?,last_message=? WHERE site_id=? AND target_id=?`, run.StartedAt, status, run.Message, site, targetID)
		// 记录只留最近 200 条。
		_, _ = db.ExecContext(ctx, `DELETE FROM backup_runs WHERE site_id=? AND run_id NOT IN (SELECT run_id FROM backup_runs WHERE site_id=? ORDER BY started_at DESC LIMIT 200)`, site, site)
		return run
	}
	svc := ServiceOperation(site, op.DataEpoch)
	bundle, err := d.BuildExport(platform.WithOperationContext(ctx, svc), svc, false)
	if err != nil {
		return record("failed", "", 0, "导出失败："+errorText(err)), err
	}
	var buf bytes.Buffer
	if err := bundle.WriteZip(&buf); err != nil {
		return record("failed", "", 0, "打包失败"), err
	}
	object := fmt.Sprintf("nicewiki-%s-%s.zip", shortID(site), time.Now().UTC().Format("20060102-150405"))
	if err := store.Put(ctx, object, buf.Bytes()); err != nil {
		return record("failed", object, int64(buf.Len()), errorText(err)), err
	}
	message := "已备份"
	if len(bundle.Warnings) > 0 {
		message = fmt.Sprintf("已备份（%d 条提示：%s）", len(bundle.Warnings), bundle.Warnings[0])
	}
	// 保留最近 keep 份：名字里的时间戳可以直接排序。
	if names, err := store.List(ctx, "nicewiki-"+shortID(site)+"-"); err == nil {
		sort.Strings(names)
		for len(names) > target.Keep {
			if err := store.Delete(ctx, names[0]); err != nil {
				message += "；旧备份清理失败：" + errorText(err)
				break
			}
			names = names[1:]
		}
	}
	return record("ok", object, int64(buf.Len()), message), nil
}

// dueBackups 列出该站点已到期的自动备份目标：每天一次、每周一次；失败后一小时再试。
func (d SiteData) dueBackups(ctx context.Context, site string, now time.Time) []string {
	rows, err := d.db().QueryContext(ctx, `SELECT target_id,schedule,last_run_at,last_status FROM backup_targets WHERE site_id=? AND schedule<>'off'`, site)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var due []string
	for rows.Next() {
		var id, schedule, last, status string
		if rows.Scan(&id, &schedule, &last, &status) != nil {
			continue
		}
		interval := 24 * time.Hour
		if schedule == "weekly" {
			interval = 7 * 24 * time.Hour
		}
		if status == "failed" {
			interval = time.Hour
		}
		if at, err := time.Parse(time.RFC3339, last); err != nil || now.Sub(at) >= interval {
			due = append(due, id)
		}
	}
	return due
}

// RunScheduledBackups 给后台循环调用：逐站点跑到期的自动备份。
func (d SiteData) RunScheduledBackups(ctx context.Context, sites []Identifier) {
	for _, site := range sites {
		op := ServiceOperation(site, d.Epoch)
		scoped := platform.WithOperationContext(platform.WithPortalProof(ctx, ""), op)
		for _, id := range d.dueBackups(scoped, site, time.Now().UTC()) {
			_, _ = d.RunBackup(scoped, op, id)
		}
	}
}

// openDrive 经身份域把长期授权换成短期访问令牌，建出对应网盘的存储；服务商轮换了长期授权就换掉保存的那份。
func (d SiteData) openDrive(ctx context.Context, client *http.Client, site string, t BackupTarget, refresh string) (remoteStore, error) {
	if d.Drive == nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, false, "部署没有配置身份服务地址，不能使用网盘备份")
	}
	access, rotated, err := d.Drive.Refresh(ctx, ServiceOperation(site, d.Epoch), t.Kind, refresh)
	if err != nil {
		return nil, err
	}
	if rotated != "" {
		if err := d.saveTargetSecret(ctx, site, t.TargetID, rotated); err != nil {
			return nil, err
		}
	}
	switch t.Kind {
	case "gdrive":
		return newGdriveStore(client, access, t.PathPrefix), nil
	case "onedrive":
		return newOnedriveStore(client, access, t.PathPrefix), nil
	default:
		return newDropboxStore(client, access, t.PathPrefix), nil
	}
}

func (d SiteData) saveTargetSecret(ctx context.Context, site, targetID, secret string) error {
	if d.Box == nil {
		return platform.NewError(platform.CodeDependencyUnavailable, false, "部署没有配置主密钥，不能保存备份凭据")
	}
	nonce, cipher, err := d.Box.Seal([]byte(secret), "backup-target:"+targetID)
	if err != nil {
		return platform.NewError(platform.CodeInternal, false, "凭据加密失败")
	}
	result, err := d.db().ExecContext(ctx, `UPDATE backup_targets SET secret_nonce=?,secret_cipher=? WHERE site_id=? AND target_id=?`, nonce, cipher, site, targetID)
	if err != nil {
		return storageFailure()
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return platform.NewError(platform.CodeNotFound, false, "备份目标不存在")
	}
	return nil
}

// StoreDriveToken 接身份域交来的长期授权，加密存进指定的网盘备份目标（类型要和授权的网盘一致）。
func (d SiteData) StoreDriveToken(ctx context.Context, op OperationContext, targetID, provider, refreshToken string) error {
	var kind string
	err := d.db().QueryRowContext(ctx, `SELECT kind FROM backup_targets WHERE site_id=? AND target_id=?`, op.Actor.SiteID, targetID).Scan(&kind)
	if errors.Is(err, sql.ErrNoRows) {
		return platform.NewError(platform.CodeNotFound, false, "备份目标不存在，请先保存目标再授权")
	}
	if err != nil {
		return storageFailure()
	}
	if kind != provider {
		return platform.NewError(platform.CodeValidation, false, "备份目标的类型和授权的网盘不一致")
	}
	return d.saveTargetSecret(ctx, op.Actor.SiteID, targetID, refreshToken)
}
