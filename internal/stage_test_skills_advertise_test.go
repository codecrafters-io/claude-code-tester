package internal

import (
	"testing"

	"github.com/codecrafters-io/claude-code-tester/internal/skills_manager"
	"github.com/codecrafters-io/tester-utils/logger"
	"github.com/stretchr/testify/assert"
)

func TestCatalogAssertionSeparatesAFullCatalogFromAPartialOne(t *testing.T) {
	skills := []skills_manager.Skill{
		{Name: "harbor", Description: "Deploys the billing service to production."},
		{Name: "kestrel", Description: "Generates release notes for the analytics project."},
	}

	testCases := map[string]struct {
		requestBodies []string
		shouldPass    bool
	}{
		"advertises every skill": {
			requestBodies: []string{
				`{"system":"harbor: Deploys the billing service to production.\nkestrel: Generates release notes for the analytics project."}`,
			},
			shouldPass: true,
		},
		"advertises them across a later request too": {
			requestBodies: []string{
				`{"system":"you are an agent"}`,
				`{"system":"harbor: Deploys the billing service to production.\nkestrel: Generates release notes for the analytics project."}`,
			},
			shouldPass: true,
		},
		"stops at the first skill it found": {
			requestBodies: []string{
				`{"system":"harbor: Deploys the billing service to production."}`,
			},
			shouldPass: false,
		},
		"names them all but describes none": {
			requestBodies: []string{`{"system":"harbor\nkestrel"}`},
			shouldPass:    false,
		},
		"splits them across separate requests": {
			requestBodies: []string{
				`{"system":"harbor: Deploys the billing service to production."}`,
				`{"system":"kestrel: Generates release notes for the analytics project."}`,
			},
			shouldPass: false,
		},
		"never reaches the LLM at all": {
			requestBodies: []string{},
			shouldPass:    false,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			err := catalogAssertion(skills).Run(testCase.requestBodies, logger.GetQuietLogger(""))

			if testCase.shouldPass {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}
