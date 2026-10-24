package migrations

import (
	"database/sql"
	_ "modernc.org/sqlite"
	"testing"
)

func TestAgentMigrationUpgradesDefaultsAndKeepsCustomLimits(t *testing.T) {
	for _, limits := range []struct{ rounds, calls, expectedRounds, expectedCalls int }{{3, 5, 12, 48}, {2, 4, 2, 4}} {
		db, err := sql.Open("sqlite", ":memory:")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`CREATE TABLE messages(message_id TEXT PRIMARY KEY); CREATE TABLE qa_policy(version INTEGER,policy_json BLOB);`); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO qa_policy VALUES(1,json_object('version',1,'maximumRetrievalRounds',?,'maximumToolCalls',?))`, limits.rounds, limits.calls); err != nil {
			t.Fatal(err)
		}
		all, err := All()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(all[len(all)-1].SQL); err != nil {
			t.Fatal(err)
		}
		var rounds, calls, version, jsonVersion int
		if err := db.QueryRow(`SELECT json_extract(policy_json,'$.maximumRetrievalRounds'),json_extract(policy_json,'$.maximumToolCalls'),version,json_extract(policy_json,'$.version') FROM qa_policy`).Scan(&rounds, &calls, &version, &jsonVersion); err != nil {
			t.Fatal(err)
		}
		if rounds != limits.expectedRounds || calls != limits.expectedCalls || version != jsonVersion {
			t.Fatal("agent默认预算或管理员设置迁移错误")
		}
		if _, err := db.Exec(`INSERT INTO messages(message_id,agent_messages_json) VALUES('question','{}')`); err != nil {
			t.Fatal("独立工具消息列没有建立")
		}
		db.Close()
	}
}
