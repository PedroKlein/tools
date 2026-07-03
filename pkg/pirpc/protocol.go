// Package pirpc provides a client for communicating with the Pi coding agent
// via its JSON-RPC protocol over Unix sockets. It is used by the q daemon and
// other tools that need to send prompts to Pi and read streaming responses.
package pirpc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
)

// --- Outbound messages ---

// PromptMsg sends a prompt to the Pi RPC process.
type PromptMsg struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	ID      string `json:"id"`
}

// NewSessionMsg resets the Pi RPC session.
type NewSessionMsg struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

// AbortMsg cancels the current generation.
type AbortMsg struct {
	Type string `json:"type"`
}

// ClientRequest is the message sent from a client to a daemon over the socket.
type ClientRequest struct {
	Type    string `json:"type"`              // "prompt", "new_session", "ping"
	Message string `json:"message,omitempty"` // prompt text (for type=prompt)
}

// --- Inbound events ---

// Event is the raw envelope from Pi RPC.
type Event struct {
	Type                  string                 `json:"type"`
	ID                    string                 `json:"id,omitempty"`
	Success               *bool                  `json:"success,omitempty"`
	Error                 string                 `json:"error,omitempty"`
	AssistantMessageEvent *AssistantMessageEvent `json:"assistantMessageEvent,omitempty"`
}

// AssistantMessageEvent carries text deltas from the assistant.
type AssistantMessageEvent struct {
	Type  string `json:"type"`
	Delta string `json:"delta"`
}

// IsTextDelta returns true if this event carries a text delta.
func (e Event) IsTextDelta() bool {
	return e.Type == "message_update" &&
		e.AssistantMessageEvent != nil &&
		e.AssistantMessageEvent.Type == "text_delta"
}

// IsAgentEnd returns true if this event signals generation complete.
func (e Event) IsAgentEnd() bool {
	return e.Type == "agent_end"
}

// IsResponse returns true if this is a command response (ack/error).
func (e Event) IsResponse() bool {
	return e.Type == "response"
}

// --- Encoder ---

// Encode writes a message as a JSON line to w.
func Encode(w io.Writer, msg any) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("encoding message: %w", err)
	}

	data = append(data, '\n')
	if _, err = w.Write(data); err != nil {
		return fmt.Errorf("writing message: %w", err)
	}

	return nil
}

// --- Decoder ---

// Decoder reads JSONL events from a stream.
type Decoder struct {
	scanner *bufio.Scanner
}

// NewDecoder creates a decoder wrapping the given reader.
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{scanner: bufio.NewScanner(r)}
}

// Next reads the next event. Returns io.EOF when the stream ends.
func (d *Decoder) Next() (Event, error) {
	for d.scanner.Scan() {
		line := d.scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var event Event
		if err := json.Unmarshal(line, &event); err != nil {
			return Event{}, fmt.Errorf("decoding event: %w (line: %s)", err, line)
		}

		return event, nil
	}

	if err := d.scanner.Err(); err != nil {
		return Event{}, fmt.Errorf("reading event stream: %w", err)
	}

	return Event{}, io.EOF
}

// ReadClientRequest reads a single ClientRequest from a reader (one JSON line).
func ReadClientRequest(r io.Reader) (ClientRequest, error) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req ClientRequest
		if err := json.Unmarshal(line, &req); err != nil {
			return ClientRequest{}, fmt.Errorf("decoding request: %w", err)
		}

		return req, nil
	}

	if err := scanner.Err(); err != nil {
		return ClientRequest{}, fmt.Errorf("reading request stream: %w", err)
	}

	return ClientRequest{}, io.EOF
}
