package protection

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

type ReadProtectionSettingsInput struct{}
type ProtectionScheduleSettings struct {
	Timezone       string `json:"timezone"`
	KeepCount      int64  `json:"keepCount"`
	IncludeSecrets bool   `json:"includeSecrets"`
	ScheduleID     string `json:"scheduleId"`
	Revision       int64  `json:"revision"`
}
type RemoteSettings struct {
	URL              string `json:"url"`
	Branch           string `json:"branch"`
	ExpectedRevision int64  `json:"expectedRevision"`
}
type ReadProtectionSettingsResult struct {
	Schedule *ProtectionScheduleSettings `json:"schedule,omitempty"`
	Remote   *RemoteSettings             `json:"remote,omitempty"`
}

// ReadProtectionSettings 只投影真实已保存配置；未配置字段省略，不拿读取人冒充原批准人。
func ReadProtectionSettings(ctx context.Context, op OperationContext, input ReadProtectionSettingsInput) (ReadProtectionSettingsResult, error) {
	if err := requireAdministrator(op); err != nil {
		return ReadProtectionSettingsResult{}, err
	}
	store, err := Catalog(op)
	if err != nil {
		return ReadProtectionSettingsResult{}, err
	}
	tx, err := store.DB.BeginTx(ctx, nil)
	if err != nil {
		return ReadProtectionSettingsResult{}, storageError(err)
	}
	defer tx.Rollback()
	result := ReadProtectionSettingsResult{}
	var raw string
	var revision int64
	err = tx.QueryRowContext(ctx, `SELECT settings_json,revision FROM protection_settings WHERE setting_id=1`).Scan(&raw, &revision)
	if err == nil {
		var schedule ProtectionScheduleSettings
		if json.Unmarshal([]byte(raw), &schedule) != nil {
			return ReadProtectionSettingsResult{}, storageError(errors.New("settings"))
		}
		schedule.Revision = revision
		result.Schedule = &schedule
	} else if !errors.Is(err, sql.ErrNoRows) {
		return ReadProtectionSettingsResult{}, storageError(err)
	}
	var remote RemoteSettings
	err = tx.QueryRowContext(ctx, `SELECT url_without_credentials,branch,revision FROM remote_config WHERE config_id=1`).Scan(&remote.URL, &remote.Branch, &remote.ExpectedRevision)
	if err == nil {
		result.Remote = &remote
	} else if !errors.Is(err, sql.ErrNoRows) {
		return ReadProtectionSettingsResult{}, storageError(err)
	}
	return result, nil
}
