package internal

import (
	"os"
	"testing"

	tester_utils_testing "github.com/codecrafters-io/tester-utils/testing"
)

var skillsStageSlugs = []string{"vh1", "jd8", "wd2", "sk5", "tq1", "gq2", "mj2"}

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
			StageSlugs:          skillsStageSlugs,
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

func normalizeTesterOutput(testerOutput []byte) []byte {
	return testerOutput
}
