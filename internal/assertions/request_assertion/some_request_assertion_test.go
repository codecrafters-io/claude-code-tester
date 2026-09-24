package request_assertion

import (
	"testing"

	"github.com/codecrafters-io/tester-utils/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var quietLogger = logger.GetQuietLogger("")

// The subagent's conversation is the one that carries its skill's token without
// the main conversation's catalog, so a match has to require both halves at once.
func TestSomeRequestAssertionNeedsBothHalvesInTheSameRequest(t *testing.T) {
	assertion := SomeRequestAssertion{
		ExpectedValues:   []string{"kumquat"},
		UnexpectedValues: []string{"tangerine"},
	}

	// A request holding only the forked token, alongside one holding both.
	require.NoError(t, assertion.Run([]string{"tangerine and kumquat", "kumquat alone"}, quietLogger))

	// The same two values, but never separated: this is what not forking looks like.
	require.Error(t, assertion.Run([]string{"tangerine and kumquat"}, quietLogger))
}

func TestSomeRequestAssertionRequiresEveryExpectedValue(t *testing.T) {
	assertion := SomeRequestAssertion{ExpectedValues: []string{"tangerine", "kumquat"}}

	require.NoError(t, assertion.Run([]string{"tangerine", "tangerine and kumquat"}, quietLogger))

	// Both values present, but split across requests rather than sharing one.
	require.Error(t, assertion.Run([]string{"tangerine", "kumquat"}, quietLogger))
}

func TestSomeRequestAssertionIsCaseInsensitive(t *testing.T) {
	assertion := SomeRequestAssertion{
		ExpectedValues:   []string{"kumquat"},
		UnexpectedValues: []string{"tangerine"},
	}

	require.NoError(t, assertion.Run([]string{"KUMQUAT"}, quietLogger))
	require.Error(t, assertion.Run([]string{"KUMQUAT TANGERINE"}, quietLogger))
}

func TestSomeRequestAssertionFailsWhenNothingWasRecorded(t *testing.T) {
	assertion := SomeRequestAssertion{ExpectedValues: []string{"kumquat"}}

	require.Error(t, assertion.Run(nil, quietLogger))
}

// The failure message is the only thing the user sees, so it has to name the
// values it looked for and say which way each one counted.
func TestSomeRequestAssertionFailureMessageNamesBothHalves(t *testing.T) {
	assertion := SomeRequestAssertion{
		ExpectedValues:   []string{"kumquat"},
		UnexpectedValues: []string{"tangerine"},
	}

	err := assertion.Run([]string{"tangerine and kumquat"}, quietLogger)

	require.Error(t, err)
	assert.Contains(t, err.Error(), `containing "kumquat" but not "tangerine"`)
	assert.Contains(t, err.Error(), "1 request(s)")
}

func TestSomeRequestAssertionFailureMessageOmitsTheNegativeHalfWhenUnused(t *testing.T) {
	assertion := SomeRequestAssertion{ExpectedValues: []string{"kumquat", "tangerine"}}

	err := assertion.Run([]string{"neither"}, quietLogger)

	require.Error(t, err)
	assert.Contains(t, err.Error(), `containing "kumquat" and "tangerine"`)
	assert.NotContains(t, err.Error(), "but not")
}
