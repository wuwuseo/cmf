package queue_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/wuwuseo/cmf/config"
	"github.com/wuwuseo/cmf/queue"
)

// 测试级别全局变量，由 TestMain 初始化
var (
	testRedisAddr = "127.0.0.1:6379"
	testRedisPass = ""
	testRedisDB   = 15

	// asynq 的归档（archived）路径依赖 Redis 6.2+ 的 ZRANGE ... BYSCORE 语法，
	// 老版本 Redis 上该路径不可用且会留下同步残骸；据此跳过相关用例。
	testRedisSupportsArchive = true
)

// TestMain 初始化测试 Redis：优先直连环境变量/本机 Redis（独立 DB 隔离），
// 失败则尝试 testcontainers 容器；两者均不可用时跳过全部测试（与 cmf/redis 测试同模式）。
func TestMain(m *testing.M) {
	// Native broker and runtime unit tests do not need Redis. This mode lets
	// them run on hosts which only provide the selected native broker.
	if os.Getenv("CMF_QUEUE_TEST_ONLY_NATIVE") == "1" {
		os.Exit(m.Run())
	}
	if v := os.Getenv("CMF_QUEUE_TEST_REDIS_ADDR"); v != "" {
		testRedisAddr = v
	}
	if v := os.Getenv("CMF_QUEUE_TEST_REDIS_PASSWORD"); v != "" {
		testRedisPass = v
	}
	if v := os.Getenv("CMF_QUEUE_TEST_REDIS_DB"); v != "" {
		fmt.Sscanf(v, "%d", &testRedisDB)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	probe := goredis.NewClient(&goredis.Options{
		Addr:        testRedisAddr,
		Password:    testRedisPass,
		DB:          testRedisDB,
		DialTimeout: 2 * time.Second,
	})
	if err := probe.Ping(ctx).Err(); err != nil {
		probe.Close()
		addr, containerErr := startTestContainer()
		if containerErr != nil {
			println("跳过队列测试：本机 Redis 不可用且无法启动容器，", containerErr.Error())
			os.Exit(0)
		}
		testRedisAddr = addr
		testRedisPass = ""
		testRedisDB = 0
		probe = goredis.NewClient(&goredis.Options{Addr: testRedisAddr, DB: 0, DialTimeout: 2 * time.Second})
	}

	// 测试前清空隔离 DB，避免历史数据影响断言
	if err := probe.FlushDB(context.Background()).Err(); err != nil {
		println("跳过队列测试：无法清空测试 DB，", err.Error())
		probe.Close()
		os.Exit(0)
	}

	// 探测 Redis 版本：低于 6.2 时归档路径不可用
	if ver, verErr := probe.Info(context.Background(), "server").Result(); verErr == nil {
		for _, line := range strings.Split(ver, "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "redis_version:") {
				continue
			}
			version := strings.TrimPrefix(line, "redis_version:")
			parts := strings.Split(version, ".")
			major, minor := 0, 0
			fmt.Sscanf(parts[0], "%d", &major)
			if len(parts) > 1 {
				fmt.Sscanf(parts[1], "%d", &minor)
			}
			if major < 6 || (major == 6 && minor < 2) {
				testRedisSupportsArchive = false
				println("检测到 Redis", version, "（< 6.2），将跳过归档相关用例")
			}
		}
	}

	probe.Close()

	os.Exit(m.Run())
}

// flushTestDB 清空测试 DB，消除用例间的状态串扰。
func flushTestDB(t *testing.T) {
	t.Helper()
	client := goredis.NewClient(&goredis.Options{
		Addr:        testRedisAddr,
		Password:    testRedisPass,
		DB:          testRedisDB,
		DialTimeout: 2 * time.Second,
	})
	defer client.Close()
	if err := client.FlushDB(context.Background()).Err(); err != nil {
		t.Fatalf("清空测试 DB 失败: %v", err)
	}
}

// startTestContainer 尝试用 testcontainers 启动 Redis 容器，返回连接地址。
func startTestContainer() (string, error) {
	// 惰性引入容器能力：仅在直连失败时执行（编译期常驻依赖，运行期按需使用）
	return runRedisContainer()
}

// waitFor 在 deadline 内轮询条件直到满足或超时。
func waitFor(t *testing.T, timeout time.Duration, cond func() bool, desc string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("等待超时: %s", desc)
}

// newTestConfig 构建指向测试 Redis 的队列配置。
func newTestConfig() queue.Config {
	cfg := queue.DefaultConfig()
	cfg.Addr = testRedisAddr
	cfg.Password = testRedisPass
	cfg.DB = testRedisDB
	cfg.Concurrency = 5
	cfg.Queues = map[string]int{"default": 10, "low": 5}
	cfg.MaxRetry = 1
	cfg.TaskCheckInterval = 100 * time.Millisecond
	cfg.RetryDelay = 100 * time.Millisecond
	return cfg
}

// =============================================================================
// 核心流程测试
// =============================================================================

// TestEnqueueAndProcess 验证入队-消费往返：处理器收到载荷，任务进入 completed 并可被 Inspector 查询。
func TestEnqueueAndProcess(t *testing.T) {
	flushTestDB(t)
	ctx := context.Background()
	cfg := newTestConfig()

	client := queue.NewClient(cfg)
	defer client.Close()
	server := queue.NewServer(cfg)
	inspector := queue.NewInspector(cfg)
	defer inspector.Close()

	received := make(chan string, 4)
	server.Handle("test:echo", func(_ context.Context, task *queue.Task) error {
		received <- string(task.Payload)
		return nil
	})
	if err := server.Start(); err != nil {
		t.Fatalf("启动 Server 失败: %v", err)
	}
	defer server.Shutdown()

	// 注册表校验
	if !server.Has("test:echo") {
		t.Fatal("Has 应返回 true")
	}
	found := false
	for _, name := range server.RegisteredTasks() {
		if name == "test:echo" {
			found = true
		}
	}
	if !found {
		t.Fatal("RegisteredTasks 应包含 test:echo")
	}

	id, err := client.Enqueue(ctx, "test:echo", []byte(`{"x":1}`), queue.WithRetention(time.Minute))
	if err != nil {
		t.Fatalf("入队失败: %v", err)
	}
	if id == "" {
		t.Fatal("入队应返回任务 ID")
	}

	select {
	case payload := <-received:
		if payload != `{"x":1}` {
			t.Fatalf("载荷不符: %s", payload)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("5 秒内未收到任务执行")
	}

	// 轮询至 completed 并核对详情
	var info queue.TaskInfo
	deadline := time.Now().Add(5 * time.Second)
	for {
		info, err = inspector.GetTask("default", id)
		if err == nil && info.State == queue.StateCompleted {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("等待超时: 任务应进入 completed（id=%s last_err=%v last_state=%s）", id, err, info.State)
		}
		time.Sleep(100 * time.Millisecond)
	}
	if info.Name != "test:echo" || info.ID != id {
		t.Fatalf("任务信息不符: %+v", info)
	}
	if info.CompletedAt.IsZero() {
		t.Fatal("completed 任务应有完成时间")
	}

	// 列表接口应能查到
	items, total, err := inspector.ListTasks(queue.StateCompleted, "default", 1, 20)
	if err != nil {
		t.Fatalf("ListTasks 失败: %v", err)
	}
	if total < 1 {
		t.Fatalf("completed 总数应 >= 1，实际 %d", total)
	}
	listed := false
	for _, item := range items {
		if item.ID == id {
			listed = true
		}
	}
	if !listed {
		t.Fatal("ListTasks 结果应包含该任务")
	}
}

// TestRetryAndArchive 验证失败重试与死信流转：失败进入 retry，重试超限进入 archived，可手动重试恢复。
func TestRetryAndArchive(t *testing.T) {
	flushTestDB(t)
	ctx := context.Background()
	cfg := newTestConfig()

	client := queue.NewClient(cfg)
	defer client.Close()
	server := queue.NewServer(cfg)
	inspector := queue.NewInspector(cfg)
	defer inspector.Close()

	var shouldSucceed atomic.Bool
	server.Handle("test:flaky", func(_ context.Context, _ *queue.Task) error {
		if !shouldSucceed.Load() {
			return errors.New("模拟失败")
		}
		return nil
	})
	if err := server.Start(); err != nil {
		t.Fatalf("启动 Server 失败: %v", err)
	}
	defer server.Shutdown()

	id, err := client.Enqueue(ctx, "test:flaky", nil, queue.WithRetention(time.Minute))
	if err != nil {
		t.Fatalf("入队失败: %v", err)
	}

	if !testRedisSupportsArchive {
		t.Skip("Redis < 6.2，归档（active→archived）路径不可用")
	}

	// 第一次失败：进入 retry 状态
	var info queue.TaskInfo
	waitFor(t, 5*time.Second, func() bool {
		info, err = inspector.GetTask("default", id)
		return err == nil && info.State == queue.StateRetry
	}, "任务失败后应进入 retry")
	if info.LastError == "" {
		t.Fatal("retry 任务应记录失败原因")
	}
	if info.Attempt != 1 || info.MaxRetry != 1 {
		t.Fatalf("重试计数不符: attempt=%d maxRetry=%d", info.Attempt, info.MaxRetry)
	}

	// 第二次失败：重试超限，进入 archived（死信）
	waitFor(t, 8*time.Second, func() bool {
		info, err = inspector.GetTask("default", id)
		return err == nil && info.State == queue.StateArchived
	}, "重试超限后应进入 archived")

	// 手动重试死信：允许成功后任务应完成
	shouldSucceed.Store(true)
	if err := inspector.Retry("default", id); err != nil {
		t.Fatalf("手动重试死信失败: %v", err)
	}
	waitFor(t, 5*time.Second, func() bool {
		info, err = inspector.GetTask("default", id)
		return err == nil && info.State == queue.StateCompleted
	}, "手动重试后任务应完成")
}

// TestDelayedTask 验证延迟执行：入队后处于 scheduled，到期后被消费。
func TestDelayedTask(t *testing.T) {
	flushTestDB(t)
	ctx := context.Background()
	cfg := newTestConfig()

	client := queue.NewClient(cfg)
	defer client.Close()
	server := queue.NewServer(cfg)
	inspector := queue.NewInspector(cfg)
	defer inspector.Close()

	done := make(chan struct{}, 1)
	server.Handle("test:delayed", func(_ context.Context, _ *queue.Task) error {
		done <- struct{}{}
		return nil
	})
	if err := server.Start(); err != nil {
		t.Fatalf("启动 Server 失败: %v", err)
	}
	defer server.Shutdown()

	id, err := client.Enqueue(ctx, "test:delayed", nil, queue.WithDelay(1500*time.Millisecond))
	if err != nil {
		t.Fatalf("入队失败: %v", err)
	}

	// 入队后应处于 scheduled 且带下次执行时间
	info, err := inspector.GetTask("default", id)
	if err != nil {
		t.Fatalf("查询任务失败: %v", err)
	}
	if info.State != queue.StateScheduled {
		t.Fatalf("延迟任务应为 scheduled，实际 %s", info.State)
	}
	if info.NextProcessAt.IsZero() {
		t.Fatal("scheduled 任务应有下次执行时间")
	}

	select {
	case <-done:
	case <-time.After(8 * time.Second):
		t.Fatal("延迟任务未被按时执行")
	}
}

// TestDeleteTask 验证删除尚未执行的任务后不可再查询。
func TestDeleteTask(t *testing.T) {
	flushTestDB(t)
	ctx := context.Background()
	cfg := newTestConfig()

	client := queue.NewClient(cfg)
	defer client.Close()
	inspector := queue.NewInspector(cfg)
	defer inspector.Close()

	id, err := client.Enqueue(ctx, "test:nobody", nil, queue.WithDelay(time.Hour))
	if err != nil {
		t.Fatalf("入队失败: %v", err)
	}
	if err := inspector.Delete("default", id); err != nil {
		t.Fatalf("删除任务失败: %v", err)
	}
	if _, err := inspector.GetTask("default", id); !errors.Is(err, queue.ErrTaskNotFound) {
		t.Fatalf("删除后查询应返回 ErrTaskNotFound，实际: %v", err)
	}
}

// TestUniqueEnqueue 验证去重窗口：窗口内相同任务拒绝重复入队。
func TestUniqueEnqueue(t *testing.T) {
	flushTestDB(t)
	ctx := context.Background()
	cfg := newTestConfig()

	client := queue.NewClient(cfg)
	defer client.Close()

	if _, err := client.Enqueue(ctx, "test:unique", []byte(`{"k":1}`), queue.WithUnique(time.Minute)); err != nil {
		t.Fatalf("首次入队失败: %v", err)
	}
	if _, err := client.Enqueue(ctx, "test:unique", []byte(`{"k":1}`), queue.WithUnique(time.Minute)); !errors.Is(err, queue.ErrDuplicateTask) {
		t.Fatalf("重复入队应返回 ErrDuplicateTask，实际: %v", err)
	}
}

// TestQueueStats 验证队列统计与非法状态过滤。
func TestQueueStats(t *testing.T) {
	flushTestDB(t)
	ctx := context.Background()
	cfg := newTestConfig()

	client := queue.NewClient(cfg)
	defer client.Close()
	server := queue.NewServer(cfg)
	inspector := queue.NewInspector(cfg)
	defer inspector.Close()

	server.Handle("test:stats", func(_ context.Context, _ *queue.Task) error { return nil })
	if err := server.Start(); err != nil {
		t.Fatalf("启动 Server 失败: %v", err)
	}
	defer server.Shutdown()

	if _, err := client.Enqueue(ctx, "test:stats", nil, queue.WithQueue("low"), queue.WithRetention(time.Minute)); err != nil {
		t.Fatalf("入队失败: %v", err)
	}

	waitFor(t, 5*time.Second, func() bool {
		stats, err := inspector.QueueStats("low")
		return err == nil && stats.Processed >= 1
	}, "low 队列统计应记录到处理量")

	stats, err := inspector.QueueStats("low")
	if err != nil {
		t.Fatalf("QueueStats 失败: %v", err)
	}
	if stats.Queue != "low" || stats.Completed < 1 {
		t.Fatalf("统计快照不符: %+v", stats)
	}

	// 全队列快照应包含 low
	all, err := inspector.AllQueueStats(ctx)
	if err != nil {
		t.Fatalf("AllQueueStats 失败: %v", err)
	}
	seen := false
	for _, s := range all {
		if s.Queue == "low" {
			seen = true
		}
	}
	if !seen {
		t.Fatal("AllQueueStats 应包含 low 队列")
	}

	// 非法状态应被拒绝
	if _, _, err := inspector.ListTasks("bogus", "low", 1, 10); !errors.Is(err, queue.ErrInvalidTaskState) {
		t.Fatalf("非法状态应返回 ErrInvalidTaskState，实际: %v", err)
	}
	// 不存在的队列返回空列表
	items, total, err := inspector.ListTasks(queue.StatePending, "no-such-queue", 1, 10)
	if err != nil || total != 0 || len(items) != 0 {
		t.Fatalf("不存在队列应返回空列表，实际: %v %d %d", err, total, len(items))
	}
}

// TestPruneArchived 验证死信清理：早于截止时间的 archived 任务被删除。
func TestPruneArchived(t *testing.T) {
	flushTestDB(t)
	if !testRedisSupportsArchive {
		t.Skip("Redis < 6.2，归档（active→archived）路径不可用")
	}
	ctx := context.Background()
	cfg := newTestConfig()
	cfg.MaxRetry = 0 // 失败一次即进死信，加速测试

	client := queue.NewClient(cfg)
	defer client.Close()
	server := queue.NewServer(cfg)
	inspector := queue.NewInspector(cfg)
	defer inspector.Close()

	server.Handle("test:dead", func(_ context.Context, _ *queue.Task) error {
		return errors.New("必失败")
	})
	if err := server.Start(); err != nil {
		t.Fatalf("启动 Server 失败: %v", err)
	}
	defer server.Shutdown()

	id, err := client.Enqueue(ctx, "test:dead", nil)
	if err != nil {
		t.Fatalf("入队失败: %v", err)
	}
	waitFor(t, 5*time.Second, func() bool {
		info, err := inspector.GetTask("default", id)
		return err == nil && info.State == queue.StateArchived
	}, "任务应进入 archived")

	// 刚归档的任务不能被一小时前的截止时间清理。
	removed, err := inspector.PruneArchived("default", time.Now().Add(-time.Hour))
	if err != nil || removed != 0 {
		t.Fatalf("新死信应保留，removed=%d, err=%v", removed, err)
	}
	removed, err = inspector.PruneArchived("default", time.Now().Add(time.Second))
	if err != nil {
		t.Fatalf("PruneArchived 失败: %v", err)
	}
	if removed < 1 {
		t.Fatalf("应清理至少 1 条死信，实际 %d", removed)
	}
	if _, err := inspector.GetTask("default", id); !errors.Is(err, queue.ErrTaskNotFound) {
		t.Fatalf("清理后查询应返回 ErrTaskNotFound，实际: %v", err)
	}
}

// TestUnregisteredTaskGoesArchived 验证未注册任务在执行端失败并进入死信（入队端不报错）。
func TestUnregisteredTaskGoesArchived(t *testing.T) {
	flushTestDB(t)
	ctx := context.Background()
	cfg := newTestConfig()
	cfg.MaxRetry = 0

	client := queue.NewClient(cfg)
	defer client.Close()
	server := queue.NewServer(cfg)
	inspector := queue.NewInspector(cfg)
	defer inspector.Close()

	if err := server.Start(); err != nil {
		t.Fatalf("启动 Server 失败: %v", err)
	}
	defer server.Shutdown()

	if !testRedisSupportsArchive {
		t.Skip("Redis < 6.2，归档（active→archived）路径不可用")
	}

	id, err := client.Enqueue(ctx, "test:ghost", nil)
	if err != nil {
		t.Fatalf("入队失败: %v", err)
	}
	waitFor(t, 5*time.Second, func() bool {
		info, err := inspector.GetTask("default", id)
		return err == nil && info.State == queue.StateArchived
	}, "未注册任务应进入 archived")
}

// TestConfigFromApp 验证从 cmf 全局配置构建队列配置：正常路径与缺失连接报错。
func TestConfigFromApp(t *testing.T) {
	cfg := &config.Config{}
	cfg.Redis.Default = "redis"
	cfg.Redis.Connections = map[string]config.Redis{
		"redis": {Addr: "127.0.0.1:6379", Password: "root", DB: 2},
	}

	qcfg, err := queue.NewConfigFromApp(cfg)
	if err != nil {
		t.Fatalf("构建队列配置失败: %v", err)
	}
	if qcfg.Addr != "127.0.0.1:6379" || qcfg.DB != 2 || qcfg.MaxRetry != 3 {
		t.Fatalf("队列配置字段不符: %+v", qcfg)
	}

	// 显式连接名优先
	cfg.Redis.Connections["other"] = config.Redis{Addr: "127.0.0.1:6380", DB: 5}
	qcfg, err = queue.NewConfigFromApp(cfg, "other")
	if err != nil {
		t.Fatalf("构建队列配置失败: %v", err)
	}
	if qcfg.Addr != "127.0.0.1:6380" || qcfg.DB != 5 {
		t.Fatalf("显式连接名未生效: %+v", qcfg)
	}

	// 缺失连接应报错
	if _, err := queue.NewConfigFromApp(cfg, "missing"); !errors.Is(err, queue.ErrRedisConnectionNotFound) {
		t.Fatalf("缺失连接应返回 ErrRedisConnectionNotFound，实际: %v", err)
	}
}
