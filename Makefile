.PHONY: release build test test_with_bash copy_course_file

current_version_number := $(shell git tag --list "v*" | sort -V | tail -n 1 | cut -c 2-)
next_version_number := $(shell echo $$(($(current_version_number)+1)))

release:
	git tag v$(next_version_number)
	git push origin main v$(next_version_number)

build:
	go build -o dist/main.out ./cmd/tester

test:
	TESTER_DIR=$(shell pwd) go test -v ./...

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

test_base_with_claude_code: build
	CODECRAFTERS_REPOSITORY_DIR=$(shell pwd)/internal/test_helpers/pass_all \
	CODECRAFTERS_TEST_CASES_JSON="[{\"slug\":\"yy2\",\"tester_log_prefix\":\"stage-1\",\"title\":\"Stage #1: Communicate with LLM\"},{\"slug\":\"aq1\",\"tester_log_prefix\":\"stage-2\",\"title\":\"Stage #2: Advertise Read Tool\"},{\"slug\":\"md6\",\"tester_log_prefix\":\"stage-3\",\"title\":\"Stage #3: Execute Read Tool\"},{\"slug\":\"ff2\",\"tester_log_prefix\":\"stage-5\",\"title\":\"Stage #5: Agent Loop\"},{\"slug\":\"oz7\",\"tester_log_prefix\":\"stage-6\",\"title\":\"Stage #6: Write Tool\"},{\"slug\":\"bp2\",\"tester_log_prefix\":\"stage-7\",\"title\":\"Stage #7: Glob Tool\"},{\"slug\":\"oq5\",\"tester_log_prefix\":\"stage-8\",\"title\":\"Stage #8: Bash Tool\"}]" \
	dist/main.out

test_all: build
	make test_base_with_claude_code || true

copy_course_file:
	hub api \
		repos/codecrafters-io/build-your-own-claude-code/contents/course-definition.yml \
		| jq -r .content \
		| base64 -d \
		> internal/test_helpers/course_definition.yml

update_tester_utils:
	go get -u github.com/codecrafters-io/tester-utils