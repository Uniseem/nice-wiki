// Package protection 实现本域备份目录、完整性与恢复编排，不读取其他服务私有存储。
package protection

import (
 "context"
 "database/sql"
 "errors"
 "net/url"
 "os"
 "path/filepath"
 "strings"
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
type Store struct{Root string;Epoch string;Key platform.SigningKey;DB *sql.DB;Control *sql.DB;closeOnce sync.Once;closeErr error}
var installedStore atomic.Pointer[Store]

// OpenCatalog 只打开本域active/control；control决定不随业务目录替换。
func OpenCatalog(ctx context.Context,root,epoch string,key platform.SigningKey)(*Store,error){
 if err:=lifecycle.ValidatePrivatePath(root,"directory",false);err!=nil{return nil,err}
 for _,name:=range []string{"active","control","restore","active/objects","active/incoming","control/quarantine"}{path:=filepath.Join(root,name);if err:=os.MkdirAll(path,0700);err!=nil{return nil,storageError(err)};if err:=lifecycle.ValidatePrivatePath(path,"directory",false);err!=nil{return nil,err}}
 catalog,err:=openDatabase(ctx,filepath.Join(root,"active","protection.db"));if err!=nil{return nil,err}
 control,err:=openDatabase(ctx,filepath.Join(root,"control","protection-control.db"));if err!=nil{_ = catalog.Close();return nil,err}
 store:=&Store{Root:root,Epoch:epoch,Key:key,DB:catalog,Control:control};success:=false;defer func(){if !success{_ = store.Close()}}()
 for _,item:=range []struct{DB *sql.DB;Name string}{{catalog,"0001_domain.sql"},{control,"0002_control.sql"}}{if err=applyMigration(ctx,item.DB,item.Name);err!=nil{return nil,err};if err=outbox.EnsureSchema(ctx,item.DB);err!=nil{return nil,err};if err=inbox.EnsureSchema(ctx,item.DB);err!=nil{return nil,err}}
 if _,err=catalog.ExecContext(ctx,`INSERT OR IGNORE INTO catalog_meta(singleton,data_epoch,revision) VALUES(1,?,0)`,epoch);err!=nil{return nil,storageError(err)}
 var stored string;if err=catalog.QueryRowContext(ctx,`SELECT data_epoch FROM catalog_meta WHERE singleton=1`).Scan(&stored);err!=nil{return nil,storageError(err)};if stored!=epoch{return nil,platform.NewError(platform.CodeVersionMismatch,false,"目录当前代次与启动运行权威不同")}
 success=true;return store,nil
}
func openDatabase(ctx context.Context,path string)(*sql.DB,error){
 if err:=lifecycle.ValidatePrivatePath(path,"regular-file",true);err!=nil{return nil,err}
 dsn:=(&url.URL{Scheme:"file",Path:path}).String()+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)&_pragma=busy_timeout(3000)"
 db,err:=sql.Open("sqlite",dsn);if err!=nil{return nil,storageError(err)};db.SetMaxOpenConns(1);db.SetMaxIdleConns(1)
 if err=db.PingContext(ctx);err!=nil{_ = db.Close();return nil,storageError(err)};return db,nil
}
func applyMigration(ctx context.Context,db *sql.DB,name string)error{
 var version int;if err:=db.QueryRowContext(ctx,`PRAGMA user_version`).Scan(&version);err!=nil{return storageError(err)};if version>1{return platform.NewError(platform.CodeUnsupportedSchema,false,"保护目录schema高于当前版本")};if version==1{return nil}
 raw,err:=protectionmigrations.Files.ReadFile(name);if err!=nil{return platform.NewError(platform.CodeInternal,false,"保护目录迁移未内嵌")};tx,err:=db.BeginTx(ctx,nil);if err!=nil{return storageError(err)};defer tx.Rollback()
 if _,err=tx.ExecContext(ctx,string(raw));err!=nil{return storageError(err)};if _,err=tx.ExecContext(ctx,`PRAGMA user_version=1`);err!=nil{return storageError(err)};return storageError(tx.Commit())
}
func (s *Store) Close()error{s.closeOnce.Do(func(){s.closeErr=errors.Join(s.DB.Close(),s.Control.Close())});return storageError(s.closeErr)}
func InstallCatalog(s *Store){installedStore.Store(s)}
func Catalog(op OperationContext)(*Store,error){s:=installedStore.Load();if s==nil{return nil,platform.NewError(platform.CodeDependencyUnavailable,true,"保护目录未装配")};if s.Epoch!=op.DataEpoch{return nil,platform.NewError(platform.CodeVersionMismatch,false,"保护目录属于其他数据代次")};return s,nil}
func storageError(err error)error{if err==nil{return nil};return platform.NormalizeIOError(err,platform.CodeInternal,false,"保护私有存储失败")}
func requireAdministrator(op OperationContext)error{if op.Actor.Kind!="admin"{return platform.NewError(platform.CodeForbidden,false,"保护目录只供后台管理员读取")};return nil}

// PrivateObjectPath 只允许本域incoming/objects/restore隔离根，不接受ObjectRef里的任意路径。
func PrivateObjectPath(store *Store,path string)(string,error){
 if !filepath.IsAbs(path)||filepath.Clean(path)!=path{return "",platform.NewError(platform.CodeValidation,false,"对象路径必须是受控绝对路径")}
 allowed:=false;for _,root:=range []string{filepath.Join(store.Root,"active","incoming"),filepath.Join(store.Root,"active","objects"),filepath.Join(store.Root,"restore")}{relative,err:=filepath.Rel(root,path);if err==nil&&relative!="."&&relative!=".."&&!strings.HasPrefix(relative,".."+string(filepath.Separator)){allowed=true;break}}
 if !allowed{return "",platform.NewError(platform.CodeValidation,false,"对象路径超出本域隔离根")};if err:=lifecycle.ValidatePrivatePath(path,"regular-file",false);err!=nil{return "",err};return path,nil
}
