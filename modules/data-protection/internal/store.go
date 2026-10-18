// Package protection 实现本域备份目录、完整性与恢复编排，不读取其他服务私有存储。
package protection

import (
 "context"
 "database/sql"
 "errors"
 "encoding/json"
 "net/url"
 "os"
 "path/filepath"
 "strings"
 "strconv"
 "sync"
 "sync/atomic"
 _ "modernc.org/sqlite"
 "nice-wiki/libs/platform"
 "nice-wiki/libs/platform/inbox"
 "nice-wiki/libs/platform/lifecycle"
 "nice-wiki/libs/platform/outbox"
 protectionmigrations "nice-wiki/modules/data-protection/migrations"
)

type OperationContext=platform.OperationContext
type BackupJobClient interface{CreateJob(context.Context,any,string)(json.RawMessage,error)}
type Store struct{Root string;Epoch string;Key platform.SigningKey;Keyring map[string]string;AllowedGitHosts []string;GitCredentialRefs map[string]string;Jobs BackupJobClient;Runtime func(context.Context)(platform.RuntimeView,error);Authorize func(context.Context,OperationContext,string,string)error;PayloadValidators map[string]func(json.RawMessage)error;CryptoMu sync.Mutex;DB *sql.DB;Control *sql.DB;closeOnce sync.Once;closeErr error}
var installedStore atomic.Pointer[Store]

// OpenCatalog 打开active业务库、control权威目录和独立vault；控制事实不随业务目录替换。
func OpenCatalog(ctx context.Context,root,epoch string,key platform.SigningKey)(*Store,error){
 if err:=lifecycle.ValidatePrivatePath(root,"directory",false);err!=nil{return nil,err};if err:=rejectLegacyVault(root);err!=nil{return nil,err}
 for _,name:=range []string{"active","control","restore","vault","active/incoming","control/quarantine"}{path:=filepath.Join(root,name);if err:=os.MkdirAll(path,0700);err!=nil{return nil,storageError(err)};if err:=lifecycle.ValidatePrivatePath(path,"directory",false);err!=nil{return nil,err};if err:=syncObjectDirectory(path,root);err!=nil{return nil,err}}
 catalog,err:=openDatabase(ctx,filepath.Join(root,"active","protection.db"));if err!=nil{return nil,err}
 control,err:=openDatabase(ctx,filepath.Join(root,"control","protection-control.db"));if err!=nil{_ = catalog.Close();return nil,err}
 store:=&Store{Root:root,Epoch:epoch,Key:key,DB:catalog,Control:control};success:=false;defer func(){if !success{_ = store.Close()}}()
 for _,item:=range []struct{DB *sql.DB;Name string;Max int}{{catalog,"0001_domain.sql",2},{control,"0002_control.sql",3}}{if err=applyMigration(ctx,item.DB,item.Name,1,item.Max);err!=nil{return nil,err};if err=outbox.EnsureSchema(ctx,item.DB);err!=nil{return nil,err};if err=inbox.EnsureSchema(ctx,item.DB);err!=nil{return nil,err}}
 if err=rejectLegacyControllerRows(ctx,catalog);err!=nil{return nil,err}
 if err=applyMigration(ctx,control,"0003_vault_catalog.sql",2,3);err!=nil{return nil,err};if err=applyMigration(ctx,catalog,"0004_vault_layout.sql",2,2);err!=nil{return nil,err}
 if _,err=catalog.ExecContext(ctx,`INSERT OR IGNORE INTO catalog_meta(singleton,data_epoch,revision) VALUES(1,?,0)`,epoch);err!=nil{return nil,storageError(err)}
 var stored string;if err=catalog.QueryRowContext(ctx,`SELECT data_epoch FROM catalog_meta WHERE singleton=1`).Scan(&stored);err!=nil{return nil,storageError(err)};if stored!=epoch{return nil,platform.NewError(platform.CodeVersionMismatch,false,"目录当前代次与启动运行权威不同")}
 if err=applyMigration(ctx,control,"0005_backup_intent.sql",3,3);err!=nil{return nil,err}
 if err=verifyVaultInventory(ctx,store);err!=nil{return nil,err}
 success=true;return store,nil
}
func openDatabase(ctx context.Context,path string)(*sql.DB,error){
 if err:=lifecycle.ValidatePrivatePath(path,"regular-file",true);err!=nil{return nil,err}
 dsn:=(&url.URL{Scheme:"file",Path:path}).String()+"?_pragma=journal_mode(WAL)&_pragma=synchronous(FULL)&_pragma=foreign_keys(ON)&_pragma=busy_timeout(3000)"
 db,err:=sql.Open("sqlite",dsn);if err!=nil{return nil,storageError(err)};db.SetMaxOpenConns(1);db.SetMaxIdleConns(1)
 if err=db.PingContext(ctx);err!=nil{_ = db.Close();return nil,storageError(err)};return db,nil
}
func applyMigration(ctx context.Context,db *sql.DB,name string,target,maxVersion int)error{
 var version int;if err:=db.QueryRowContext(ctx,`PRAGMA user_version`).Scan(&version);err!=nil{return storageError(err)};if version>maxVersion{return platform.NewError(platform.CodeUnsupportedSchema,false,"保护目录schema高于当前版本")};if version>=target{return nil};if version!=target-1{return platform.NewError(platform.CodeUnsupportedSchema,false,"保护目录迁移版本不连续")}
 raw,err:=protectionmigrations.Files.ReadFile(name);if err!=nil{return platform.NewError(platform.CodeInternal,false,"保护目录迁移未内嵌")};tx,err:=db.BeginTx(ctx,nil);if err!=nil{return storageError(err)};defer tx.Rollback()
 if _,err=tx.ExecContext(ctx,string(raw));err!=nil{return storageError(err)};if _,err=tx.ExecContext(ctx,`PRAGMA user_version=`+strconv.Itoa(target));err!=nil{return storageError(err)};return storageError(tx.Commit())
}
func (s *Store) Close()error{s.closeOnce.Do(func(){s.closeErr=errors.Join(s.DB.Close(),s.Control.Close())});return storageError(s.closeErr)}
func InstallCatalog(s *Store){installedStore.Store(s)}
func Catalog(op OperationContext)(*Store,error){s:=installedStore.Load();if s==nil{return nil,platform.NewError(platform.CodeDependencyUnavailable,true,"保护目录未装配")};if s.Epoch!=op.DataEpoch{return nil,platform.NewError(platform.CodeVersionMismatch,false,"保护目录属于其他数据代次")};return s,nil}
func storageError(err error)error{if err==nil{return nil};return platform.NormalizeIOError(err,platform.CodeInternal,false,"保护私有存储失败")}
func requireAdministrator(op OperationContext)error{if op.Actor.Kind!="admin"{return platform.NewError(platform.CodeForbidden,false,"保护目录只供后台管理员读取")};return nil}

// PrivateObjectPath 只允许本域incoming/vault/restore隔离根，不接受ObjectRef里的任意路径。
func PrivateObjectPath(store *Store,path string)(string,error){
 if !filepath.IsAbs(path)||filepath.Clean(path)!=path{return "",platform.NewError(platform.CodeValidation,false,"对象路径必须是受控绝对路径")}
 allowed:=false;for _,root:=range []string{filepath.Join(store.Root,"active","incoming"),filepath.Join(store.Root,"vault"),filepath.Join(store.Root,"restore")}{relative,err:=filepath.Rel(root,path);if err==nil&&relative!="."&&relative!=".."&&!strings.HasPrefix(relative,".."+string(filepath.Separator)){allowed=true;break}}
 if !allowed{return "",platform.NewError(platform.CodeValidation,false,"对象路径超出本域隔离根")};if err:=lifecycle.ValidatePrivatePath(path,"regular-file",false);err!=nil{return "",err};return path,nil
}

// rejectLegacyVault 未知旧密文不能因为换布局而被静默漏备份或覆盖。
func rejectLegacyVault(root string)error{
 legacy:=filepath.Join(root,"active","objects");if _,err:=os.Lstat(legacy);errors.Is(err,os.ErrNotExist){return nil};if err:=lifecycle.ValidatePrivatePath(legacy,"directory",false);err!=nil{return err};entries,err:=os.ReadDir(legacy);if err!=nil{return storageError(err)};if len(entries)>0{return platform.NewError(platform.CodeLocked,false,"旧保护对象布局非空，必须先明确验证迁移")};return nil
}
func rejectLegacyControllerRows(ctx context.Context,db *sql.DB)error{
 for _,name:=range []string{"backup_runs","backup_objects","stored_objects"}{var exists int;if err:=db.QueryRowContext(ctx,`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`,name).Scan(&exists);err!=nil{return storageError(err)};if exists==0{continue};var count int;if err:=db.QueryRowContext(ctx,`SELECT COUNT(*) FROM `+name).Scan(&count);err!=nil{return storageError(err)};if count>0{return platform.NewError(platform.CodeLocked,false,"旧active控制目录有未知记录，必须先明确验证迁移")}};return nil
}

// verifyVaultInventory 不接纳没有真实索引或原始journal归属的历史密文，未知文件不删除。
func verifyVaultInventory(ctx context.Context,store *Store)error{
 known:=map[string]bool{}
 rows,err:=store.Control.QueryContext(ctx,`SELECT object_id,relative_path FROM stored_objects`);if err!=nil{return storageError(err)}
 for rows.Next(){var id,path string;if err=rows.Scan(&id,&path);err!=nil{_ = rows.Close();return storageError(err)};if !platform.ValidIdentifier(id)||path!=id+".nwb"{_ = rows.Close();return platform.NewError(platform.CodeIntegrityError,false,"vault对象目录路径不合法")};known[path]=true};if err=rows.Err();err!=nil{_ = rows.Close();return storageError(err)};_ = rows.Close()
 rows,err=store.Control.QueryContext(ctx,`SELECT payload_json FROM protection_journal WHERE kind='encrypt'`);if err!=nil{return storageError(err)}
 for rows.Next(){var raw string;var receipt cryptoReceipt;if err=rows.Scan(&raw);err!=nil{_ = rows.Close();return storageError(err)};if json.Unmarshal([]byte(raw),&receipt)!=nil||!platform.ValidIdentifier(receipt.ObjectID)||receipt.InputHash==""{_ = rows.Close();return platform.NewError(platform.CodeIntegrityError,false,"vault加密journal损坏")};known[receipt.ObjectID+".nwb"]=true;known["."+receipt.ObjectID+".tmp"]=true};if err=rows.Err();err!=nil{_ = rows.Close();return storageError(err)};_ = rows.Close()
 entries,err:=os.ReadDir(filepath.Join(store.Root,"vault"));if err!=nil{return storageError(err)};for _,entry:=range entries{if !known[entry.Name()]{return platform.NewError(platform.CodeLocked,false,"vault含未登记历史对象，必须先明确验证迁移")};if err=lifecycle.ValidatePrivatePath(filepath.Join(store.Root,"vault",entry.Name()),"regular-file",false);err!=nil{return err}};return nil
}
