package sse

import (
	"context"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
)

func TestHandleConnect_MissingTicket(t *testing.T) {
	app := fiber.New()
	hub := NewHub()
	store := NewTicketStore(time.Minute)
	app.Get("/sse", func(c fiber.Ctx) error {
		return HandleConnect(c, StreamConfig{Hub: hub, TicketStore: store})
	})
	req := httptest.NewRequest("GET", "/sse", nil)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 2 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 401 {
		t.Fatalf("缺少 ticket 应返回 401，实际 %d", resp.StatusCode)
	}
}

func TestHandleConnect_InvalidTicket(t *testing.T) {
	app := fiber.New()
	hub := NewHub()
	store := NewTicketStore(time.Minute)
	app.Get("/sse", func(c fiber.Ctx) error {
		return HandleConnect(c, StreamConfig{Hub: hub, TicketStore: store})
	})
	req := httptest.NewRequest("GET", "/sse?ticket=not-exist", nil)
	resp, _ := app.Test(req, fiber.TestConfig{Timeout: 2 * time.Second})
	if resp.StatusCode != 401 {
		t.Fatalf("无效 ticket 应返回 401，实际 %d", resp.StatusCode)
	}
}

func TestHandleConnect_StreamReadyAndHeartbeat(t *testing.T) {
	app := fiber.New()
	hub := NewHub()
	store := NewTicketStore(time.Minute)
	cfg := StreamConfig{Hub: hub, TicketStore: store, HeartbeatInterval: 100 * time.Millisecond}
	app.Get("/sse", func(c fiber.Ctx) error { return HandleConnect(c, cfg) })

	ticket, _, _ := store.Issue("u1")
	req := httptest.NewRequest("GET", "/sse?ticket="+ticket, nil)

	// 推送一条消息（异步触发，等连接建立后由 hub 推送）
	go func() {
		time.Sleep(80 * time.Millisecond)
		hub.PushToUser("u1", Event{ID: "42", Event: "message", Data: "hello"})
	}()

	resp, err := app.Test(req, fiber.TestConfig{Timeout: 500 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	s := string(body)
	for _, want := range []string{"event: ready", "id: 42", "event: message", `"hello"`} {
		if !strings.Contains(s, want) {
			t.Fatalf("流内容缺少 %q\n----\n%s", want, s)
		}
	}
}

func TestHandleConnect_MissedFetcher(t *testing.T) {
	app := fiber.New()
	hub := NewHub()
	store := NewTicketStore(time.Minute)
	called := 0
	fetcher := func(_ context.Context, uid string, lastID int) ([]Event, error) {
		called++
		if uid != "u1" || lastID != 5 {
			t.Errorf("MissedFetcher 参数错误 uid=%s lastID=%d", uid, lastID)
		}
		return []Event{{ID: "6", Event: "message", Data: "back"}}, nil
	}
	app.Get("/sse", func(c fiber.Ctx) error {
		return HandleConnect(c, StreamConfig{Hub: hub, TicketStore: store, MissedFetcher: fetcher, HeartbeatInterval: time.Second})
	})
	ticket, _, _ := store.Issue("u1")
	req := httptest.NewRequest("GET", "/sse?ticket="+ticket+"&lastEventId=5", nil)
	resp, _ := app.Test(req, fiber.TestConfig{Timeout: 300 * time.Millisecond})
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if called != 1 {
		t.Fatalf("MissedFetcher 应被调用 1 次，实际 %d", called)
	}
	if !strings.Contains(string(body), "id: 6") {
		t.Fatalf("补发的事件 ID:6 未出现在流中，body=%s", string(body))
	}
}
