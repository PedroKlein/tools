package pirpc_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"runtime"
	"strings"
	"testing"

	"github.com/PedroKlein/tools/pkg/pirpc"
)

func TestSocketPath(t *testing.T) {
	path := pirpc.SocketPath()
	if path == "" {
		t.Fatal("SocketPath() returned empty string")
	}

	if !strings.HasSuffix(path, "q.sock") {
		t.Errorf("SocketPath() = %q, want suffix q.sock", path)
	}

	if runtime.GOOS == "darwin" && !strings.Contains(path, "q-") {
		t.Errorf("on darwin, expected path to contain 'q-', got %q", path)
	}
}

func TestPIDPath(t *testing.T) {
	path := pirpc.PIDPath()
	if path == "" {
		t.Fatal("PIDPath() returned empty string")
	}

	if !strings.HasSuffix(path, "q.pid") {
		t.Errorf("PIDPath() = %q, want suffix q.pid", path)
	}
}

func TestEncode(t *testing.T) {
	var buf bytes.Buffer

	msg := pirpc.ClientRequest{Type: "prompt", Message: "hello"}
	if err := pirpc.Encode(&buf, msg); err != nil {
		t.Fatalf("Encode: %v", err)
	}

	line := buf.String()
	if !strings.HasSuffix(line, "\n") {
		t.Error("encoded message should end with newline")
	}

	var decoded pirpc.ClientRequest
	if err := json.Unmarshal([]byte(strings.TrimSpace(line)), &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.Type != "prompt" || decoded.Message != "hello" {
		t.Errorf("round-trip failed: got %+v", decoded)
	}
}

func TestDecoderNext(t *testing.T) {
	events := []string{
		`{"type":"message_update","assistantMessageEvent":{"type":"text_delta","delta":"Hello"}}`,
		`{"type":"message_update","assistantMessageEvent":{"type":"text_delta","delta":" world"}}`,
		`{"type":"agent_end"}`,
	}
	input := strings.Join(events, "\n") + "\n"

	dec := pirpc.NewDecoder(strings.NewReader(input))

	// First event: text delta
	e1, err := dec.Next()
	if err != nil {
		t.Fatalf("event 1: %v", err)
	}

	if !e1.IsTextDelta() {
		t.Errorf("event 1 should be text delta, got type=%s", e1.Type)
	}

	if e1.AssistantMessageEvent.Delta != "Hello" {
		t.Errorf("event 1 delta = %q, want Hello", e1.AssistantMessageEvent.Delta)
	}

	// Second event: text delta
	e2, err := dec.Next()
	if err != nil {
		t.Fatalf("event 2: %v", err)
	}

	if e2.AssistantMessageEvent.Delta != " world" {
		t.Errorf("event 2 delta = %q, want ' world'", e2.AssistantMessageEvent.Delta)
	}

	// Third event: agent_end
	e3, err := dec.Next()
	if err != nil {
		t.Fatalf("event 3: %v", err)
	}

	if !e3.IsAgentEnd() {
		t.Errorf("event 3 should be agent_end, got type=%s", e3.Type)
	}

	// EOF
	_, err = dec.Next()
	if !errors.Is(err, io.EOF) {
		t.Errorf("expected io.EOF after all events, got %v", err)
	}
}

func TestDecoderSkipsEmptyLines(t *testing.T) {
	input := "\n\n" + `{"type":"agent_end"}` + "\n\n"
	dec := pirpc.NewDecoder(strings.NewReader(input))

	e, err := dec.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !e.IsAgentEnd() {
		t.Errorf("expected agent_end, got type=%s", e.Type)
	}
}

func TestEventHelpers(t *testing.T) {
	tests := []struct {
		name  string
		event pirpc.Event
		delta bool
		end   bool
		resp  bool
	}{
		{
			name:  "text delta",
			event: pirpc.Event{Type: "message_update", AssistantMessageEvent: &pirpc.AssistantMessageEvent{Type: "text_delta", Delta: "x"}},
			delta: true,
		},
		{
			name:  "agent end",
			event: pirpc.Event{Type: "agent_end"},
			end:   true,
		},
		{
			name:  "response",
			event: pirpc.Event{Type: "response"},
			resp:  true,
		},
		{
			name:  "message_update without delta type",
			event: pirpc.Event{Type: "message_update", AssistantMessageEvent: &pirpc.AssistantMessageEvent{Type: "other"}},
			delta: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.event.IsTextDelta(); got != tt.delta {
				t.Errorf("IsTextDelta() = %v, want %v", got, tt.delta)
			}

			if got := tt.event.IsAgentEnd(); got != tt.end {
				t.Errorf("IsAgentEnd() = %v, want %v", got, tt.end)
			}

			if got := tt.event.IsResponse(); got != tt.resp {
				t.Errorf("IsResponse() = %v, want %v", got, tt.resp)
			}
		})
	}
}

func TestReadClientRequest(t *testing.T) {
	input := `{"type":"prompt","message":"test"}` + "\n"

	req, err := pirpc.ReadClientRequest(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ReadClientRequest: %v", err)
	}

	if req.Type != "prompt" || req.Message != "test" {
		t.Errorf("got %+v, want type=prompt message=test", req)
	}
}

func TestReadClientRequestEOF(t *testing.T) {
	_, err := pirpc.ReadClientRequest(strings.NewReader(""))
	if !errors.Is(err, io.EOF) {
		t.Errorf("expected io.EOF on empty input, got %v", err)
	}
}

func TestIsAvailableNoSocket(t *testing.T) {
	// With no daemon running on a random path, should return false
	// We can't easily test this without setting env, but at minimum it shouldn't panic
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())

	if pirpc.IsAvailable() {
		t.Error("IsAvailable() should be false with no daemon socket")
	}
}
