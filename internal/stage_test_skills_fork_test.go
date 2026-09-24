package internal

import (
	"fmt"
	"testing"

	"github.com/codecrafters-io/claude-code-tester/internal/skills_manager"
	"github.com/codecrafters-io/tester-utils/logger"
	"github.com/stretchr/testify/assert"
)

// The stage's whole verdict rests on these two assertions, so pin the line they
// draw using the request shapes each kind of submission produces.
func TestForkAssertionsSeparateForkingFromInlineSubmissions(t *testing.T) {
	const token = "blueberry"

	catalog := `"role":"system","content":"You have access to the following skills:\n\n- apple: ...\n- grape: ..."`
	body := fmt.Sprintf(`"role":"user","content":"Respond with exactly one word: %s\n\n%s."`, token, skills_manager.RespondWithTokenBodyMarker)
	invocation := `"role":"user","content":"/apple"`
	result := fmt.Sprintf(`"role":"user","content":"The /apple skill ran in a subagent and returned:\n\n%s"`, token)

	testCases := map[string]struct {
		requestBodies []string
		shouldPass    bool
	}{
		"forks, and brings the result back": {
			requestBodies: []string{
				"{" + body + "}",
				"{" + catalog + "," + result + "}",
			},
			shouldPass: true,
		},
		// Claude Code shows the subagent the catalog too. The body is still
		// absent from the main conversation, which is what the pair keys on.
		"forks, and the subagent also sees the catalog": {
			requestBodies: []string{
				"{" + catalog + "," + body + "}",
				"{" + catalog + "," + result + "}",
			},
			shouldPass: true,
		},
		"ignores the field and runs the body inline": {
			requestBodies: []string{
				"{" + catalog + "," + body + "}",
			},
			shouldPass: false,
		},
		"forks but never tells the main conversation": {
			requestBodies: []string{
				"{" + body + "}",
				"{" + catalog + "," + invocation + "}",
			},
			shouldPass: false,
		},
		"never runs the skill at all": {
			requestBodies: []string{
				"{" + catalog + "," + invocation + "}",
			},
			shouldPass: false,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			bodyAssertion, resultAssertion := forkAssertions(token)
			quietLogger := logger.GetQuietLogger("")

			err := bodyAssertion.Run(testCase.requestBodies, quietLogger)
			if err == nil {
				err = resultAssertion.Run(testCase.requestBodies, quietLogger)
			}

			if testCase.shouldPass {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}
