CREATE TABLE suggestion_approvals (
 approval_id TEXT PRIMARY KEY,
 site_id TEXT NOT NULL,
 data_epoch TEXT NOT NULL,
 version INTEGER NOT NULL CHECK(version > 0),
 enabled INTEGER NOT NULL CHECK(enabled IN (0,1)),
 approved_at TEXT NOT NULL,
 approved_by TEXT NOT NULL,
 permission_revision INTEGER NOT NULL,
 root_operation_id TEXT NOT NULL,
 UNIQUE(site_id,data_epoch,version)
);
CREATE TABLE suggestion_policy (
 site_id TEXT NOT NULL,
 data_epoch TEXT NOT NULL,
 version INTEGER NOT NULL CHECK(version > 0),
 approval_id TEXT NOT NULL REFERENCES suggestion_approvals(approval_id),
 PRIMARY KEY(site_id,data_epoch)
);
CREATE TRIGGER suggestion_approvals_no_update BEFORE UPDATE ON suggestion_approvals
BEGIN SELECT RAISE(ABORT,'immutable suggestion approval'); END;
CREATE TRIGGER suggestion_approvals_no_delete BEFORE DELETE ON suggestion_approvals
BEGIN SELECT RAISE(ABORT,'immutable suggestion approval'); END;
