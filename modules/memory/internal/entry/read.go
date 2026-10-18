package entry

import (
 "context"
 "database/sql"
 "encoding/base64"
 "encoding/json"
 "nice-wiki/libs/platform"
 "nice-wiki/modules/memory/internal/gen"
 memoryruntime "nice-wiki/modules/memory/internal/runtime"
)

type ReadEntryInput struct{EntryId string `json:"entryId"`;Version *int `json:"version,omitempty"`}
type ReadEntryResult=gen.ReadEntryResult
type ListEntriesInput struct{State *string `json:"state,omitempty"`;Language *string `json:"language,omitempty"`;Cursor *string `json:"cursor,omitempty"`;Limit int `json:"limit"`}
type ListEntriesResult=gen.ListEntriesResult
func ReadEntry(ctx context.Context,op OperationContext,input ReadEntryInput)(ReadEntryResult,error){
 if err:=memoryruntime.RequireManager(op);err!=nil{return ReadEntryResult{},err};var result ReadEntryResult
 err:=memoryruntime.ReadSnapshot(ctx,func(s *memoryruntime.Store,tx *sql.Tx)error{var err error;result.Entry,err=s.ReadEntryObject(ctx,tx,op.Actor.SiteID,input.EntryId,input.Version);return err});return result,err
}
type entryCursor struct{Site string `json:"site"`;State *string `json:"state"`;Language *string `json:"language"`;Last string `json:"last"`}
func ListEntries(ctx context.Context,op OperationContext,input ListEntriesInput)(ListEntriesResult,error){
 if err:=memoryruntime.RequireManager(op);err!=nil{return ListEntriesResult{},err};result:=ListEntriesResult{Entries:[]gen.MemoryEntry{}}
 err:=memoryruntime.ReadSnapshot(ctx,func(s *memoryruntime.Store,tx *sql.Tx)error{
  cursor:=entryCursor{Site:op.Actor.SiteID,State:input.State,Language:input.Language}
  if input.Cursor!=nil{raw,err:=base64.RawURLEncoding.DecodeString(*input.Cursor);if err!=nil||json.Unmarshal(raw,&cursor)!=nil||cursor.Site!=op.Actor.SiteID||!sameFilter(cursor.State,input.State)||!sameFilter(cursor.Language,input.Language){return platform.NewError(platform.CodeValidation,false,"记忆列表游标不匹配")}}
  query:=`SELECT e.entry_id FROM memory_entries e JOIN entry_current c ON c.site_id=e.site_id AND c.entry_id=e.entry_id AND c.version=e.version WHERE e.site_id=? AND e.entry_id>?`;args:=[]any{op.Actor.SiteID,cursor.Last}
  if input.State!=nil{query+=` AND e.state=?`;args=append(args,*input.State)};if input.Language!=nil{query+=` AND e.language=?`;args=append(args,*input.Language)};query+=` ORDER BY e.entry_id LIMIT ?`;args=append(args,input.Limit+1)
  rows,err:=tx.QueryContext(ctx,query,args...);if err!=nil{return memoryruntime.DBError(err)};ids:=[]string{};for rows.Next(){var id string;if err=rows.Scan(&id);err!=nil{rows.Close();return memoryruntime.DBError(err)};ids=append(ids,id)};err=rows.Err();rows.Close();if err!=nil{return memoryruntime.DBError(err)}
  more:=len(ids)>input.Limit;if more{ids=ids[:input.Limit]};for _,id:=range ids{e,err:=s.ReadEntryObject(ctx,tx,op.Actor.SiteID,id,nil);if err!=nil{return err};result.Entries=append(result.Entries,e)}
  if more{cursor.Last=ids[len(ids)-1];raw,_:=json.Marshal(cursor);next:=base64.RawURLEncoding.EncodeToString(raw);result.NextCursor=&next};return nil
 });return result,err
}
func sameFilter(a,b *string)bool{return (a==nil&&b==nil)||(a!=nil&&b!=nil&&*a==*b)}
