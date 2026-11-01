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

The seven graded stages stay within Claude Code's built-in LSP capabilities.
The finale lets the agent catch its own mistake: a bad Write returns a type
error, the model uses that feedback to choose a repair, and the next Write
clears the error. The scripted model refuses to select the repair without an
error diagnostic from the preceding Write. Rename is outside this extension.
Official allocation of current stage IDs is deferred at the course author's
request.

The reference agent uses Python's standard library. The controlled stdio fixture
runs from the tester binary, so grading does not require installing Pyright.

```sh
make test_lsp
make test_lsp_offline
```

These commands cover 23 stage cases and eight deliberately broken clients:
missing initialization, wrong coordinate conversion, omitted declarations,
discarded hover results, unsynchronized writes, discarded unversioned reports,
reused cached unversioned reports, and false empty diagnostics after a timeout. Stage output is recorded in the
existing fixture format. Each case gets a private loopback model endpoint via
OPENROUTER_BASE_URL, keeping unrelated traffic out of its recording.

The LSP tool uses Claude Code's hover, goToDefinition and findReferences names,
filePath, and one-based editor coordinates. JSON-RPC positions remain zero-based.
The project configuration uses the product's named server map with command,
args and extensionToLanguage. Plugin installation and discovery are outside the
course. Diagnostic feedback uses the base course's Write tool-result channel;
Claude Code can deliver it as a separate conversation attachment.

To regenerate recorded stage output:

```sh
make record_lsp_fixtures
```

For live-model trials, configure CODECRAFTERS_SECRET_OPENROUTER_API_KEY and run:

```sh
make test_lsp_with_users_code
```

The earlier CI credential received HTTP 401. No valid key is configured locally.
Live-model reliability, 100-run live flakiness checks and model-backed prose lint
remain unverified. Offline or scripted checks do not establish those results.

### Independent implementation checks

Install the pinned Python language server outside the repository:

```sh
npm install --prefix /tmp/lsp-pyright --ignore-scripts pyright@1.1.414
make test_lsp_with_pyright PYRIGHT_LANGSERVER=/tmp/lsp-pyright/node_modules/.bin/pyright-langserver
```

This runs the reference client against Pyright through initialization, hover,
alias definition, references, a bad write, diagnostic feedback, repair, clearing
and shutdown. It enables basic type checking in the temporary project. The
client accepts unversioned publications instead of rejecting Pyright's reports.
An unversioned publication cannot identify which edit was analyzed; the course
fixture supplies one response per open/change for deterministic grading.

The second check runs the actual CI-pinned Claude Code 2.1.266 against Pyright,
using a temporary plugin and a local scripted Anthropic endpoint:

```sh
make test_lsp_with_claude_code CLAUDE_CODE_BINARY=/path/to/claude-2.1.266 PYRIGHT_LANGSERVER=/tmp/lsp-pyright/node_modules/.bin/pyright-langserver
```

It verifies protocol traffic, one-based-to-zero-based coordinate conversion,
includeDeclaration true, file synchronization, type-error delivery to the model,
and a completed repair. It makes no paid model calls and uses a temporary config
directory. The stage runner prints the verified edit error and diagnostic
clearing after a successful repair, so the payoff is visible in learner logs.
Bash remains available, matching the completed base course: the pinned
CLI suppresses diagnostic attachments when both Bash and PowerShell are absent.
The scripted model only calls navigation and file tools.

This native check compares the features against an independent implementation;
it does not claim that Claude Code's formatted output passes the learner
fixture's structured JSON assertions. The two checks run in a dedicated CI job.

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
