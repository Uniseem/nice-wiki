package pgdb

import (
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

// mapError 把 PostgreSQL 的约束错误改写成原先 SQLite 的措辞，组件里按错误文本判断冲突的代码不用改。
func mapError(err error) error {
	if err == nil {
		return nil
	}
	var pg *pgconn.PgError
	if !errors.As(err, &pg) {
		return err
	}
	switch pg.Code {
	case "42P01":
		return &constraintError{message: "no such table: " + relationName(pg.Message), cause: err}
	case "42703":
		return &constraintError{message: "no such column: " + pg.Message, cause: err}
	case "23505":
		return &constraintError{message: "constraint failed: UNIQUE constraint failed: " + pg.TableName + "." + columnOf(pg), cause: err}
	case "23503":
		return &constraintError{message: "constraint failed: FOREIGN KEY constraint failed", cause: err}
	case "23502":
		return &constraintError{message: "constraint failed: NOT NULL constraint failed: " + pg.TableName + "." + pg.ColumnName, cause: err}
	case "23514":
		return &constraintError{message: "constraint failed: CHECK constraint failed: " + pg.ConstraintName, cause: err}
	case "P0001":
		// 触发器里 RAISE EXCEPTION：SQLite 的 RAISE(ABORT,'msg') 错误文本就是 msg。
		return &constraintError{message: pg.Message, cause: err}
	}
	return err
}

// relationName 取出 relation "x" does not exist 里的名字。
func relationName(message string) string {
	if open := strings.Index(message, `"`); open >= 0 {
		if closing := strings.Index(message[open+1:], `"`); closing >= 0 {
			return message[open+1 : open+1+closing]
		}
	}
	return message
}

func columnOf(pg *pgconn.PgError) string {
	detail := pg.Detail
	if open := strings.Index(detail, "("); open >= 0 {
		if closing := strings.Index(detail[open:], ")"); closing > 0 {
			return detail[open+1 : open+closing]
		}
	}
	return pg.ConstraintName
}

type constraintError struct {
	message string
	cause   error
}

func (e *constraintError) Error() string { return e.message }
func (e *constraintError) Unwrap() error { return e.cause }
