package bootstrap

import (
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
)

func TestCleanupWaitsForActiveHTTPRequests(t *testing.T) {
	app := fiber.New()
	b := &Bootstrap{}
	cleaned := make(chan struct{})
	b.RegisterCleanupFunc(func() error { close(cleaned); return nil })
	b.registerShutdownHooks(app)
	shuttingDown := make(chan struct{})
	app.Hooks().OnPreShutdown(func() error { close(shuttingDown); return nil })
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	t.Cleanup(func() { once.Do(func() { close(release) }) })
	app.Get("/work", func(c fiber.Ctx) error {
		close(entered)
		<-release
		select {
		case <-cleaned:
			return c.SendStatus(500)
		default:
			return c.SendStatus(204)
		}
	})
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() { _ = app.Listener(ln, fiber.ListenConfig{DisableStartupMessage: true}) }()
	result := make(chan error, 1)
	go func() {
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get("http://" + ln.Addr().String() + "/work")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode != 204 {
				t.Errorf("request lost dependencies during shutdown: %d", resp.StatusCode)
			}
		}
		result <- err
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("request did not start")
	}
	stopped := make(chan error, 1)
	go func() { stopped <- app.Shutdown() }()
	select {
	case <-shuttingDown:
	case <-time.After(5 * time.Second):
		t.Fatal("shutdown did not start")
	}
	select {
	case <-cleaned:
		t.Error("cleanup ran before request drained")
	default:
	}
	once.Do(func() { close(release) })
	if err := <-result; err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-stopped:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("shutdown did not finish")
	}
	select {
	case <-cleaned:
	default:
		t.Fatal("cleanup was never called")
	}
}
