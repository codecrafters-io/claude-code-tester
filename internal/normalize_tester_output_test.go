package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeTesterOutputDropsOnlyTheUserProgramsLines(t *testing.T) {
	testCases := map[string]struct {
		testerOutput   string
		expectedOutput string
	}{
		"a plain line from the user's program": {
			testerOutput:   "[tester::#VH1] Running tests\n[your_program] 3\n[tester::#VH1] Test passed\n",
			expectedOutput: "[tester::#VH1] Running tests\n[tester::#VH1] Test passed\n",
		},
		"the colour codes the tester wraps it in": {
			testerOutput:   "\x1b[33m[your_program] \x1b[0m\x1b[0mpersimmon-7442\x1b[0m\n[tester::#SK5] Test passed\n",
			expectedOutput: "[tester::#SK5] Test passed\n",
		},
		"however many lines it printed": {
			testerOutput:   "[your_program] I see.\n[your_program] \n[your_program] Here they are:\n[tester::#SK5] Test passed\n",
			expectedOutput: "[tester::#SK5] Test passed\n",
		},
		"a last line with no newline after it": {
			testerOutput:   "[tester::#VH1] Running tests\n[your_program] 3",
			expectedOutput: "[tester::#VH1] Running tests\n",
		},
		"nothing at all, when the tester said nothing": {
			testerOutput:   "[your_program] 3\n",
			expectedOutput: "",
		},
		"the tally of requests, whatever it came to": {
			testerOutput:   "[tester::#MJ2] User's program sent 7 request(s) to the LLM\n",
			expectedOutput: "[tester::#MJ2] User's program sent N request(s) to the LLM\n",
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, testCase.expectedOutput, string(normalizeTesterOutput([]byte(testCase.testerOutput))))
		})
	}
}

func TestNormalizeTesterOutputKeepsWhatTheTesterPrinted(t *testing.T) {
	testCases := map[string]string{
		"a stage's verdict":                       "\x1b[33m[tester::#VH1] \x1b[0m\x1b[32mTest passed\x1b[0m\n",
		"a failure message quoting the program":   "[tester::#VH1] Expected integer value, got \"[your_program] 3\"\n",
		"a line that only mentions the program":   "[tester::#SK5] The your_program output was empty\n",
		"a line the program printed mid-sentence": "[tester::#SK5] Ran [your_program] twice\n",
		"a count that isn't the request tally":    "[tester::#SK5] Seeded 2 skills alongside the invoked one\n",
	}

	for name, testerOutput := range testCases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, testerOutput, string(normalizeTesterOutput([]byte(testerOutput))))
		})
	}
}
