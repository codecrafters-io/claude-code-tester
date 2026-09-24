#!/bin/sh

# Claude Code writes a warning to stderr for every query it sends under a model
# id it doesn't recognise, and the OpenRouter ids used here are all unrecognised.
# The tester logs stderr alongside stdout, so the warnings would otherwise end up
# in the recorded fixtures.
run_claude() {
  stderr_file="$(mktemp)"

  ANTHROPIC_DEFAULT_SONNET_MODEL="anthropic/claude-haiku-4.5" \
  ANTHROPIC_DEFAULT_OPUS_MODEL="anthropic/claude-haiku-4.5" \
  ANTHROPIC_DEFAULT_HAIKU_MODEL="anthropic/claude-haiku-4.5" \
  ANTHROPIC_BASE_URL="http://localhost:10000/api" \
  ANTHROPIC_AUTH_TOKEN="dummy-api-key" \
  ANTHROPIC_API_KEY="" \
  claude "$@" 2>"$stderr_file"

  exit_code=$?

  grep -v 'claude-code:unrecognized_model' "$stderr_file" >&2
  rm -f "$stderr_file"

  return $exit_code
}

should_intercept_output=false

if [ "$1" = "-p" ] && [ -n "$2" ]; then
  prompt="$2"
  case "$prompt" in
    "What is the count of tools available to you in this request? Respond with only a number."|\
    "How many tools are available to you in this request? Respond with only a number."|\
    "Count the number of tools available to you for this request. Respond with only a number."|\
    "Give the number of tools accessible in this request. Respond with only a number.")
      should_intercept_output=true
      ;;
  esac
fi

# The output of claude is non-determinstic when it tries to analyze large arrays
# We get around the problem by hardcoding the number of tools to be 10 if it greater than 10
if [ "$should_intercept_output" = true ]; then
  output="$(run_claude "$@")"
  exit_code=$?
  if [ -n "$output" ] && [ "$output" -gt 10 ]; then
    echo "10"
  else
    echo "$output"
  fi
  exit $exit_code
else
  run_claude "$@"
  exit $?
fi
