package skills_manager

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/codecrafters-io/tester-utils/random"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	// RandomNames/RandomTokens draw from the package-level rng, which panics
	// until Init has been called. The tester binary does this in its CLI entrypoint.
	random.Init()
	os.Exit(m.Run())
}

func TestMarkdownContents(t *testing.T) {
	skill := Skill{
		Name:        "atlas",
		Description: "Deploys the billing service to production.",
		Body:        "Respond with exactly one word: tangerine",
	}

	expected := strings.Join([]string{
		"---",
		"name: atlas",
		"description: Deploys the billing service to production.",
		"---",
		"",
		"Respond with exactly one word: tangerine",
		"",
	}, "\n")

	assert.Equal(t, expected, skill.markdownContents())
}

// The fork stage's whole subject is one frontmatter field, so the seeded file
// has to carry it verbatim. TestMarkdownContents above is the other half: it
// asserts on the full contents, so it fails if the field is ever written
// unconditionally.
func TestMarkdownContentsWithForkedContext(t *testing.T) {
	skill := Skill{
		Name:               "atlas",
		Description:        "Deploys the billing service to production.",
		Body:               "Respond with exactly one word: tangerine",
		RunInForkedContext: true,
	}

	expected := strings.Join([]string{
		"---",
		"name: atlas",
		"description: Deploys the billing service to production.",
		"context: fork",
		"---",
		"",
		"Respond with exactly one word: tangerine",
		"",
	}, "\n")

	assert.Equal(t, expected, skill.markdownContents())
}

func TestPaths(t *testing.T) {
	skill := Skill{Name: "atlas"}

	assert.Equal(t, ".claude/skills/atlas/SKILL.md", skill.MarkdownPath())
	assert.Equal(t, ".claude/skills/atlas/scripts/checksum.sh", skill.ScriptPath(ChecksumScriptFileName))
}

// The script stage seeds a file at ScriptPath but the body names it with a
// relative reference. Resolving that reference against the skill's own folder
// has to land on the seeded file, or the stage fails for a reason that has
// nothing to do with the user's code.
func TestScriptReferenceResolvesToScriptPath(t *testing.T) {
	skill := Skill{Name: "atlas"}

	resolved := filepath.Join(skill.DirPath(), ScriptReference(ChecksumScriptFileName))

	assert.Equal(t, skill.ScriptPath(ChecksumScriptFileName), resolved)
}

// The stacking stage seeds bodies built by StackedLineBody and then asserts on
// StackedLine. Substituting $ARGUMENTS the way the user's program will has to
// land on exactly the value the assertion looks for.
func TestStackedLineBodyYieldsStackedLineAfterSubstitution(t *testing.T) {
	body := StackedLineBody("tangerine")

	substituted := strings.ReplaceAll(body, "$ARGUMENTS", "4127")

	assert.Contains(t, substituted, StackedLine("tangerine", "4127"))
}

// Stages assert on one skill by name while others are seeded alongside it. If
// any name were a substring of another, a different skill appearing in the
// output could satisfy or trip that assertion.
func TestNoSkillNameIsASubstringOfAnother(t *testing.T) {
	assertNoValueIsASubstringOfAnother(t, skillNameWords)
}

// The stacking stage asserts that both invoked skills' tokens appear in one
// response. If one token were a substring of another, a program that expanded
// only the first invocation could still pass.
func TestNoTokenIsASubstringOfAnother(t *testing.T) {
	assertNoValueIsASubstringOfAnother(t, tokenWords)
}

// Tokens are asserted on exactly, and names are asserted on for absence. Any
// overlap between the two lists would make those assertions ambiguous.
func TestTokensAndSkillNamesAreDisjoint(t *testing.T) {
	for _, token := range tokenWords {
		for _, name := range skillNameWords {
			assert.NotContains(t, token, name, "token %q contains skill name %q", token, name)
			assert.NotContains(t, name, token, "skill name %q contains token %q", name, token)
		}
	}
}

// Every stage seeds distinct skills, so drawing n names must never repeat one.
func TestRandomNamesAndTokensAreDistinct(t *testing.T) {
	assertNoDuplicates(t, RandomNames(len(skillNameWords)))
	assertNoDuplicates(t, RandomTokens(len(tokenWords)))
}

// Topic descriptions must not leak the skill's name, otherwise the advertise
// stage could be passed by matching the question against the name alone.
func TestTopicDescriptionsNeverMentionASkillName(t *testing.T) {
	allTopics := append(append([]Topic{}, descriptionTopics...), invocationTopics...)

	for _, topic := range allTopics {
		for _, name := range skillNameWords {
			assert.NotContains(t, strings.ToLower(topic.Description), name, "description %q mentions %q", topic.Description, name)
		}
	}
}

// Each invocation question must match exactly one topic, or the model-invoked
// stage has no single correct answer.
func TestInvocationQuestionsAreUnique(t *testing.T) {
	questions := make([]string, 0, len(invocationTopics))

	for _, topic := range invocationTopics {
		questions = append(questions, topic.Question)
	}

	assertNoDuplicates(t, questions)
}

// ChecksumOf must agree with `sha256sum <file> | cut -c1-8`. This vector was
// produced by piping the same bytes through shasum -a 256.
func TestChecksumOfMatchesShellEquivalent(t *testing.T) {
	assert.Equal(t, "c94e3754", ChecksumOf("apple orange banana pear grape mango"))
}

func TestChecksumScriptReferencesTheDataFile(t *testing.T) {
	assert.Contains(t, ChecksumScriptContents, fmt.Sprintf("sha256sum %s | cut -c1-8", DataFileName))
}

func assertNoValueIsASubstringOfAnother(t *testing.T, values []string) {
	t.Helper()

	for _, outer := range values {
		for _, inner := range values {
			if outer == inner {
				continue
			}

			assert.NotContains(t, outer, inner, "%q contains %q", outer, inner)
		}
	}
}

func assertNoDuplicates(t *testing.T, values []string) {
	t.Helper()

	seen := map[string]bool{}

	for _, value := range values {
		assert.False(t, seen[value], "duplicate value %q", value)
		seen[value] = true
	}
}
