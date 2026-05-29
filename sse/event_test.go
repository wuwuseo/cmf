package sse

import (
	"bytes"
	"strings"
	"testing"
)

func TestEvent_WriteTo_AllFields(t *testing.T) {
	var buf bytes.Buffer
	e := &Event{ID: "1", Event: "message", Data: map[string]any{"k": "v"}}
	if _, err := e.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}
	s := buf.String()
	for _, want := range []string{"id: 1\n", "event: message\n", `data: {"k":"v"}` + "\n\n"} {
		if !strings.Contains(s, want) {
			t.Fatalf("缺少片段 %q，实际输出 %q", want, s)
		}
	}
}

func TestEvent_WriteTo_NoIDNoEvent(t *testing.T) {
	var buf bytes.Buffer
	e := &Event{Data: 42}
	if _, err := e.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != "data: 42\n\n" {
		t.Fatalf("仅 data 时输出错误: %q", got)
	}
}
