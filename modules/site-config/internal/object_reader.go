package siteconfig

import (
    "bytes"
    "context"
    "database/sql"
    "errors"
    "io"
    "path/filepath"

    "nice-wiki/libs/platform"
    "nice-wiki/modules/site-config/internal/gen"
)

type ReadConfigObjectInput struct {
    ObjectId gen.Identifier `json:"objectId"`
    Version gen.Revision `json:"version"`
    ExpectedHash gen.Hash `json:"expectedHash"`
}

// ReadConfigObjectResult 仅在本进程流转；HTTP根据Object输出绑定头并消费Reader。
type ReadConfigObjectResult struct {
    Object gen.ObjectRef `json:"object"`
    Reader io.ReadCloser `json:"-"`
}

// ReadConfigObject 只读取已有COMMITTED和正式索引的settings不可变对象。
// 有界字节快照不依赖HTTP生命周期内文件继续存在，不引入全局句柄表。
func ReadConfigObject(ctx context.Context,op platform.OperationContext,input ReadConfigObjectInput) (ReadConfigObjectResult,error) {
    if err := authorizeConfigRead(op,true); err != nil { return ReadConfigObjectResult{},err }
    store := currentActiveStore()
    if store == nil { return ReadConfigObjectResult{},platform.NewError(platform.CodeDependencyUnavailable,true,"active业务库不可用") }
    store.configMu.Lock()
    defer store.configMu.Unlock()
    tx,err := store.db.BeginTx(ctx,&sql.TxOptions{ReadOnly:true})
    if err != nil { return ReadConfigObjectResult{},platform.NewError(platform.CodeDependencyUnavailable,true,"打开配置对象读取事务失败") }
    defer tx.Rollback()
    siteID,current,_,err := store.readConfigWatermark(ctx,op,tx)
    if err != nil { return ReadConfigObjectResult{},err }
    _,record,raw,err := store.readIndexedConfig(ctx,tx,input.ObjectId)
    if err != nil { return ReadConfigObjectResult{},err }
    if record.SiteID != siteID || record.Object.Version > current || record.DataEpoch != op.DataEpoch {
        return ReadConfigObjectResult{},platform.NewError(platform.CodeIntegrityError,false,"配置对象超出当前权威水位")
    }
    if record.Object.Version != input.Version || record.Object.Sha256 != input.ExpectedHash {
        return ReadConfigObjectResult{},platform.NewError(platform.CodeIntegrityError,false,"配置对象与指定版本或hash不一致")
    }
    return ReadConfigObjectResult{Object:record.Object,Reader:io.NopCloser(bytes.NewReader(raw))},nil
}

// readIndexedConfig 将SQL索引、文件决定、原始回执和正文视作同一读取边界。
// 调用方持有configMu；所有文件操作复用config_files.go的目录能力与大小上限。
func (s *ActiveStore) readIndexedConfig(ctx context.Context,tx *sql.Tx,objectID string) (gen.SiteConfig,configJournalRecord,[]byte,error) {
    var siteID,revisionID,hash string
    var size,revision int
    err := tx.QueryRowContext(ctx,`SELECT site_id,revision_id,sha256,byte_size,revision FROM config_revision_index WHERE object_id=?`,objectID).Scan(&siteID,&revisionID,&hash,&size,&revision)
    if errors.Is(err,sql.ErrNoRows) { return gen.SiteConfig{},configJournalRecord{},nil,platform.NewError(platform.CodeNotFound,false,"配置对象尚无完成提交索引") }
    if err != nil { return gen.SiteConfig{},configJournalRecord{},nil,platform.NewError(platform.CodeDependencyUnavailable,true,"配置对象索引不可读") }
    if !configPathID(siteID) || !configPathID(revisionID) { return gen.SiteConfig{},configJournalRecord{},nil,platform.NewError(platform.CodeIntegrityError,false,"配置对象索引标识不合法") }
    record,err := readConfigRecord(configRecordPath(s.activeRoot,revisionID,"COMMITTED"))
    if err != nil { return gen.SiteConfig{},record,nil,err }
    expected := gen.ObjectRef{Owner:"site-config",ObjectId:objectID,Version:revision,Sha256:hash,ByteSize:size,MediaType:"application/json"}
    if record.State != "COMMITTED" || record.SiteID != siteID || record.RevisionID != revisionID || record.Object != expected {
        return gen.SiteConfig{},record,nil,platform.NewError(platform.CodeIntegrityError,false,"配置对象索引与COMMITTED不一致")
    }
    if err := verifyConfigReceipt(ctx,tx,record); err != nil { return gen.SiteConfig{},record,nil,err }
    raw,err := readConfigFile(configObjectPath(s.activeRoot,siteID,revisionID))
    if err != nil { return gen.SiteConfig{},record,nil,err }
    config,err := decodeConfigDocument(raw,expected)
    if err != nil { return config,record,nil,err }
    if config.SiteId != siteID { return config,record,nil,platform.NewError(platform.CodeIntegrityError,false,"配置正文站点与索引不一致") }
    return config,record,raw,nil
}

func configHeadPath(root,siteID string) string { return filepath.Join(root,"settings",siteID,"head.json") }
