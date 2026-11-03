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

Six graded stages add navigation and synchronized edits to the existing agent
loop:

1. dz5: Connect the LSP client and read configuration.
2. iy3: Advertise the LSP tool without implementing navigation yet.
3. gz6: Find the implementation with one successful definition lookup and Read.
4. rb6: Find the affected callers.
5. bn2: Keep the analysis current.
6. ql9: Change an API across files.

The supplied transport starts the configured server, handles JSON-RPC framing
and initialization, and queues notifications while requests are running. The
tester provides CODECRAFTERS_LSP_URL to the submitted program. The native starter
clients retrieve extensionToLanguage through the configuration action and call
request, notify, and notifications through that local URL. Document language IDs
come from configuration rather than a Python default.
Every LSP case supplies a connection. The connection stage checks configuration
before the first model request and uses Read to inspect the example; it passes
without an advertised LSP tool. The advertisement stage checks the schema in
every model request and passes before navigation is implemented.
The definition exercise supplies valid Python files and a single location. It
does not grade multiple definitions, missing definitions, or server-error tool
results. Reference lookup later introduces location arrays and empty results.
Synchronization grades successful writes, including repeated versions and writes
before opening a file; write failures and synchronization recovery are outside
that exercise.
The local HTTP facade is course scaffolding; the server still speaks stdio LSP.
Learners construct document messages, track versions, return tool results and
keep the server current after edits. The helper never runs an agent.

The reference agent uses Python's standard library. The controlled stdio fixture
runs from the tester binary, so grading doesn't require installing Pyright.

```sh
make test_lsp
make test_lsp_offline
```

These commands cover 12 graded cases and deliberately broken clients, including
missing or late configuration, missing LSP advertisement, incomplete schemas,
wrong coordinates, lost results, omitted declaration context, and unsynchronized
edits. Separate transport tests cover fragmented startup, notifications during
requests, failed initialization, and process cleanup.

The capstone migrates pricing.total from integer cents to a Price TypedDict,
updates checkout and reporting, repeats navigation, and runs the project tests
through Bash. It requires no diagnostic collection. Final files, protocol
traffic, and matching tool results are checked.

Navigation operations are goToDefinition and findReferences, with filePath and
one-based UTF-16 coordinates. Protocol positions are zero-based. Configuration
uses the product's named command/args/extensionToLanguage map. Plugin management,
hover stages and rename are outside the extension. The two setup IDs dz5/iy3 come from the slugs supplied on 3 November 2026.
Later stages retain gz6/rb6/bn2/ql9. The combined setup ID cx7 and retired
ls1/ir2/hv2 IDs are not reallocated.

Regenerate recorded output when prompts or cases change:

```sh
make record_lsp_fixtures
```

For live-model trials, configure CODECRAFTERS_SECRET_OPENROUTER_API_KEY and run
`make test_lsp_with_users_code`. Live-model reliability, 100-run live flakiness
checks and learner pacing trials remain release gates. Offline scripted checks
do not establish those results.

### Independent implementation checks

Install a pinned server outside the repository, then run:

```sh
npm install --prefix /tmp/lsp-pyright --ignore-scripts pyright@1.1.414
make test_lsp_with_pyright PYRIGHT_LANGSERVER=/tmp/lsp-pyright/node_modules/.bin/pyright-langserver
make test_lsp_with_claude_code CLAUDE_CODE_BINARY=/path/to/claude-2.1.266 PYRIGHT_LANGSERVER=/tmp/lsp-pyright/node_modules/.bin/pyright-langserver
```

The first check uses the supplied transport and reference integration with real
Pyright. It waits for Pyright's project discovery before asserting workspace
references, checks the target after an edited import, then observes the actual
Price return-type error and diagnostic clearing. The repaired caller returns 500.
The helper transports notifications; the reference agent implements freshness.

The second runs Claude Code 2.1.266 with a temporary plugin, Pyright and a local
scripted Anthropic endpoint. It verifies definition, references, position conversion,
includeDeclaration true, the same API migration, diagnostic delivery and repair.
Bash stays available because this product version suppresses automatic diagnostic
attachments without Bash or PowerShell. The scripted model never invokes a shell.
The native check observes product diagnostic attachments separately from the
learner fixture's structured Write results. Neither check uses paid model calls.

Versioned stale reports are rejected; unversioned reports are accepted in arrival
order and cannot establish which edit was analyzed. Missing feedback is an error.
The fixture publishes once per open/change except in its missing-feedback case;
real servers determine publication timing. Cleared diagnostics do not prove
behavioral correctness.

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
