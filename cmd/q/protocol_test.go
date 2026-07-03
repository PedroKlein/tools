package main

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestEncode_PromptMsg(t *testing.T) {
	var buf bytes.Buffer

	msg := PromptMsg{Type: "prompt", Message: "hello", ID: "abc123"}

	if err := Encode(&buf, msg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	line := buf.String()
	if !strings.HasSuffix(line, "\n") {
		t.Error("expected trailing newline")
	}

	if !strings.Contains(line, `"type":"prompt"`) {
		t.Errorf("expected type field, got: %s", line)
	}

	if !strings.Contains(line, `"message":"hello"`) {
		t.Errorf("expected message field, got: %s", line)
	}
}

func TestEncode_NewSessionMsg(t *testing.T) {
	var buf bytes.Buffer

	msg := NewSessionMsg{Type: "new_session", ID: "xyz"}

	if err := Encode(&buf, msg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	line := buf.String()
	if !strings.Contains(line, `"type":"new_session"`) {
		t.Errorf("expected new_session type, got: %s", line)
	}
}

func TestDecoder_SingleEvent(t *testing.T) {
	input := `{"type":"agent_end"}` + "\n"
	dec := NewDecoder(strings.NewReader(input))

	event, err := dec.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !event.IsAgentEnd() {
		t.Errorf("expected agent_end, got type=%s", event.Type)
	}
}

func TestDecoder_TextDelta(t *testing.T) {
	input := `{"type":"message_update","assistantMessageEvent":{"type":"text_delta","delta":"hello world"}}` + "\n"
	dec := NewDecoder(strings.NewReader(input))

	event, err := dec.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !event.IsTextDelta() {
		t.Error("expected IsTextDelta to be true")
	}

	if event.AssistantMessageEvent.Delta != "hello world" {
		t.Errorf("expected 'hello world', got %q", event.AssistantMessageEvent.Delta)
	}
}

func TestDecoder_MultipleEvents(t *testing.T) {
	input := strings.Join([]string{
		`{"type":"message_update","assistantMessageEvent":{"type":"text_delta","delta":"foo"}}`,
		`{"type":"message_update","assistantMessageEvent":{"type":"text_delta","delta":"bar"}}`,
		`{"type":"agent_end"}`,
	}, "\n") + "\n"

	dec := NewDecoder(strings.NewReader(input))

	// Event 1
	e1, err := dec.Next()
	if err != nil {
		t.Fatalf("event 1: %v", err)
	}

	if e1.AssistantMessageEvent.Delta != "foo" {
		t.Errorf("event 1: expected 'foo', got %q", e1.AssistantMessageEvent.Delta)
	}

	// Event 2
	e2, err := dec.Next()
	if err != nil {
		t.Fatalf("event 2: %v", err)
	}

	if e2.AssistantMessageEvent.Delta != "bar" {
		t.Errorf("event 2: expected 'bar', got %q", e2.AssistantMessageEvent.Delta)
	}

	// Event 3
	e3, err := dec.Next()
	if err != nil {
		t.Fatalf("event 3: %v", err)
	}

	if !e3.IsAgentEnd() {
		t.Errorf("event 3: expected agent_end, got %s", e3.Type)
	}

	// EOF
	_, err = dec.Next()
	if !errors.Is(err, io.EOF) {
		t.Errorf("expected io.EOF, got %v", err)
	}
}

func TestDecoder_EmptyLines(t *testing.T) {
	input := "\n\n" + `{"type":"agent_end"}` + "\n\n"
	dec := NewDecoder(strings.NewReader(input))

	event, err := dec.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !event.IsAgentEnd() {
		t.Errorf("expected agent_end, got %s", event.Type)
	}
}

func TestDecoder_MalformedJSON(t *testing.T) {
	input := "not valid json\n"
	dec := NewDecoder(strings.NewReader(input))

	_, err := dec.Next()
	if err == nil {
		t.Error("expected error for malformed JSON")
	}
}

func TestDecoder_EOF(t *testing.T) {
	dec := NewDecoder(strings.NewReader(""))

	_, err := dec.Next()
	if !errors.Is(err, io.EOF) {
		t.Errorf("expected io.EOF, got %v", err)
	}
}

func TestDecoder_ResponseEvent(t *testing.T) {
	success := true
	input := `{"type":"response","id":"abc","success":true}` + "\n"
	dec := NewDecoder(strings.NewReader(input))

	event, err := dec.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !event.IsResponse() {
		t.Error("expected IsResponse to be true")
	}

	if event.ID != "abc" {
		t.Errorf("expected id abc, got %s", event.ID)
	}

	if event.Success == nil || *event.Success != success {
		t.Error("expected success=true")
	}
}

func TestEvent_UnknownType(t *testing.T) {
	input := `{"type":"some_unknown_event","data":"stuff"}` + "\n"
	dec := NewDecoder(strings.NewReader(input))

	event, err := dec.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should not crash — just returns with type set
	if event.Type != "some_unknown_event" {
		t.Errorf("expected type preserved, got %s", event.Type)
	}

	if event.IsTextDelta() || event.IsAgentEnd() || event.IsResponse() {
		t.Error("unknown type should not match any Is* method")
	}
}
