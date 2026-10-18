package analytics

import (
 "context"
 "crypto/hmac"
 "crypto/rand"
 "crypto/sha256"
 "encoding/hex"
 "errors"
 "io"
 "os"
 "path/filepath"
 "strings"
 "time"
 "nice-wiki/libs/platform"
 "nice-wiki/libs/platform/lifecycle"
)

// BindReportingTimeZone在当前代次固定计数日界线；未知旧hash不能猜测来源时区。
func BindReportingTimeZone(ctx context.Context,rt *Runtime,name string,view platform.RuntimeView)error{
 if name==""{return nil}
 location,err:=time.LoadLocation(name);if err!=nil{return platform.NewError(platform.CodeValidation,false,"可信报表时区不合法")}
 var stored string;if err=rt.DB.QueryRowContext(ctx,`SELECT reporting_timezone FROM analytics_meta WHERE singleton=1`).Scan(&stored);err!=nil{return storageError(err)}
 if stored!=""&&stored!=name{return platform.NewError(platform.CodeVersionMismatch,false,"当前代次已有不同报表时区")}
 if stored==""{
  if view.Maintenance||view.Phase!=platform.PhaseRunning{return platform.NewError(platform.CodeLocked,true,"维护阶段不能首次绑定报表时区")}
  tx,e:=rt.DB.BeginTx(ctx,nil);if e!=nil{return storageError(e)};defer tx.Rollback();var count int64
  if err=tx.QueryRowContext(ctx,`SELECT count(*) FROM beacon_inbox`).Scan(&count);err!=nil{return storageError(err)}
  if count!=0{return platform.NewError(platform.CodeIntegrityError,false,"旧匿名明细缺少可证明的计数时区")}
  if _,err=tx.ExecContext(ctx,`UPDATE analytics_meta SET reporting_timezone=? WHERE singleton=1 AND reporting_timezone=''`,name);err!=nil{return storageError(err)}
  if _,err=advanceWatermark(ctx,tx,"reporting-config");err!=nil{return err}
  if err=tx.Commit();err!=nil{return storageError(err)}
 }
 rt.CountingLocation=location;return nil
}

// HashDayToken仅处理可信入口交付的地址；原地址与规范UA只在当前栈内存在。
func HashDayToken(ctx context.Context,op OperationContext,in HashDayTokenInput)(HashDayTokenResult,error){
 rt,err:=runtimeFrom(ctx,op);if err!=nil{return HashDayTokenResult{},err}
 if rt.CountingLocation==nil{return HashDayTokenResult{},platform.NewError(platform.CodeDependencyUnavailable,false,"可信报表时区未配置")}
 if in.SiteID!=rt.SiteID||in.Day!=time.Now().In(rt.CountingLocation).Format("2006-01-02"){return HashDayTokenResult{},platform.NewError(platform.CodeValidation,false,"匿名计数日期或站点不匹配")}
 peer,err:=trustedPeer(in.PeerAddress);if err!=nil{return HashDayTokenResult{},err}
 rt.mu.Lock();missing:=rt.salts[in.Day]==nil;rt.mu.Unlock();if missing{if err=PruneDaySalts(ctx,rt,time.Now());err!=nil{return HashDayTokenResult{},err}}
 rt.mu.Lock();defer rt.mu.Unlock();salt:=rt.salts[in.Day]
 if salt==nil{salt,err=loadDaySalt(rt.Root,in.Day);if err!=nil{return HashDayTokenResult{},err};for _,old:=range rt.salts{clear(old)};rt.salts=map[string][]byte{in.Day:salt}}
 raw,err:=platform.CanonicalJSON([]string{rt.SiteID,rt.Epoch,in.Day,peer.String(),strings.ToLower(strings.Join(strings.Fields(in.UserAgent)," "))});if err!=nil{return HashDayTokenResult{},err}
 digest:=hmac.New(sha256.New,salt);_,_ = digest.Write(raw);return HashDayTokenResult{DayHash:hex.EncodeToString(digest.Sum(nil))},nil
}

// loadDaySalt在control固定日期引用创建独立随机盐；残缺旧盐不重建以免同日重复计数。
func loadDaySalt(root,day string)([]byte,error){
 path:=filepath.Join(root,"control","visitor-"+day+".key");if err:=lifecycle.ValidatePrivatePath(path,"regular-file",true);err!=nil{return nil,err}
 file,err:=os.OpenFile(path,os.O_WRONLY|os.O_CREATE|os.O_EXCL,0600)
 if errors.Is(err,os.ErrExist){old,e:=os.Open(path);if e!=nil{return nil,storageError(e)};defer old.Close();info,e:=old.Stat();if e!=nil{return nil,storageError(e)};if info.Mode().Perm()&0077!=0||info.Size()!=32{return nil,platform.NewError(platform.CodeIntegrityError,false,"匿名日盐长度或权限不合法")};raw:=make([]byte,32);if _,e=io.ReadFull(old,raw);e!=nil{return nil,storageError(e)};return raw,nil}
 if err!=nil{return nil,storageError(err)};defer file.Close();raw:=make([]byte,32)
 if _,err=rand.Read(raw);err==nil{_,err=file.Write(raw)};if err==nil{err=file.Sync()};if err!=nil{clear(raw);return nil,storageError(err)}
 if err=syncDirectory(filepath.Dir(path),root);err!=nil{clear(raw);return nil,err};return raw,nil
}
