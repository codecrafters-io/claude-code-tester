package internal

import (
	"os"
	"regexp"
	"testing"

	tester_utils_testing "github.com/codecrafters-io/tester-utils/testing"
)

var skillsStageSlugs = []string{"vh1", "jd8", "wd2", "sk5", "tq1", "gq2", "mj2"}

// Claude Code expands a stack of skills only in an interactive session. Under
// `-p` it expands the first skill and hands the rest of the line to it as
// literal argument text, so sk5 can't run against the real CLI.
var skillsStageSlugsRealClaudeCode = []string{"vh1", "jd8", "wd2", "tq1", "gq2", "mj2"}

func TestStages(t *testing.T) {
	os.Setenv("CODECRAFTERS_RANDOM_SEED", "1234567890")
	os.Setenv("OPENROUTER_BASE_URL", "http://localhost:10000/api/v1")
	os.Setenv("OPENROUTER_API_KEY", "dummy-api-key")

	testCases := map[string]tester_utils_testing.TesterOutputTestCase{
		"base_stages_pass_all": {
			StageSlugs:          []string{"yy2", "aq1", "md6", "ff2", "oz7", "oq5"},
			CodePath:            "./test_helpers/pass_all",
			ExpectedExitCode:    0,
			StdoutFixturePath:   "./test_helpers/fixtures/base_stages/success",
			NormalizeOutputFunc: normalizeTesterOutput,
		},
		"base_stages_stage_1_fail": {
			StageSlugs:          []string{"yy2"},
			CodePath:            "./test_helpers/scenarios/base_stages/stage_1_fail",
			ExpectedExitCode:    1,
			StdoutFixturePath:   "./test_helpers/fixtures/base_stages/stage_1_fail",
			NormalizeOutputFunc: normalizeTesterOutput,
		},
		"base_stages_users_code_pass_all": {
			StageSlugs:          []string{"yy2", "aq1", "md6", "ff2", "oz7", "oq5"},
			CodePath:            "./test_helpers/scenarios/base_stages/users_code_pass_all",
			ExpectedExitCode:    0,
			StdoutFixturePath:   "./test_helpers/fixtures/base_stages/users_code_pass_all",
			NormalizeOutputFunc: normalizeTesterOutput,
		},
		"base_stages_wrong_model_usage": {
			StageSlugs:          []string{"yy2"},
			CodePath:            "./test_helpers/scenarios/base_stages/wrong_model_usage",
			ExpectedExitCode:    1,
			StdoutFixturePath:   "./test_helpers/fixtures/base_stages/wrong_model_usage",
			NormalizeOutputFunc: normalizeTesterOutput,
		},
		"base_stages_unauthorized_endpoint_access": {
			StageSlugs:          []string{"yy2"},
			CodePath:            "./test_helpers/scenarios/base_stages/unauthorized_endpoint_access",
			ExpectedExitCode:    1,
			StdoutFixturePath:   "./test_helpers/fixtures/base_stages/unauthorized_endpoint_access",
			NormalizeOutputFunc: normalizeTesterOutput,
		},
		"base_stages_responses_api_pass": {
			StageSlugs:          []string{"yy2"},
			CodePath:            "./test_helpers/scenarios/base_stages/responses_api_pass",
			ExpectedExitCode:    0,
			StdoutFixturePath:   "./test_helpers/fixtures/base_stages/responses_api_pass",
			NormalizeOutputFunc: normalizeTesterOutput,
		},
		"skills_stages_pass_all": {
			StageSlugs:          skillsStageSlugsRealClaudeCode,
			CodePath:            "./test_helpers/pass_all",
			ExpectedExitCode:    0,
			StdoutFixturePath:   "./test_helpers/fixtures/skills_stages/success",
			NormalizeOutputFunc: normalizeTesterOutput,
		},
		"skills_stages_users_code_pass_all": {
			StageSlugs:          skillsStageSlugs,
			CodePath:            "./test_helpers/scenarios/skills_stages/users_code_pass_all",
			ExpectedExitCode:    0,
			StdoutFixturePath:   "./test_helpers/fixtures/skills_stages/users_code_pass_all",
			NormalizeOutputFunc: normalizeTesterOutput,
		},
	}

	tester_utils_testing.TestTesterOutput(t, testerDefinition, testCases)
}

// A line the user's program printed, with any colour codes that precede it.
var userProgramLinePattern = regexp.MustCompile(`(?m)^(?:\x1b\[[0-9;]*m)*\[your_program\] .*\n?`)

// The request tally the fork stage logs, whose value is however many tool calls
// the model decided to make on the way to its answer.
var requestCountPattern = regexp.MustCompile(`sent \d+ request\(s\)`)

// normalizeTesterOutput removes what a live model decided and keeps what the
// tester did about it. Every scenario here drives a real model, so the program's
// wording and the number of calls it takes differ from one recording to the next
// even when it behaves identically, and a fixture pinning either would report a
// failure over the model's choices. What the program printed is already covered
// by the stage assertions, whose verdicts stay in the fixture.
func normalizeTesterOutput(testerOutput []byte) []byte {
	normalized := userProgramLinePattern.ReplaceAll(testerOutput, nil)

	return requestCountPattern.ReplaceAll(normalized, []byte("sent N request(s)"))
}
