#!/bin/sh

# Make sure to set these environment variables before launching claude code for testing

## Prevent using any model other than haiku 
# ANTHROPIC_DEFAULT_SONNET_MODEL="anthropic/claude-haiku-4.5"
# ANTHROPIC_DEFAULT_OPUS_MODEL="anthropic/claude-haiku-4.5"
# ANTHROPIC_DEFAULT_HAIKU_MODEL="anthropic/claude-haiku-4.5"

## Forces claude code's requests through tester's proxy
# ANTHROPIC_BASE_URL="http://localhost:10000/api"
# ANTHROPIC_AUTH_TOKEN="<OPENROUTER_API_KEY>"
# ANTHROPIC_API_KEY="" (Must be empty)
# Source: https://openrouter.ai/docs/guides/guides/claude-code-integration
exec claude "$@"
