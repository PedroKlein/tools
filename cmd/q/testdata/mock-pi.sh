#!/bin/bash
# Mock pi --mode rpc for integration testing.
# Reads JSONL from stdin, writes canned responses to stdout.
# Simulates pi's RPC protocol without any network calls.

while IFS= read -r line; do
  type=$(echo "$line" | grep -o '"type":"[^"]*"' | head -1 | cut -d'"' -f4)

  case "$type" in
    prompt)
      # Extract message for echo
      msg=$(echo "$line" | grep -o '"message":"[^"]*"' | head -1 | cut -d'"' -f4)
      # Send text deltas
      echo '{"type":"message_update","assistantMessageEvent":{"type":"text_delta","delta":"mock: "}}'
      echo '{"type":"message_update","assistantMessageEvent":{"type":"text_delta","delta":"'"$msg"'"}}'
      echo '{"type":"agent_end"}'
      ;;
    new_session)
      echo '{"type":"response","success":true}'
      ;;
    *)
      echo '{"type":"response","success":false,"error":"unknown type: '"$type"'"}'
      ;;
  esac
done
