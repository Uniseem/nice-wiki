// Package queue 是事件和任务共用的 PostgreSQL 队列（schema queue），只通过平台库访问。
//
//   - 生产方在自己的业务事务里直接写 queue.events 与 queue.deliveries（同库、同事务），不再有 outbox 转发和 jobs 中转。
//   - 消费方读取时带可见超时，作为租约；读取次数用来计重试；超过次数进死信；本地事务里按事件 ID 去重后再确认。
//   - 延迟消息就是可见时间在未来的行，用于退避和定时。
package queue

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sync"

	_ "github.com/jackc/pgx/v5/stdlib" // 注册 pgx 驱动，队列用原生 PostgreSQL 语句，不经方言翻译

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/pgdb"
)

// schemaSQL 是队列的全部表结构；幂等，各进程启动时都会执行。
var schemaSQL = []string{
	`CREATE SCHEMA IF NOT EXISTS queue`,
	`CREATE TABLE IF NOT EXISTS queue.events (
		event_id      text PRIMARY KEY,
		seq           bigserial UNIQUE,
		event_type    text NOT NULL,
		event_version integer NOT NULL,
		producer      text NOT NULL,
		subject_id    text NOT NULL,
		revision      bigint NOT NULL,
		occurred_at   text NOT NULL,
		data_epoch    text NOT NULL,
		site_id       text NOT NULL DEFAULT '',
		payload       bytea NOT NULL,
		payload_hash  text NOT NULL,
		created_at    timestamptz NOT NULL DEFAULT now()
	)`,
	`CREATE INDEX IF NOT EXISTS events_created ON queue.events (created_at)`,
	`CREATE TABLE IF NOT EXISTS queue.deliveries (
		event_id   text NOT NULL REFERENCES queue.events (event_id) ON DELETE CASCADE,
		consumer   text NOT NULL,
		state      text NOT NULL DEFAULT 'queued',
		attempts   integer NOT NULL DEFAULT 0,
		visible_at timestamptz NOT NULL DEFAULT now(),
		last_error text NOT NULL DEFAULT '',
		updated_at timestamptz NOT NULL DEFAULT now(),
		delivery_id text GENERATED ALWAYS AS (md5(event_id || '|' || consumer)) STORED,
		PRIMARY KEY (event_id, consumer)
	)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS deliveries_id ON queue.deliveries (delivery_id)`,
	`CREATE INDEX IF NOT EXISTS deliveries_due ON queue.deliveries (consumer, visible_at) WHERE state = 'queued'`,
	`CREATE INDEX IF NOT EXISTS deliveries_dead ON queue.deliveries (updated_at) WHERE state = 'dead-letter'`,
	`CREATE TABLE IF NOT EXISTS queue.subscriptions (
		consumer  text NOT NULL,
		event_key text NOT NULL,
		PRIMARY KEY (consumer, event_key)
	)`,
}

// Queue 是进程内共享的队列句柄。
type Queue struct {
	db *sql.DB
}

var (
	openMu sync.Mutex
	opened = map[string]*Queue{}
)

// ForConfig 按组件配置里的数据库连接串返回队列句柄（同一连接串在进程内共用一个），并确保队列表存在。
func ForConfig(ctx context.Context, config platform.ServiceConfig) (*Queue, error) {
	dsn, err := pgdb.ReadDSN(config.DatabaseRef)
	if err != nil || dsn == "" {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, false, "数据库连接串不可读")
	}
	return Open(ctx, dsn)
}

// Open 打开连接串对应的队列。
func Open(ctx context.Context, dsn string) (*Queue, error) {
	openMu.Lock()
	defer openMu.Unlock()
	if q := opened[dsn]; q != nil {
		return q, nil
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "打开队列连接失败")
	}
	// 部署时每个组件一个连接串（一个角色），每个都开一个队列连接池：池子小一些，免得撑满数据库连接数。
	db.SetMaxOpenConns(4)
	if err := ensure(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	q := &Queue{db: db}
	opened[dsn] = q
	return q, nil
}

// ensure 在建议锁内建表，避免几个进程同时启动时互相踩。
func ensure(ctx context.Context, db *sql.DB) error {
	conn, err := db.Conn(ctx)
	if err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "连接队列库失败")
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, `SELECT pg_advisory_lock(7310001)`); err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "获取队列建表锁失败")
	}
	defer conn.ExecContext(context.Background(), `SELECT pg_advisory_unlock(7310001)`)
	// 队列表是各组件共用的：部署时归共享角色所有，各组件角色作为成员读写。
	var shared bool
	if err := conn.QueryRowContext(ctx, pgdb.SharedRoleCheck).Scan(&shared); err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "读取数据库角色失败")
	}
	if shared {
		if _, err := conn.ExecContext(ctx, "SET ROLE "+pgdb.SharedRole); err != nil {
			return platform.NewError(platform.CodeDependencyUnavailable, true, "切换到共享角色失败")
		}
		defer conn.ExecContext(context.Background(), "RESET ROLE")
	}
	for _, statement := range append(append([]string{}, schemaSQL...), jobSchemaSQL...) {
		if _, err := conn.ExecContext(ctx, statement); err != nil {
			if os.Getenv("NW_DEBUG") != "" {
				fmt.Fprintf(os.Stderr, "创建队列表失败：%v\n", err)
			}
			return platform.NewError(platform.CodeDependencyUnavailable, true, "创建队列表失败")
		}
	}
	return nil
}

// DB 返回队列的原生数据库句柄，供 ops 的管理接口读写队列表。
func (q *Queue) DB() *sql.DB { return q.db }
