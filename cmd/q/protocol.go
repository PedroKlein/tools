package main

import (
	"io"

	"github.com/PedroKlein/tools/pkg/pirpc"
)

// Re-export types from pirpc for backward compatibility within cmd/q.
type (
	PromptMsg             = pirpc.PromptMsg
	NewSessionMsg         = pirpc.NewSessionMsg
	AbortMsg              = pirpc.AbortMsg
	ClientRequest         = pirpc.ClientRequest
	Event                 = pirpc.Event
	AssistantMessageEvent = pirpc.AssistantMessageEvent
	Decoder               = pirpc.Decoder
)

var (
	Encode     = pirpc.Encode
	NewDecoder = pirpc.NewDecoder
)

func ReadClientRequest(r io.Reader) (ClientRequest, error) {
	//nolint:wrapcheck // thin internal shim re-exporting pirpc; callers add context
	return pirpc.ReadClientRequest(r)
}
