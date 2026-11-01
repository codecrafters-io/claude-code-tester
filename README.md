# Claude Code Challenge Tester

This is a program that validates your progress on the "Build your own Claude
Code" challenge.

## Requirements for binary

- Following environment variables:
  - `CODECRAFTERS_SUBMISSION_DIR` - root of the user's code submission
  - `CODECRAFTERS_TEST_CASES_JSON` - test cases in JSON format

## User code requirements

- A binary named `your_program.sh` that executes the program.
- A file named `codecrafters.yml`, with the following values:
  - `debug`

## LSP extension

Requires Go 1.25.5 and Python 3.10 or newer. The reference agent uses the Python
standard library. The tester starts its stdio language-server fixture from the
same compiled binary, so no Python language server is required for grading.

Run the tests, including recorded stage output and deliberately broken clients:

```sh
make test_lsp
```

Run all eight stages against the reference agent, with a deterministic local
model that emits tool calls:

```sh
make test_lsp_offline
```

The agent still performs its real HTTP/tool loop and talks to the stdio server.
These checks cover 26 cases: initialization, partial frames, null/error results,
aliases, document versions, diagnostics, and multi-file UTF-16 rename edits.
They include a sending-only stage-one client and six rejected broken clients.
Each LSP case starts its own model endpoint on an OS-assigned loopback port. The
submission receives that endpoint through `OPENROUTER_BASE_URL`, keeping
requests from earlier tests out of the current recording.

To run the same stages with the course's live model, set
`CODECRAFTERS_SECRET_OPENROUTER_API_KEY` and run:

```sh
make test_lsp_with_users_code
```

`make test` includes this live reference scenario alongside the existing base
and Skills scenarios. All three use the existing model proxy. LSP output is
compared to the same fixture as the offline run, with model wording and
temporary configuration paths removed. To regenerate it, run
`make record_lsp_fixtures`. Live-model trials have not been verified locally
because no API key is configured.

For a separate real-server smoke check, install a pinned Pyright outside the
repo:

```sh
npm install --prefix /tmp/lsp-pyright --ignore-scripts pyright@1.1.414
make test_lsp_with_pyright PYRIGHT_LANGSERVER=/tmp/lsp-pyright/node_modules/.bin/pyright-langserver
```

This checks initialization, hover, imported-alias definition, references, rename
responses, and shutdown. It does not run the graded diagnostics or apply rename
edits. Pyright may omit diagnostic versions and treats an alias call as a use of
the local alias. Its rename response can use `documentChanges`, beyond the
course client's `changes` subset. The controlled fixture intentionally supplies
versioned publications after each open/change and includes that call in its
reference set. The reference client ignores unversioned publications and reports
a bounded error when current-version diagnostics are unavailable. This smoke
check does not claim the course subset supports every language server.

## CI and repository conventions

`make test` runs the entire suite without Go's test cache, as Redis's tester
does. `make test_offline` runs protocol checks, fixture comparisons, and unit
tests. `make test_live` runs the existing `TestStages` scenarios against
OpenRouter. Both groups run as separate CI jobs, so an invalid model credential
cannot hide an offline protocol regression. No live tests are skipped or allowed
to fail.

The base and Skills Python scenarios pin Python 3.14 through `.python-version`,
matching the course starter. This prevents uv from selecting Python 3.15 when
the locked native dependencies only support Python through 3.14.

The tester follows Redis's slug-to-Go-function registration, synchronized course
YAML, scenario fixtures, reference implementation, teardown, and flakiness-check
patterns. LSP uses a controlled stdio fixture for repeatable protocol
assertions; the separate live-model and Pyright checks retain their documented
scope.
