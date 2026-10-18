package protection

import (
 "context"
 "encoding/json"
 "encoding/hex"
 "errors"
 "os"
 "path/filepath"
 "nice-wiki/libs/platform"
 "nice-wiki/libs/platform/lifecycle"
 "nice-wiki/libs/platform/outbox"
 "nice-wiki/modules/data-protection/internal/gen"
)

type RecoverProtectionJournalInput struct{StartupDataEpoch string `json:"startupDataEpoch"`}
type RecoverProtectionJournalResult struct{Recovered int `json:"recovered"`;RequiresMaintenance bool `json:"requiresMaintenance"`;ControlReady bool `json:"controlReady"`}
type pendingJournal struct{ID,Kind,State,Payload string}

// RecoverProtectionJournal 只重放已持久的完整文件事实；未知或不完整事实绝不宣称恢复成功。
func RecoverProtectionJournal(ctx context.Context,op OperationContext,in RecoverProtectionJournalInput)(RecoverProtectionJournalResult,error){
 store,err:=Catalog(op);if err!=nil{return RecoverProtectionJournalResult{},err};if in.StartupDataEpoch!=store.Epoch{return RecoverProtectionJournalResult{},platform.NewError(platform.CodeVersionMismatch,false,"journal恢复启动代次不一致")};store.CryptoMu.Lock();defer store.CryptoMu.Unlock();result:=RecoverProtectionJournalResult{ControlReady:true}
 rows,err:=store.Control.QueryContext(ctx,`SELECT operation_id,kind,state,payload_json FROM protection_journal WHERE state NOT IN('complete','needs-retry') ORDER BY updated_at,operation_id`);if err!=nil{return result,storageError(err)};pending:=[]pendingJournal{};for rows.Next(){var record pendingJournal;if err=rows.Scan(&record.ID,&record.Kind,&record.State,&record.Payload);err!=nil{_ = rows.Close();return result,storageError(err)};pending=append(pending,record)};err=rows.Err();_ = rows.Close();if err!=nil{return result,storageError(err)}
 for _,record:=range pending{
  switch record.Kind{
  case "encrypt":
   if record.State!="prepared"&&record.State!="object-installed"{result.RequiresMaintenance=true;result.ControlReady=false;continue}
   var receipt cryptoReceipt;if json.Unmarshal([]byte(record.Payload),&receipt)!=nil||!platform.ValidIdentifier(receipt.ObjectID)||receipt.InputHash==""{return result,platform.NewError(platform.CodeIntegrityError,false,"加密journal损坏")};final:=filepath.Join(store.Root,"active","objects",receipt.ObjectID+".nwb");temp:=filepath.Join(store.Root,"active","objects","."+receipt.ObjectID+".tmp")
   if receipt.Ref==nil{if record.State=="object-installed"{return result,platform.NewError(platform.CodeIntegrityError,false,"已安装密文journal缺少引用")};if _,statErr:=os.Stat(final);!errors.Is(statErr,os.ErrNotExist){result.RequiresMaintenance=true;result.ControlReady=false;continue};if err=lifecycle.ValidatePrivatePath(temp,"regular-file",true);err!=nil{return result,err};if err=os.Remove(temp);err!=nil&&!errors.Is(err,os.ErrNotExist){return result,storageError(err)};if err=syncObjectDirectory(filepath.Dir(temp),store.Root);err!=nil{return result,err};if err=writeCryptoJournal(ctx,store,record.ID,"encrypt","needs-retry",receipt);err!=nil{return result,err};result.Recovered++;continue}
   if _,statErr:=os.Stat(final);errors.Is(statErr,os.ErrNotExist){if _,verifyErr:=VerifyObjectHash(ctx,op,VerifyObjectHashInput{ObjectPath:temp,ExpectedHash:receipt.Ref.SHA256,ExpectedBytes:receipt.Ref.ByteSize});verifyErr!=nil{result.RequiresMaintenance=true;result.ControlReady=false;continue};if err=os.Rename(temp,final);err!=nil{return result,storageError(err)};if err=syncObjectDirectory(filepath.Dir(final),store.Root);err!=nil{return result,err}}
   if err=activateEncryptedObject(ctx,op,store,record.ID,receipt);err!=nil{return result,err};result.Recovered++
  case "decrypt":
   if record.State!="prepared"&&record.State!="verified-temp"{result.RequiresMaintenance=true;result.ControlReady=false;continue}
   var receipt decryptReceipt;if json.Unmarshal([]byte(record.Payload),&receipt)!=nil||!platform.ValidIdentifier(receipt.ObjectID)||!platform.ValidIdentifier(receipt.RestoreID)||receipt.InputHash==""{return result,platform.NewError(platform.CodeIntegrityError,false,"解密journal损坏")}
   if record.State=="verified-temp"{if _,err=activateDecryptedObject(ctx,op,store,record.ID,receipt);err!=nil{return result,err};result.Recovered++;continue}
   dir:=filepath.Join(store.Root,"restore",receipt.RestoreID,"decrypted");temp:=filepath.Join(dir,"."+receipt.ObjectID+".tmp");if err=lifecycle.ValidatePrivatePath(temp,"regular-file",true);err!=nil{return result,err};if err=os.Remove(temp);err!=nil&&!errors.Is(err,os.ErrNotExist){return result,storageError(err)};if _,statErr:=os.Stat(dir);statErr==nil{if err=syncObjectDirectory(dir,store.Root);err!=nil{return result,err}};if err=writeCryptoJournal(ctx,store,record.ID,"decrypt","needs-retry",receipt);err!=nil{return result,err};result.Recovered++
  default:result.RequiresMaintenance=true;result.ControlReady=false
  }
 }
 var decisions int;if err=store.Control.QueryRowContext(ctx,`SELECT COUNT(*) FROM restore_decisions d JOIN restore_runs r ON r.restore_id=d.restore_id WHERE r.status NOT IN('completed','rolled-back','aborted')`).Scan(&decisions);err!=nil{return result,storageError(err)};if decisions>0{result.RequiresMaintenance=true}
 backups,err:=store.DB.QueryContext(ctx,`SELECT backup_id,encrypted_manifest_ref,source_epoch,manifest_hash FROM backup_runs WHERE status='ready'`);if err!=nil{return result,storageError(err)};type readyBackup struct{ID,Ref,Epoch,Hash string};ready:=[]readyBackup{};for backups.Next(){var backup readyBackup;if err=backups.Scan(&backup.ID,&backup.Ref,&backup.Epoch,&backup.Hash);err!=nil{_ = backups.Close();return result,storageError(err)};ready=append(ready,backup)};err=backups.Err();_ = backups.Close();if err!=nil{return result,storageError(err)}
 for _,backup:=range ready{refs:=[]string{backup.Ref};bad:=false;owners:=map[string]bool{};objects,queryErr:=store.DB.QueryContext(ctx,`SELECT owner,encrypted_ref FROM backup_objects WHERE backup_id=? ORDER BY owner`,backup.ID);if queryErr!=nil{return result,storageError(queryErr)};for objects.Next(){var raw,owner string;if err=objects.Scan(&owner,&raw);err!=nil{_ = objects.Close();return result,storageError(err)};if !requiredSnapshotOwners[owner]||owners[owner]{bad=true};owners[owner]=true;refs=append(refs,raw)};err=objects.Err();_ = objects.Close();if err!=nil{return result,storageError(err)};if len(owners)!=len(requiredSnapshotOwners){bad=true}
  for _,raw:=range refs{var ref platform.ObjectRef;if json.Unmarshal([]byte(raw),&ref)!=nil||ref.Owner!="data-protection"||!platform.ValidIdentifier(ref.ObjectID){bad=true;break};if _,verifyErr:=VerifyObjectHash(ctx,op,VerifyObjectHashInput{ObjectPath:filepath.Join(store.Root,"active","objects",ref.ObjectID+".nwb"),ExpectedHash:ref.SHA256,ExpectedBytes:ref.ByteSize});verifyErr!=nil{if ctx.Err()!=nil{return result,storageError(ctx.Err())};bad=true;break}}
  if bad{hashBytes,hashErr:=hex.DecodeString(backup.Hash);if hashErr!=nil||len(hashBytes)!=32||!platform.ValidIdentifier(backup.ID)||!platform.ValidIdentifier(backup.Epoch){return result,platform.NewError(platform.CodeIntegrityError,false,"ready备份的权威元数据也已损坏")};issues,_:=platform.CanonicalJSON([]gen.SafeIssue{{Code:gen.INTEGRITYERROR,Owner:"data-protection",Field:"encryptedObjects",Reason:"ready备份的密文文件缺失或完整性不符"}});tx,txErr:=store.DB.BeginTx(ctx,nil);if txErr!=nil{return result,storageError(txErr)};_,err=tx.ExecContext(ctx,`UPDATE backup_runs SET status='failed',issues_json=? WHERE backup_id=? AND status='ready'`,string(issues),backup.ID);if err==nil{_,err=tx.ExecContext(ctx,`UPDATE catalog_meta SET revision=revision+1 WHERE singleton=1`)};if err!=nil{_ = tx.Rollback();return result,storageError(err)};var revision int64;if err=tx.QueryRowContext(ctx,`SELECT revision FROM catalog_meta WHERE singleton=1`).Scan(&revision);err!=nil{_ = tx.Rollback();return result,storageError(err)};payload,_:=platform.CanonicalJSON(map[string]any{"backupId":backup.ID,"state":"failed","sourceEpoch":backup.Epoch,"manifestHash":backup.Hash});if err=outbox.EnqueueEvent(tx,platform.EventEnvelope{EventID:platform.NewIdentifier(),EventType:"data-protection.backup-state-changed.v1",EventVersion:1,Producer:"data-protection",SubjectID:backup.ID,Revision:revision,OccurredAt:platform.Now(),DataEpoch:op.DataEpoch,Payload:payload});err!=nil{_ = tx.Rollback();return result,err};if err=tx.Commit();err!=nil{return result,storageError(err)};result.Recovered++}
 }
 return result,nil
}
