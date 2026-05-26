package telemetry

import (
	"encoding/json"
	"os"
	"sync"
	"time"

	"go.opentelemetry.io/otel/trace"
)

type LogEntry struct {
	Pod     string `json:"pod"`
	Source  string `json:"source"`
	Name    string `json:"name"`
	Message string `json:"msg"`
	Ts      int64  `json:"ts"`
}

var (
	mu       sync.RWMutex
	buf      [256]LogEntry
	head     int
	size     int
	nodeName string // dynamic pod name
)

func init() {
	host, err := os.Hostname()
	if err != nil || host == "" {
		nodeName = "pastaay-node"
	} else {
		nodeName = host
	}
}

func Emit(source, name, msg string) {
	mu.Lock()
	defer mu.Unlock()
	buf[head] = LogEntry{
		Source: source, Name: name, Pod: source + "/" + name,
		Message: msg, Ts: time.Now().UnixMilli(),
	}
	head = (head + 1) % 256
	if size < 256 {
		size++
	}
}

func Snapshot() []LogEntry {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]LogEntry, size)
	start := (head - size + 256) % 256
	for i := 0; i < size; i++ {
		out[i] = buf[(start+i)%256]
	}
	return out
}

func EmitError(protocol, target, msg, payload string, span trace.Span) {
	logData := map[string]interface{}{
		"level": "ERROR", "protocol": protocol, "target": target, "message": msg,
		"payload": payload,
	}
	if span != nil && span.SpanContext().IsValid() {
		logData["trace_id"] = span.SpanContext().TraceID().String()
		logData["span_id"] = span.SpanContext().SpanID().String()
	}
	jsonLog, _ := json.Marshal(logData)
	Emit(nodeName, protocol, string(jsonLog))
}

func EmitInfo(protocol, message string, data map[string]interface{}, span trace.Span) {
	data["level"] = "INFO"
	data["protocol"] = protocol
	data["message"] = message
	if span != nil && span.SpanContext().IsValid() {
		data["trace_id"] = span.SpanContext().TraceID().String()
		data["span_id"] = span.SpanContext().SpanID().String()
	}
	jsonLog, _ := json.Marshal(data)
	Emit(nodeName, protocol, string(jsonLog))
}
