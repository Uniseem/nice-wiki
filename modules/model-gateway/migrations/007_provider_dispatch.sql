-- schemaVersion 7：全部固定路线凭据引用和服务器生成的加密额外请求头。
ALTER TABLE operation_receipts ADD COLUMN result_hash TEXT;
CREATE TABLE route_credential_refs(invocation_id TEXT NOT NULL REFERENCES invocations(invocation_id),provider_id TEXT NOT NULL,credential_version INTEGER NOT NULL,data_epoch TEXT NOT NULL,released INTEGER NOT NULL DEFAULT 0,PRIMARY KEY(invocation_id,provider_id,credential_version));
CREATE TABLE header_secrets(ref_id TEXT PRIMARY KEY,provider_id TEXT NOT NULL,provider_version INTEGER NOT NULL,header_name TEXT NOT NULL,data_epoch TEXT NOT NULL,nonce BLOB NOT NULL,ciphertext BLOB NOT NULL);
CREATE TABLE route_header_refs(invocation_id TEXT NOT NULL REFERENCES invocations(invocation_id),ref_id TEXT NOT NULL REFERENCES header_secrets(ref_id),released INTEGER NOT NULL DEFAULT 0,PRIMARY KEY(invocation_id,ref_id));
CREATE TABLE credential_tracking(singleton INTEGER PRIMARY KEY CHECK(singleton=1),enabled INTEGER NOT NULL DEFAULT 0 CHECK(enabled IN(0,1)));
INSERT INTO credential_tracking(singleton,enabled) VALUES(1,0);
