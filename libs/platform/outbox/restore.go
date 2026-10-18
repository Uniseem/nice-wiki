package outbox

import (
 "context"
 "database/sql"
 "path/filepath"
 "strings"
 "nice-wiki/libs/platform"
 "nice-wiki/libs/platform/lifecycle"
)

// PrepareRestoredOutbox只改变已认证隔离副本的活动投影；原payload/epoch/费用事实完整保留。
func PrepareRestoredOutbox(ctx context.Context,db *sql.DB)error{
 root,targetEpoch,sealHash,err:=lifecycle.SnapshotStageBinding(ctx);if err!=nil{return err};if db==nil{return platform.NewError(platform.CodeDependencyUnavailable,false,"staged数据库连接未装配")};rows,err:=db.QueryContext(ctx,"PRAGMA database_list");if err!=nil{return platform.NewError(platform.CodeInternal,true,"读取staged数据库归属失败")};found:=false;for rows.Next(){var sequence int;var name,path string;if err=rows.Scan(&sequence,&name,&path);err!=nil{_ = rows.Close();return platform.NewError(platform.CodeInternal,true,"读取staged数据库归属失败")};if name=="main"{absolute,absErr:=filepath.Abs(path);if absErr!=nil{_ = rows.Close();return platform.NewError(platform.CodeIntegrityError,false,"staged数据库路径未知")};relative,relErr:=filepath.Rel(root,absolute);if relErr!=nil||relative=="."||relative==".."||strings.HasPrefix(relative,".."+string(filepath.Separator)){_ = rows.Close();return platform.NewError(platform.CodeForbidden,false,"数据库不属于当前隔离stage")};if err=lifecycle.ValidatePrivatePath(absolute,"regular-file",false);err!=nil{_ = rows.Close();return err};found=true}};err=rows.Err();_ = rows.Close();if err!=nil||!found{return platform.NewError(platform.CodeIntegrityError,false,"staged数据库主文件归属不完整")}
 // V1旧表可缺新增控制行，标准EnsureSchema在隔离树中幂等建立，绝不raw-copy live连接。
 var tableCount int;if err=db.QueryRowContext(ctx,`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='nw_outbox'`).Scan(&tableCount);err!=nil||tableCount!=1{return platform.NewError(platform.CodeIntegrityError,false,"源数据库缺真实outbox表，不能创建空表冒充历史")};if err=EnsureSchema(ctx,db);err!=nil{return err};tx,err:=db.BeginTx(ctx,nil);if err!=nil{return platform.NewError(platform.CodeInternal,true,"建立staged outbox隔离事务失败")};defer tx.Rollback();var unknown int64;if err=tx.QueryRowContext(ctx,`SELECT COUNT(*) FROM nw_outbox WHERE state NOT IN ('pending','relayed','quarantined','discarded') OR (data_epoch=? AND state NOT IN ('relayed','discarded'))`,targetEpoch).Scan(&unknown);err!=nil{return platform.NewError(platform.CodeInternal,true,"读取staged活动事实失败")};if unknown!=0{return platform.NewError(platform.CodeIntegrityError,false,"源outbox状态未知或冒称新代活动")}
 if _,err=tx.ExecContext(ctx,`UPDATE nw_outbox_control SET sealed_hash='' WHERE id=1`);err!=nil{return platform.NewError(platform.CodeInternal,true,"打开隔离副本迁移gate失败")};if _,err=tx.ExecContext(ctx,`UPDATE nw_outbox SET state='discarded',lease_until='' WHERE data_epoch<>? AND state IN ('pending','quarantined')`,targetEpoch);err!=nil{return platform.NewError(platform.CodeInternal,true,"撤销源事件活动资格失败")};if _,err=tx.ExecContext(ctx,`UPDATE nw_outbox_control SET sealed_hash=?,last_seal_hash=? WHERE id=1`,sealHash,sealHash);err!=nil{return platform.NewError(platform.CodeInternal,true,"固定本次恢复封口失败")};if err=tx.Commit();err!=nil{return platform.NewError(platform.CodeInternal,true,"提交staged事件隔离失败")};return nil
}
