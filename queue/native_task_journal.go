package queue

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// nativeTaskJournal is an ordered, durable log stored in the selected broker.
// Every participant reduces the same prefix of the log before deciding whether
// a task may run or a management command succeeded.
type nativeTaskJournal interface {
	AppendTaskEvent(context.Context, []byte) error
	TailTaskOffset(context.Context) (uint64, error)
	ReplayTaskEvents(context.Context, uint64, uint64, string, func([]byte) error) (uint64, error)
}

var errNativeTaskBusy = errors.New("queue: 任务已被其他 worker 认领")

const nativeTaskDeleted = "deleted"

type nativeTaskEvent struct {
	EventID string       `json:"event_id"`
	Kind    string       `json:"kind"`
	ID      string       `json:"id,omitempty"`
	Queue   string       `json:"queue,omitempty"`
	Task    notification `json:"task,omitempty"`
	Token   string       `json:"token,omitempty"`
	Attempt int          `json:"attempt,omitempty"`
	Error   string       `json:"error,omitempty"`
	Digest  string       `json:"digest,omitempty"`
	NextAt  time.Time    `json:"next_at,omitempty"`
	At      time.Time    `json:"at"`
}

type nativeTaskRecord struct {
	Info            TaskInfo
	Digest          string
	CreatedAt       time.Time
	Token           string
	LeaseAt         time.Time
	CancelRequested bool
	NeedsDispatch   bool
	DispatchAt      time.Time
	DispatchToken   string
}

type nativeTaskManagement struct {
	journal nativeTaskJournal
	queues  map[string]int
	publish func(context.Context, string, []byte) error
	mu      sync.Mutex
	cursor  uint64
	tasks   map[string]*nativeTaskRecord
	daily   map[string]map[string]*nativeDailyCount
}

type nativeDailyCount struct {
	processed int
	failed    int
}

func nativeTaskKey(queue, id string) string { return queue + "\x00" + id }

func nativeTaskDigest(task notification) string {
	data, _ := json.Marshal(task)
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum)
}

// reduceNativeEvent applies a command only when its precondition still holds.
// This makes concurrent commands deterministic on every process, including
// commands that are rejected because a worker claimed the task first.
func reduceNativeEvent(tasks map[string]*nativeTaskRecord, ev nativeTaskEvent) {
	key := nativeTaskKey(ev.Queue, ev.ID)
	r := tasks[key]
	switch ev.Kind {
	case "create":
		if r != nil || ev.Task.ID == "" || ev.Task.ID != ev.ID {
			return
		}
		record := &nativeTaskRecord{Digest: nativeTaskDigest(ev.Task), CreatedAt: ev.At, NeedsDispatch: ev.NextAt.IsZero(), DispatchAt: ev.At, Info: TaskInfo{
			ID: ev.ID, Queue: ev.Queue, Name: ev.Task.Name, Payload: ev.Task.Payload,
			State: StatePending, MaxRetry: ev.Task.MaxRetry, Timeout: time.Duration(ev.Task.Timeout),
		}}
		if !ev.NextAt.IsZero() {
			record.Info.State = StateScheduled
			record.Info.NextProcessAt = ev.NextAt
		}
		tasks[key] = record
	case "activate":
		if r != nil && r.Info.State == StateScheduled && !ev.At.Before(r.Info.NextProcessAt) {
			r.Info.State = StatePending
			r.Info.NextProcessAt = time.Time{}
			r.NeedsDispatch = true
			r.DispatchAt = ev.At
			r.DispatchToken = ev.Token
		}
	case "claim":
		if r == nil || r.Digest != ev.Digest {
			return
		}
		if r.Info.State != StatePending && r.Info.State != StateRetry &&
			!(r.Info.State == StateActive && ev.At.Sub(r.LeaseAt) > 2*time.Minute) {
			return
		}
		r.Info.State = StateActive
		r.NeedsDispatch = false
		r.DispatchToken = ""
		r.Info.Attempt = ev.Attempt
		r.Token = ev.Token
		r.LeaseAt = ev.At
		r.CancelRequested = false
	case "heartbeat":
		if r != nil && r.Info.State == StateActive && r.Token == ev.Token {
			r.LeaseAt = ev.At
		}
	case "complete":
		if r != nil && r.Info.State == StateActive && r.Token == ev.Token {
			if r.CancelRequested {
				r.Info.LastError = context.Canceled.Error()
				r.Info.LastFailedAt = ev.At
				if r.Info.Attempt < r.Info.MaxRetry {
					r.Info.State = StateRetry
				} else {
					r.Info.State = StateArchived
				}
			} else {
				r.Info.State = StateCompleted
				r.Info.CompletedAt = ev.At
			}
			r.Token = ""
			r.CancelRequested = false
		}
	case "fail":
		if r != nil && r.Info.State == StateActive && r.Token == ev.Token {
			r.Info.Attempt = ev.Attempt
			r.Info.LastError = ev.Error
			r.Info.LastFailedAt = ev.At
			if ev.Attempt < r.Info.MaxRetry {
				r.Info.State = StateRetry
			} else {
				r.Info.State = StateArchived
			}
			r.Token = ""
			r.CancelRequested = false
		}
	case "retry":
		if r != nil && (r.Info.State == StateScheduled || r.Info.State == StateRetry || r.Info.State == StateArchived) {
			r.Info.State = StatePending
			r.Info.Attempt = 0
			r.Info.NextProcessAt = time.Time{}
			r.NeedsDispatch = true
			r.DispatchAt = ev.At
			r.DispatchToken = ""
		}
	case "dispatched":
		if r != nil && r.Info.State == StatePending {
			r.NeedsDispatch = false
			r.DispatchToken = ""
		}
	case "delete":
		if r != nil && r.Info.State != StateActive {
			r.Info.State = nativeTaskDeleted
			r.Info.Payload = nil
			r.Info.Name = ""
			r.NeedsDispatch = false
			r.DispatchToken = ""
		}
	case "cancel":
		if r != nil && r.Info.State == StateActive {
			r.CancelRequested = true
		}
	}
}

func (m *nativeTaskManagement) replay(ctx context.Context, stop uint64, marker string) (map[string]*nativeTaskRecord, error) {
	if m.tasks == nil {
		m.tasks = make(map[string]*nativeTaskRecord)
		m.daily = make(map[string]map[string]*nativeDailyCount)
	}
	if marker == "" && m.cursor == stop {
		return m.copyTasks(), nil
	}
	next, err := m.journal.ReplayTaskEvents(ctx, m.cursor, stop, marker, func(data []byte) error {
		var item nativeTaskEvent
		if err := json.Unmarshal(data, &item); err != nil {
			return fmt.Errorf("queue: 任务状态日志损坏: %w", err)
		}
		reduceNativeEvent(m.tasks, item)
		if item.Kind == "complete" || item.Kind == "fail" {
			if record := m.tasks[nativeTaskKey(item.Queue, item.ID)]; record != nil {
				completed := item.Kind == "complete" && record.Info.CompletedAt.Equal(item.At)
				failed := record.Info.LastFailedAt.Equal(item.At)
				if completed || failed {
					day := item.At.Local().Format("2006-01-02")
					if m.daily[day] == nil {
						m.daily[day] = make(map[string]*nativeDailyCount)
					}
					if m.daily[day][item.Queue] == nil {
						m.daily[day][item.Queue] = &nativeDailyCount{}
					}
					m.daily[day][item.Queue].processed++
					if failed {
						m.daily[day][item.Queue].failed++
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		m.cursor = 0
		m.tasks = nil
		m.daily = nil
		return nil, err
	}
	m.cursor = next
	return m.copyTasks(), nil
}

func (m *nativeTaskManagement) copyTasks() map[string]*nativeTaskRecord {
	tasks := make(map[string]*nativeTaskRecord, len(m.tasks))
	for key, record := range m.tasks {
		copy := *record
		if copy.Info.State == StateActive && time.Since(copy.LeaseAt) > 2*time.Minute {
			copy.Info.IsOrphaned = true
		}
		tasks[key] = &copy
	}
	return tasks
}

func (m *nativeTaskManagement) command(ctx context.Context, ev nativeTaskEvent) (map[string]*nativeTaskRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ev.EventID = uuid.NewString()
	ev.At = time.Now().UTC()
	data, err := json.Marshal(ev)
	if err != nil {
		return nil, err
	}
	if err := m.journal.AppendTaskEvent(ctx, data); err != nil {
		return nil, err
	}
	return m.replay(ctx, 0, ev.EventID)
}

func (m *nativeTaskManagement) snapshot(ctx context.Context) (map[string]*nativeTaskRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	tail, err := m.journal.TailTaskOffset(ctx)
	if err != nil {
		return nil, err
	}
	if tail < m.cursor {
		m.cursor = 0
		m.tasks = nil
		m.daily = nil
		return nil, errors.New("queue: 任务状态日志发生回退")
	}
	return m.replay(ctx, tail, "")
}

func (m *nativeTaskManagement) Create(ctx context.Context, queue string, task notification, due time.Time) error {
	tasks, err := m.command(ctx, nativeTaskEvent{Kind: "create", Queue: queue, ID: task.ID, Task: task, NextAt: due})
	if err != nil {
		return err
	}
	r := tasks[nativeTaskKey(queue, task.ID)]
	if r == nil || r.Digest != nativeTaskDigest(task) || (r.Info.State != StatePending && r.Info.State != StateScheduled) {
		return ErrTaskStateConflict
	}
	return nil
}

func (m *nativeTaskManagement) MarkDispatched(ctx context.Context, queue, id string) error {
	_, err := m.command(ctx, nativeTaskEvent{Kind: "dispatched", Queue: queue, ID: id})
	return err
}

func (m *nativeTaskManagement) Recover(ctx context.Context) error {
	tasks, err := m.snapshot(ctx)
	if err != nil {
		return err
	}
	for _, r := range tasks {
		if r.Info.State == StateScheduled && !time.Now().Before(r.Info.NextProcessAt) {
			token := uuid.NewString()
			activated, err := m.command(ctx, nativeTaskEvent{Kind: "activate", Queue: r.Info.Queue, ID: r.Info.ID, Token: token})
			if err != nil {
				return err
			}
			current := activated[nativeTaskKey(r.Info.Queue, r.Info.ID)]
			if current == nil || current.DispatchToken != token {
				continue
			}
			r = current
		}
		if r.Info.State != StatePending || !r.NeedsDispatch {
			continue
		}
		if r.DispatchToken == "" && time.Since(r.DispatchAt) < 5*time.Second {
			continue
		}
		data, err := nativeTaskBody(r.Info)
		if err != nil {
			return err
		}
		if err := m.publish(ctx, r.Info.Queue, data); err != nil {
			return err
		}
		if err := m.MarkDispatched(ctx, r.Info.Queue, r.Info.ID); err != nil {
			return err
		}
	}
	return nil
}

func nativeTaskBody(info TaskInfo) ([]byte, error) {
	return json.Marshal(notification{ID: info.ID, Name: info.Name, Payload: info.Payload, Timeout: int64(info.Timeout), MaxRetry: info.MaxRetry})
}

func (m *nativeTaskManagement) Claim(ctx context.Context, queue string, task notification, attempt int) (string, bool, error) {
	for importAttempt := 0; importAttempt < 2; importAttempt++ {
		token := uuid.NewString()
		tasks, err := m.command(ctx, nativeTaskEvent{Kind: "claim", Queue: queue, ID: task.ID, Token: token, Attempt: attempt, Digest: nativeTaskDigest(task)})
		if err != nil {
			return "", false, err
		}
		r := tasks[nativeTaskKey(queue, task.ID)]
		if r == nil && importAttempt == 0 {
			// Messages written before the state journal was introduced have no
			// create event. Import them once; a tombstone prevents resurrection.
			if err := m.Create(ctx, queue, task, time.Time{}); err != nil && !errors.Is(err, ErrTaskStateConflict) {
				return "", false, err
			}
			continue
		}
		if r != nil && r.Info.State == StateActive && r.Token != token {
			return "", false, errNativeTaskBusy
		}
		return token, r != nil && r.Info.State == StateActive && r.Token == token, nil
	}
	return "", false, nil
}

func (m *nativeTaskManagement) Finish(ctx context.Context, queue, id, token string, attempt int, processErr error) error {
	ev := nativeTaskEvent{Kind: "complete", Queue: queue, ID: id, Token: token, Attempt: attempt}
	if processErr != nil {
		ev.Kind = "fail"
		ev.Error = processErr.Error()
		if len(ev.Error) > 1024 {
			ev.Error = ev.Error[:1024]
		}
	}
	tasks, err := m.command(ctx, ev)
	if err != nil {
		return err
	}
	r := tasks[nativeTaskKey(queue, id)]
	if r != nil && r.Info.State == StateActive && r.Token != token {
		return errNativeTaskBusy
	}
	if r == nil || r.Token != "" {
		return ErrTaskStateConflict
	}
	if processErr == nil && r.Info.State != StateCompleted {
		return context.Canceled
	}
	return nil
}

func (m *nativeTaskManagement) Heartbeat(ctx context.Context, queue, id, token string) error {
	_, err := m.command(ctx, nativeTaskEvent{Kind: "heartbeat", Queue: queue, ID: id, Token: token})
	return err
}

func (m *nativeTaskManagement) canceled(ctx context.Context, queue, id, token string) (bool, error) {
	tasks, err := m.snapshot(ctx)
	if err != nil {
		return false, err
	}
	r := tasks[nativeTaskKey(queue, id)]
	return r != nil && r.Info.State == StateActive && r.Token == token && r.CancelRequested, nil
}

func (m *nativeTaskManagement) ListTasks(state, queue string, page, pageSize int) ([]TaskInfo, int, error) {
	if _, ok := parseState(state); !ok {
		return nil, 0, ErrInvalidTaskState
	}
	if queue == "" {
		queue = "default"
	}
	if _, ok := m.queues[queue]; !ok {
		return []TaskInfo{}, 0, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	tasks, err := m.snapshot(ctx)
	if err != nil {
		return nil, 0, err
	}
	all := make([]TaskInfo, 0)
	for _, task := range tasks {
		if task.Info.Queue == queue && task.Info.State == state {
			all = append(all, task.Info)
		}
	}
	sort.Slice(all, func(i, j int) bool { return all[i].ID < all[j].ID })
	total := len(all)
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	start := (page - 1) * pageSize
	if start >= total {
		return []TaskInfo{}, total, nil
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return all[start:end], total, nil
}

func (m *nativeTaskManagement) GetTask(queue, id string) (TaskInfo, error) {
	if queue == "" {
		queue = "default"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	tasks, err := m.snapshot(ctx)
	if err != nil {
		return TaskInfo{}, err
	}
	if r := tasks[nativeTaskKey(queue, id)]; r != nil {
		if r.Info.State != nativeTaskDeleted {
			return r.Info, nil
		}
	}
	return TaskInfo{}, ErrTaskNotFound
}

func (m *nativeTaskManagement) QueueStats(queue string) (QueueStats, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	tasks, err := m.snapshot(ctx)
	if err != nil {
		return QueueStats{}, err
	}
	s := nativeQueueStats(tasks, queue)
	m.populateDaily(&s)
	return s, nil
}

func nativeQueueStats(tasks map[string]*nativeTaskRecord, queue string) QueueStats {
	s := QueueStats{Queue: queue}
	oldestPending := time.Time{}
	for _, r := range tasks {
		if r.Info.Queue != queue || r.Info.State == nativeTaskDeleted {
			continue
		}
		s.Size++
		s.MemoryUsage += int64(len(r.Info.Payload) + len(r.Info.Name) + len(r.Info.ID) + 256)
		switch r.Info.State {
		case StatePending:
			s.Pending++
			if oldestPending.IsZero() || r.DispatchAt.Before(oldestPending) {
				oldestPending = r.DispatchAt
			}
		case StateActive:
			s.Active++
		case StateScheduled:
			s.Scheduled++
		case StateRetry:
			s.Retry++
		case StateArchived:
			s.Archived++
		case StateCompleted:
			s.Completed++
		}
	}
	if !oldestPending.IsZero() {
		s.Latency = time.Since(oldestPending)
	}
	return s
}

func (m *nativeTaskManagement) populateDaily(s *QueueStats) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if counts := m.daily[time.Now().Local().Format("2006-01-02")][s.Queue]; counts != nil {
		s.Processed, s.Failed = counts.processed, counts.failed
	}
}

func (m *nativeTaskManagement) AllQueueStats(ctx context.Context) ([]QueueStats, error) {
	tasks, err := m.snapshot(ctx)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(m.queues))
	for queue := range m.queues {
		names = append(names, queue)
	}
	sort.Strings(names)
	out := make([]QueueStats, 0, len(names))
	for _, queue := range names {
		s := nativeQueueStats(tasks, queue)
		m.populateDaily(&s)
		out = append(out, s)
	}
	return out, nil
}

func (m *nativeTaskManagement) Retry(queue, id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	tasks, err := m.snapshot(ctx)
	if err != nil {
		return err
	}
	r := tasks[nativeTaskKey(queue, id)]
	if r == nil || r.Info.State == nativeTaskDeleted {
		return ErrTaskNotFound
	}
	redispatch := r.Info.State == StatePending && r.NeedsDispatch
	if !redispatch && r.Info.State != StateScheduled && r.Info.State != StateRetry && r.Info.State != StateArchived {
		return ErrTaskStateConflict
	}
	if !redispatch {
		tasks, err = m.command(ctx, nativeTaskEvent{Kind: "retry", Queue: queue, ID: id})
		if err != nil {
			return err
		}
		r = tasks[nativeTaskKey(queue, id)]
		if r == nil || r.Info.State != StatePending {
			return ErrTaskStateConflict
		}
	}
	data, err := nativeTaskBody(r.Info)
	if err != nil {
		return err
	}
	if err := m.publish(ctx, queue, data); err != nil {
		return err
	}
	_ = m.MarkDispatched(ctx, queue, id)
	return nil
}

func (m *nativeTaskManagement) Delete(queue, id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	tasks, err := m.snapshot(ctx)
	if err != nil {
		return err
	}
	if r := tasks[nativeTaskKey(queue, id)]; r == nil || r.Info.State == nativeTaskDeleted {
		return ErrTaskNotFound
	}
	tasks, err = m.command(ctx, nativeTaskEvent{Kind: "delete", Queue: queue, ID: id})
	if err != nil {
		return err
	}
	if r := tasks[nativeTaskKey(queue, id)]; r == nil || r.Info.State != nativeTaskDeleted {
		return ErrTaskStateConflict
	}
	return nil
}

func (m *nativeTaskManagement) Cancel(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	tasks, err := m.snapshot(ctx)
	if err != nil {
		return err
	}
	queue := ""
	for _, r := range tasks {
		if r.Info.ID == id && r.Info.State != nativeTaskDeleted {
			queue = r.Info.Queue
			break
		}
	}
	if queue == "" {
		return ErrTaskNotFound
	}
	tasks, err = m.command(ctx, nativeTaskEvent{Kind: "cancel", Queue: queue, ID: id})
	if err != nil {
		return err
	}
	r := tasks[nativeTaskKey(queue, id)]
	if r == nil || r.Info.State != StateActive || !r.CancelRequested {
		return ErrTaskStateConflict
	}
	return nil
}

func (*nativeTaskManagement) Close() error { return nil }

var _ TaskManagementAdapter = (*nativeTaskManagement)(nil)
