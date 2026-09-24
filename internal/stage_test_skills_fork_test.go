package internal

import (
	"fmt"
	"testing"

	"github.com/codecrafters-io/claude-code-tester/internal/skills_manager"
	"github.com/codecrafters-io/tester-utils/logger"
	"github.com/stretchr/testify/assert"
)

// The stage's whole verdict rests on this assertion, so pin the line it draws
// using the request shapes each kind of submission produces.
func TestForkAssertionSeparatesForkingFromInlineSubmissions(t *testing.T) {
	const token = "blueberry"
	const question = "What is the database migration status?"

	catalog := `"role":"system","content":"You have access to the following skills:\n\n- apple: ...\n- grape: ..."`
	asked := fmt.Sprintf(`"role":"user","content":"%s"`, question)
	body := fmt.Sprintf(`"role":"user","content":"Respond with exactly one word: %s\n\n%s."`, token, skills_manager.RespondWithTokenBodyMarker)

	testCases := map[string]struct {
		requestBodies []string
		shouldPass    bool
	}{
		"forks, so the subagent never sees the question": {
			requestBodies: []string{
				"{" + catalog + "," + asked + "}",
				"{" + body + "}",
			},
			shouldPass: true,
		},
		// Claude Code shows the subagent the catalog too. The question is still
		// absent from it, which is what the assertion keys on.
		"forks, and the subagent also sees the catalog": {
			requestBodies: []string{
				"{" + catalog + "," + asked + "}",
				"{" + catalog + "," + body + "}",
			},
			shouldPass: true,
		},
		"ignores the field and continues the main conversation": {
			requestBodies: []string{
				"{" + catalog + "," + asked + "}",
				"{" + catalog + "," + asked + "," + body + "}",
			},
			shouldPass: false,
		},
		"starts a second conversation but copies the question into it": {
			requestBodies: []string{
				"{" + catalog + "," + asked + "}",
				"{" + asked + "," + body + "}",
			},
			shouldPass: false,
		},
		"never runs the skill at all": {
			requestBodies: []string{
				"{" + catalog + "," + asked + "}",
			},
			shouldPass: false,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			err := forkAssertion(question).Run(testCase.requestBodies, logger.GetQuietLogger(""))

			if testCase.shouldPass {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}
