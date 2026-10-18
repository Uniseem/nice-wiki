package analytics

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/lifecycle"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// RecoverExports只续接真实prepared对象；未完成字节不作为ready，未知旧记录拒绝。
func RecoverExports(ctx context.Context, op OperationContext) error {
	rt, err := runtimeFrom(ctx, op)
	if err != nil {
		return err
	}
	rt.exportMu.Lock()
	defer rt.exportMu.Unlock()
	rows, err := rt.DB.QueryContext(ctx, `SELECT export_id,state,object_ref FROM export_runs WHERE data_epoch=? AND state<>'expired'`, rt.Epoch)
	if err != nil {
		return storageError(err)
	}
	type saved struct{ ID, State, Ref string }
	records := []saved{}
	known := map[string]bool{}
	for rows.Next() {
		var rec saved
		if err = rows.Scan(&rec.ID, &rec.State, &rec.Ref); err != nil {
			break
		}
		if !exportObjectNameValid(rec.ID) {
			err = platform.NewError(platform.CodeIntegrityError, false, "导出journal对象ID不合法")
			break
		}
		records = append(records, rec)
		known[rec.ID+".csv"] = true
		known[rec.ID+".csv.part"] = true
	}
	if err == nil {
		err = rows.Err()
	}
	_ = rows.Close()
	if err != nil {
		return storageError(err)
	}
	entries, err := os.ReadDir(filepath.Join(rt.Root, "active", "exports"))
	if err != nil {
		return storageError(err)
	}
	for _, entry := range entries {
		if !known[entry.Name()] || entry.IsDir() {
			return platform.NewError(platform.CodeIntegrityError, false, "发现未登记导出对象，拒绝猜测删除或就绪")
		}
	}
	for _, rec := range records {
		switch rec.State {
		case "prepared":
			if err = installPreparedExport(ctx, op, rt, rec.ID); err != nil {
				return err
			}
		case "preparing":
			path := filepath.Join(rt.Root, "active", "exports", rec.ID+".csv.part")
			if err = lifecycle.ValidatePrivatePath(path, "regular-file", true); err != nil {
				return err
			}
			if _, e := os.Lstat(path); e == nil {
				work, _, cancel, e := lifecycle.NewWriteContext(platform.WithOperationContext(ctx, op), rt.Window)
				if e != nil {
					return e
				}
				e = work.Err()
				if e == nil {
					e = os.Remove(path)
				}
				cancel()
				if e != nil {
					return storageError(e)
				}
				if e = syncDirectory(filepath.Dir(path), rt.Root); e != nil {
					return e
				}
			} else if !errors.Is(e, os.ErrNotExist) {
				return storageError(e)
			}
		case "ready":
			var ref platform.ObjectRef
			if json.Unmarshal([]byte(rec.Ref), &ref) != nil {
				return platform.NewError(platform.CodeIntegrityError, false, "已封存导出引用损坏")
			}
			file, e := verifyExportFile(filepath.Join(rt.Root, "active", "exports", rec.ID+".csv"), ref)
			if e != nil {
				return e
			}
			_ = file.Close()
		case "expired":
		default:
			return platform.NewError(platform.CodeUnsupportedSchema, false, "未知导出journal状态")
		}
	}
	return nil
}

// PurgeExports保留过期幂等墓碑，旧操作不会因清文件又获得新的24小时授权。
func PurgeExports(ctx context.Context, op OperationContext, in PurgeExportsInput) (PurgeExportsResult, error) {
	rt, err := runtimeFrom(ctx, op)
	if err != nil {
		return PurgeExportsResult{}, err
	}
	now, err := time.Parse(time.RFC3339Nano, in.Now)
	if err != nil {
		return PurgeExportsResult{}, platform.NewError(platform.CodeValidation, false, "清理时间不合法")
	}
	rt.exportMu.Lock()
	defer rt.exportMu.Unlock()
	rows, err := rt.DB.QueryContext(ctx, `SELECT export_id FROM export_runs WHERE expires_at<=? AND state<>'expired' AND data_epoch=? ORDER BY expires_at,export_id LIMIT 500`, now.UTC().Format(analyticsTimeLayout), rt.Epoch)
	if err != nil {
		return PurgeExportsResult{}, storageError(err)
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			break
		}
		ids = append(ids, id)
	}
	if err == nil {
		err = rows.Err()
	}
	_ = rows.Close()
	if err != nil {
		return PurgeExportsResult{}, storageError(err)
	}
	result := PurgeExportsResult{}
	for _, id := range ids {
		if !exportObjectNameValid(id) {
			return result, platform.NewError(platform.CodeIntegrityError, false, "导出journal对象ID不合法")
		}
		work, _, cancel, e := lifecycle.NewWriteContext(platform.WithOperationContext(ctx, op), rt.Window)
		if e != nil {
			return result, e
		}
		for _, suffix := range []string{".csv.part", ".csv"} {
			path := filepath.Join(rt.Root, "active", "exports", id+suffix)
			if e = lifecycle.ValidatePrivatePath(path, "regular-file", true); e != nil {
				break
			}
			if e = work.Err(); e != nil {
				break
			}
			e = os.Remove(path)
			if errors.Is(e, os.ErrNotExist) {
				e = nil
			}
			if e != nil {
				break
			}
		}
		if e == nil {
			e = syncDirectory(filepath.Join(rt.Root, "active", "exports"), rt.Root)
		}
		cancel()
		if e != nil {
			return result, storageError(e)
		}
		work, _, tx, done, e := beginWrite(ctx, op)
		if e != nil {
			return result, e
		}
		_, e = tx.ExecContext(work, `UPDATE export_runs SET state='expired',object_ref='',result_json='' WHERE export_id=? AND data_epoch=?`, id, rt.Epoch)
		if e == nil {
			_, e = advanceWatermark(work, tx, "export-retention")
		}
		if e == nil {
			e = tx.Commit()
		}
		_ = tx.Rollback()
		done()
		if e != nil {
			return result, storageError(e)
		}
		result.Deleted++
	}
	return result, nil
}

// ExportKind只读原记录，不把缺失类型默认为统计或审计。
func ExportKind(ctx context.Context, rt *Runtime, id string) (string, error) {
	var kind string
	err := rt.DB.QueryRowContext(ctx, `SELECT kind FROM export_runs WHERE export_id=?`, id).Scan(&kind)
	if errors.Is(err, sql.ErrNoRows) {
		return "", platform.NewError(platform.CodeNotFound, false, "导出不存在")
	}
	if err != nil {
		return "", storageError(err)
	}
	if kind != "pages" && kind != "usage" && kind != "audits" {
		return "", platform.NewError(platform.CodeIntegrityError, false, "导出类型未登记")
	}
	return kind, nil
}

// 保证固定名字只来自数据库绑定的服务生成ID，不拼接调用方文件路径。
func exportObjectNameValid(id string) bool {
	return strings.HasPrefix(id, "export.") && len(id) == 71 && platform.ValidHash(id[7:])
}
