.PHONY: release build test test_offline test_live test_with_bash copy_course_file

# Skill stacking needs 2.1.199 and the $0/$1 argument shorthand landed after
# 2.1.14, so the skills stages fail against anything older.
claude_code_version := 2.1.266

current_version_number := $(shell git tag --list "v*" | sort -V | tail -n 1 | cut -c 2-)
next_version_number := $(shell echo $$(($(current_version_number)+1)))

release:
	git tag v$(next_version_number)
	git push origin main v$(next_version_number)

build:
	go build -o dist/main.out ./cmd/tester

test:
	TESTER_DIR=$(shell pwd) go test -count=1 -v ./...

# Keep deterministic protocol checks visible when live-model credentials fail.
test_offline:
	TESTER_DIR=$(shell pwd) go test -count=1 -v -skip '^TestStages$$' ./...

test_live:
	CODECRAFTERS_LSP_LIVE=1 TESTER_DIR=$(shell pwd) go test -count=1 -v -run '^TestStages$$' ./internal

test_without_local_caching:
	go clean -testcache
	TESTER_DIR=$(shell pwd) go test -count=1 -v ./...

test_and_watch:
	onchange '**/*' -- go test -v ./internal/

record_fixtures:
	CODECRAFTERS_RECORD_FIXTURES=true make test

TEST_TARGET ?= test_without_local_caching
RUNS ?= 100
test_flakiness:
	@$(foreach i,$(shell seq 1 $(RUNS)), \
		echo "Running iteration $(i)/$(RUNS) of \"make $(TEST_TARGET)\"" ; \
		make $(TEST_TARGET) > /tmp/test ; \
		if [ "$$?" -ne 0 ]; then \
			echo "Test failed on iteration $(i)" ; \
			cat /tmp/test ; \
			exit 1 ; \
		fi ;\
	)

# Base stages test cases
define BASE_STAGES
[{\"slug\":\"yy2\",\"tester_log_prefix\":\"stage-1\",\"title\":\"Stage #1: Communicate with LLM\"},{\"slug\":\"aq1\",\"tester_log_prefix\":\"stage-2\",\"title\":\"Stage #2: Advertise Read Tool\"},{\"slug\":\"md6\",\"tester_log_prefix\":\"stage-3\",\"title\":\"Stage #3: Execute Read Tool\"},{\"slug\":\"ff2\",\"tester_log_prefix\":\"stage-5\",\"title\":\"Stage #5: Agent Loop\"},{\"slug\":\"oz7\",\"tester_log_prefix\":\"stage-6\",\"title\":\"Stage #6: Write Tool\"},{\"slug\":\"oq5\",\"tester_log_prefix\":\"stage-8\",\"title\":\"Stage #8: Bash Tool\"}]
endef

# Skills extension test cases
define SKILLS_STAGES
[{\"slug\":\"vh1\",\"tester_log_prefix\":\"stage-1\",\"title\":\"Stage #1: Advertise skills to the LLM\"},{\"slug\":\"jd8\",\"tester_log_prefix\":\"stage-2\",\"title\":\"Stage #2: Invoke a skill by name\"},{\"slug\":\"wd2\",\"tester_log_prefix\":\"stage-3\",\"title\":\"Stage #3: Pass arguments to a skill\"},{\"slug\":\"sk5\",\"tester_log_prefix\":\"stage-4\",\"title\":\"Stage #4: Stack multiple skills\"},{\"slug\":\"tq1\",\"tester_log_prefix\":\"stage-5\",\"title\":\"Stage #5: Run a script bundled with a skill\"},{\"slug\":\"gq2\",\"tester_log_prefix\":\"stage-6\",\"title\":\"Stage #6: Let the model choose a skill\"},{\"slug\":\"mj2\",\"tester_log_prefix\":\"stage-7\",\"title\":\"Stage #7: Run a skill in a subagent\"}]
endef

# Reusable test runner
define run_test
	CODECRAFTERS_REPOSITORY_DIR=$(shell pwd)/$(1) \
	CODECRAFTERS_TEST_CASES_JSON="$(2)" \
	dist/main.out
endef


test_base_with_claude_code: build
	$(call run_test,internal/test_helpers/pass_all,$(BASE_STAGES))

test_base_with_users_code: build
	$(call run_test,internal/test_helpers/scenarios/base_stages/users_code_pass_all,$(BASE_STAGES))

test_skills_with_users_code: build
	$(call run_test,internal/test_helpers/scenarios/skills_stages/users_code_pass_all,$(SKILLS_STAGES))

test_skills_with_claude_code: build
	$(call run_test,internal/test_helpers/pass_all,$(SKILLS_STAGES))

test_all_with_claude_code: build
	make test_base_with_claude_code || true
	make test_skills_with_claude_code || true

copy_course_file:
	hub api \
		repos/codecrafters-io/build-your-own-claude-code/contents/course-definition.yml \
		| jq -r .content \
		| base64 -d \
		> internal/test_helpers/course_definition.yml

update_tester_utils:
	go get -u github.com/codecrafters-io/tester-utils

setup:
	echo "Setting up claude-code-tester prerequisites for Linux"

	echo "Installing Claude Code Version $(claude_code_version)"
	curl -fsSL https://claude.ai/install.sh | bash -s -- $(claude_code_version)
	echo "Claude Code is now installed"

	echo "Installing uv"
	curl -LsSf https://astral.sh/uv/install.sh | sh
	echo "$$HOME/.local/bin" >> $$GITHUB_PATH
	echo "uv is now installed!"

	echo "Setup complete!"

# LSP extension test cases
define LSP_STAGES
[{\"slug\":\"dz5\",\"tester_log_prefix\":\"stage-1\",\"title\":\"Stage #1: Connect the LSP client\"},{\"slug\":\"iy3\",\"tester_log_prefix\":\"stage-2\",\"title\":\"Stage #2: Advertise the LSP tool\"},{\"slug\":\"gz6\",\"tester_log_prefix\":\"stage-3\",\"title\":\"Stage #3: Find the implementation\"},{\"slug\":\"rb6\",\"tester_log_prefix\":\"stage-4\",\"title\":\"Stage #4: Find the affected callers\"},{\"slug\":\"bn2\",\"tester_log_prefix\":\"stage-5\",\"title\":\"Stage #5: Keep the analysis current\"},{\"slug\":\"ql9\",\"tester_log_prefix\":\"stage-6\",\"title\":\"Stage #6: Change an API across files\"}]
endef

.PHONY: test_lsp_offline test_lsp_with_users_code record_lsp_fixtures

# Uses the same stage checks with a deterministic model, without API credentials.
test_lsp_offline: build
	CODECRAFTERS_LSP_LIVE= OPENROUTER_BASE_URL=http://localhost:10000/api/v1 OPENROUTER_API_KEY=dummy-api-key $(MAKE) run_lsp_reference

# Uses the existing OpenRouter proxy and CODECRAFTERS_SECRET_OPENROUTER_API_KEY.
test_lsp_with_users_code: build
	CODECRAFTERS_LSP_LIVE=1 OPENROUTER_BASE_URL=http://localhost:10000/api/v1 OPENROUTER_API_KEY=dummy-api-key $(MAKE) run_lsp_reference

run_lsp_reference:
	$(call run_test,internal/test_helpers/scenarios/lsp_stages/users_code_pass_all,$(LSP_STAGES))

record_lsp_fixtures:
	CODECRAFTERS_RECORD_FIXTURES=true TESTER_DIR=$(shell pwd) go test -count=1 -run '^TestLSPStagesOffline$$' ./internal

.PHONY: test_lsp test_lsp_with_pyright run_lsp_reference

# Recorded stage fixtures, negative clients, YAML registration and fixture unit tests.
test_lsp:
	TESTER_DIR=$(shell pwd) go test -count=1 -run '^Test(LSP|StagesMatchYAML)' ./internal
	go test -count=1 ./internal/lsp_manager ./internal/lsp_transport ./proxy_server

PYRIGHT_LANGSERVER ?= pyright-langserver
test_lsp_with_pyright: build
	CODECRAFTERS_LSP_TRANSPORT=$(shell pwd)/dist/main.out python3 internal/test_helpers/scenarios/lsp_stages/pyright_smoke.py --server "$(PYRIGHT_LANGSERVER)"

# Independent product check, with a scripted local Anthropic endpoint and Pyright.
CLAUDE_CODE_BINARY ?= claude
.PHONY: test_lsp_with_claude_code
test_lsp_with_claude_code:
	python3 internal/test_helpers/scenarios/lsp_stages/claude_code_smoke.py --claude "$(CLAUDE_CODE_BINARY)" --server "$(PYRIGHT_LANGSERVER)"
