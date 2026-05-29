package sse

import (
	"time"

	"github.com/wuwuseo/cmf/log"
)

// HubOption 函数式可选参数
type HubOption func(*Hub)

// WithLogger 注入日志器，用于记录推送超时等异常
func WithLogger(l log.Logger) HubOption {
	return func(h *Hub) { h.logger = l }
}

// WithWriteTimeout 设置单条事件写入 channel 的超时时间，默认 500ms
func WithWriteTimeout(d time.Duration) HubOption {
	return func(h *Hub) {
		if d > 0 {
			h.writeTimeout = d
		}
	}
}
