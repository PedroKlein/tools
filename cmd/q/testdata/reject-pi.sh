#!/bin/bash

while IFS= read -r line; do
  if [[ $line =~ \"id\":\"([^\"]+)\" ]]; then
    id=${BASH_REMATCH[1]}
    printf '{"type":"response","id":"%s","command":"prompt","success":false,"error":"No API key found for github-copilot."}\n' "$id"
  fi
done
