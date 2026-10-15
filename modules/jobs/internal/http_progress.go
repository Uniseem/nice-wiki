package internal

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/apivalidate"
)

type StreamProgressInput struct {
	JobID         string `json:"jobId"`
	AfterSequence int64  `json:"afterSequence"`
}
type StreamProgressResult struct{ Stream platform.JSONEventStream }
type progressRow struct {
	seq       int64
	phase     string
	completed int64
	total     int64
	created   sql.NullString
}
type progressRead struct {
	store    *Store
	frame    platform.SSEFrame
	after    int64
	found    bool
	terminal bool
	more     bool
}
type progressStream struct {
	store    *Store
	op       OperationContext
	jobID    string
	actorKey string
	ctx      context.Context
	cancel   context.CancelFunc
	mu       sync.Mutex
	after    int64
	initial  bool
	closed   bool
}

// StreamProgress 打开前核当前具体业务权限和续接范围；不把有限数组伪装成SSE。
func StreamProgress(ctx context.Context, op OperationContext, in StreamProgressInput) (StreamProgressResult, error) {
	if op.Actor.Kind != "admin" {
		return StreamProgressResult{}, platform.NewError(platform.CodeForbidden, false, "任务进度仅接受当前后台主体")
	}
	snapshot, err := readProgressFrame(ctx, op, in.JobID, in.AfterSequence, in.AfterSequence == 0)
	if err != nil {
		return StreamProgressResult{}, err
	}
	store := snapshot.store
	identity, _ := json.Marshal(struct{ Site, Subject string }{op.Actor.SiteID, op.Actor.SubjectID})
	key := platform.HashBytes(identity)
	store.mu.RLock()
	if store.closed {
		store.mu.RUnlock()
		return StreamProgressResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "任务进度存储已关闭")
	}
	store.progressMu.Lock()
	if store.progressStreams == nil {
		store.progressStreams = make(map[string]int)
	}
	if store.progressStreams[key] >= 10 {
		store.progressMu.Unlock()
		store.mu.RUnlock()
		return StreamProgressResult{}, platform.NewError(platform.CodeRateLimited, false, "当前主体进度连接已满")
	}
	store.progressStreams[key]++
	store.progressMu.Unlock()
	store.mu.RUnlock()
	life, cancel := context.WithCancel(ctx)
	return StreamProgressResult{Stream: &progressStream{store: store, op: op, jobID: in.JobID, actorKey: key, ctx: life, cancel: cancel, after: in.AfterSequence, initial: in.AfterSequence == 0}}, nil
}

// Next 不持SQL事务等待，逐次核当前权限；单帧背压和网络写超时由公共pump负责。
func (stream *progressStream) Next(ctx context.Context) (platform.SSEFrame, error) {
	readCtx, cancel := context.WithCancel(ctx)
	unlink := context.AfterFunc(stream.ctx, cancel)
	defer unlink()
	defer cancel()
	for {
		stream.mu.Lock()
		closed, after, initial := stream.closed, stream.after, stream.initial
		stream.mu.Unlock()
		if closed {
			return platform.SSEFrame{}, io.EOF
		}
		if currentStore() != stream.store {
			_ = stream.Close()
			return platform.SSEFrame{}, platform.NewError(platform.CodeVersionMismatch, false, "任务进度活动句柄已变化")
		}
		snapshot, err := readProgressFrame(readCtx, stream.op, stream.jobID, after, initial)
		if err != nil {
			_ = stream.Close()
			return platform.SSEFrame{}, err
		}
		stream.mu.Lock()
		if stream.closed {
			stream.mu.Unlock()
			return platform.SSEFrame{}, io.EOF
		}
		stream.after = snapshot.after
		if snapshot.found {
			stream.initial = false
		}
		stream.mu.Unlock()
		if snapshot.found {
			return snapshot.frame, nil
		}
		if snapshot.more {
			continue
		}
		if snapshot.terminal {
			_ = stream.Close()
			return platform.SSEFrame{}, io.EOF
		}
		timer := time.NewTimer(time.Second)
		select {
		case <-readCtx.Done():
			timer.Stop()
			_ = stream.Close()
			return platform.SSEFrame{}, readCtx.Err()
		case <-timer.C:
		}
	}
}

func (stream *progressStream) Close() error {
	stream.mu.Lock()
	if stream.closed {
		stream.mu.Unlock()
		return nil
	}
	stream.closed = true
	stream.cancel()
	stream.mu.Unlock()
	stream.store.progressMu.Lock()
	stream.store.progressStreams[stream.actorKey]--
	if stream.store.progressStreams[stream.actorKey] == 0 {
		delete(stream.store.progressStreams, stream.actorKey)
	}
	stream.store.progressMu.Unlock()
	return nil
}

// readProgressFrame 每次最多暂存64行；首次跳过已过保留期的历史，正游标不越过缺失/过期段。
func readProgressFrame(ctx context.Context, op OperationContext, jobID string, after int64, initial bool) (progressRead, error) {
	authorized, err := authorizeJobManagement(ctx, op, jobID)
	if err != nil {
		return progressRead{}, err
	}
	work, err := beginLeaseOperation(ctx, op, op.DataEpoch)
	if err != nil {
		return progressRead{}, err
	}
	defer work.close()
	job, err := work.readJobRecord(ctx, jobID, op.DataEpoch)
	if err != nil {
		return progressRead{}, err
	}
	if job.owner != authorized.owner || job.jobType != authorized.jobType || job.epoch != authorized.epoch {
		return progressRead{}, platform.NewError(platform.CodeConflict, false, "任务进度授权作用域已变化")
	}
	var latest int64
	if err := work.tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(seq),0) FROM progress WHERE job_id=?`, jobID).Scan(&latest); err != nil {
		return progressRead{}, mapQueueDBError(err)
	}
	if after > latest {
		return progressRead{}, platform.NewError(platform.CodeValidation, false, "进度游标超过已登记水位")
	}
	result := progressRead{store: work.store, after: after, terminal: job.state == "succeeded" || job.state == "cancelled" || job.state == "dead-letter"}
	cutoff := work.now.Add(-7 * 24 * time.Hour)
	rows, err := work.tx.QueryContext(ctx, `SELECT seq,phase,completed_units,total_units,created_at FROM progress WHERE job_id=? AND seq>? ORDER BY seq LIMIT 64`, jobID, result.after)
	if err != nil {
		return progressRead{}, mapQueueDBError(err)
	}
	batch := make([]progressRow, 0, 64)
	for rows.Next() {
		var row progressRow
		if err := rows.Scan(&row.seq, &row.phase, &row.completed, &row.total, &row.created); err != nil {
			rows.Close()
			return progressRead{}, mapQueueDBError(err)
		}
		batch = append(batch, row)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return progressRead{}, mapQueueDBError(err)
	}
	if len(batch) == 0 {
		return result, nil
	}
	for _, row := range batch {
		if !row.created.Valid {
			return progressRead{}, platform.NewError(platform.CodeIntegrityError, false, "历史进度发生时刻未知")
		}
		created, err := time.Parse(time.RFC3339Nano, row.created.String)
		if err != nil {
			return progressRead{}, platform.NewError(platform.CodeIntegrityError, false, "进度发生时刻损坏")
		}
		expired := created.Before(cutoff)
		if !initial && (row.seq != result.after+1 || expired) {
			return progressRead{}, platform.NewError(platform.CodeValidation, false, "进度续接范围缺失或已超过7天")
		}
		result.after = row.seq
		if expired {
			continue
		}
		payload, _ := json.Marshal(struct {
			JobID          string `json:"jobId"`
			Seq            int64  `json:"seq"`
			Phase          string `json:"phase"`
			CompletedUnits int64  `json:"completedUnits"`
			TotalUnits     int64  `json:"totalUnits"`
		}{jobID, row.seq, row.phase, row.completed, row.total})
		result.frame = platform.SSEFrame{ID: strconv.FormatInt(row.seq, 10), Event: "progress", Data: payload}
		result.found = true
		return result, nil
	}
	result.more = result.after < latest
	return result, nil
}

// NewProgressHandlers 只供统一服务边界装配；pump开始后不再写JSON Problem。
func NewProgressHandlers() map[string]http.Handler {
	return map[string]http.Handler{"streamProgress": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		op, _ := platform.OperationContextFrom(r.Context())
		after, err := bindProgressCursor(r)
		if err != nil {
			writeEventResponse(w, op.RequestID, nil, err, 0)
			return
		}
		result, err := StreamProgress(r.Context(), op, StreamProgressInput{JobID: r.PathValue("jobId"), AfterSequence: after})
		if err != nil {
			writeEventResponse(w, op.RequestID, nil, err, 0)
			return
		}
		_ = apivalidate.ServeJSONEventStream(r.Context(), w, result.Stream)
	})}
}

func bindProgressCursor(r *http.Request) (int64, error) {
	var after int64
	present := false
	values := []string{}
	for _, name := range []string{"afterSequence", "lastEventId"} {
		if entries, ok := r.URL.Query()[name]; ok {
			values = append(values, entries...)
		}
	}
	values = append(values, r.Header.Values("Last-Event-ID")...)
	for _, value := range values {
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil || parsed < 0 || (present && parsed != after) {
			return 0, platform.NewError(platform.CodeValidation, false, "进度续接游标无效或别名冲突")
		}
		after, present = parsed, true
	}
	return after, nil
}
