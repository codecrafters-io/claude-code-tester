#!/bin/sh

# ANTHROPIC_DEFAULT_X_MODEL -> Forces Claude Code to use just the Haiku model
# ANTHROPIC_BASE_URL -> Required for Claude Code
# ANTHROPIC_AUTH_TOKEN -> Required for Claude Code 
# ANTHROPIC_API_KEY="" -> Required for Claude Code (Must be empty)
# Source: https://openrouter.ai/docs/guides/guides/claude-code-integration
ANTHROPIC_DEFAULT_SONNET_MODEL="anthropic/claude-haiku-4.5" \
  ANTHROPIC_DEFAULT_OPUS_MODEL="anthropic/claude-haiku-4.5" \
  ANTHROPIC_DEFAULT_HAIKU_MODEL="anthropic/claude-haiku-4.5" \
  ANTHROPIC_BASE_URL="http://localhost:10000/api" \
  ANTHROPIC_AUTH_TOKEN="dummy-api-key" \
  ANTHROPIC_API_KEY="" \
  exec claude "$@"
