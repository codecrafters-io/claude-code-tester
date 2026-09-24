package skills_manager

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/codecrafters-io/claude-code-tester/internal/workspace_manager"
	"github.com/codecrafters-io/tester-utils/logger"
	"github.com/codecrafters-io/tester-utils/random"
)

// skillNameWords are used as skill directory names.
//
// No word here is a substring of another, so an assertion naming one skill can
// never be satisfied or tripped by a different skill appearing in the output.
var skillNameWords = []string{
	"atlas",
	"beacon",
	"cinder",
	"falcon",
	"harbor",
	"jasper",
	"kestrel",
	"lumen",
	"nimbus",
	"onyx",
	"quartz",
	"zephyr",
}

// tokenWords are used as unguessable values hidden inside skill bodies.
//
// They are disjoint from skillNameWords so that asserting on a token can never
// be satisfied by a skill name leaking into the output, and no word here is a
// substring of another, so the stacking stage's "both tokens appear" assertion
// can't be satisfied by one token alone.
var tokenWords = []string{
	"tangerine",
	"clementine",
	"persimmon",
	"nectarine",
	"kumquat",
	"damson",
	"physalis",
	"soursop",
	"jujube",
	"medlar",
}

type Skill struct {
	Name        string
	Description string
	Body        string

	// RunInForkedContext writes `context: fork` into the frontmatter, asking the
	// agent to run this skill's body in a conversation of its own and bring only
	// the result back.
	RunInForkedContext bool
}

// RandomNames returns n distinct skill directory names.
func RandomNames(n int) []string {
	return random.RandomElementsFromArray(skillNameWords, n)
}

// RandomTokens returns n distinct words for hiding inside skill bodies.
func RandomTokens(n int) []string {
	return random.RandomElementsFromArray(tokenWords, n)
}

// RespondWithTokenBody returns a body that instructs the model to emit token verbatim.
func RespondWithTokenBody(token string) string {
	return fmt.Sprintf("Respond with exactly one word: %s\n\n%s.", token, RespondWithTokenBodyMarker)
}

// RespondWithTokenBodyMarker is the part of RespondWithTokenBody that never
// shows up in the answer, which is what lets the fork stage tell a request
// carrying the body apart from one carrying only the result.
const RespondWithTokenBodyMarker = "Do not add any other text, punctuation, or formatting"

// StackedLine pairs a skill's token with the argument its invocation carried.
//
// Asserting on the pair rather than the token alone is what makes one value
// prove two things: that the skill's body was loaded, and that the argument
// reached it.
func StackedLine(token string, arguments string) string {
	return fmt.Sprintf("%s-%s", token, arguments)
}

// StackedLineBody returns a body that contributes one line to the response.
//
// Bodies phrased as "respond with exactly X" cannot be stacked, since two of
// them in one context are contradictory instructions. This one is additive, so
// several skills can be satisfied by a single response.
func StackedLineBody(token string) string {
	return fmt.Sprintf("Add this exact line to your response: %s\n\nDo not modify the line.", StackedLine(token, "$ARGUMENTS"))
}

func (s Skill) DirPath() string {
	return filepath.Join(".claude", "skills", s.Name)
}

func (s Skill) MarkdownPath() string {
	return filepath.Join(s.DirPath(), "SKILL.md")
}

func (s Skill) ScriptPath(scriptFileName string) string {
	return filepath.Join(s.DirPath(), "scripts", scriptFileName)
}

// DataFilePath is the skill's own folder, one level up from the script, which
// is where the `..` in ChecksumScriptContents lands.
func (s Skill) DataFilePath() string {
	return filepath.Join(s.DirPath(), DataFileName)
}

// ScriptReference is how a skill body refers to one of its bundled scripts: a
// relative path from the skill's own folder, as the Agent Skills standard
// prescribes. Forward slashes and no filepath.Join: this is literal text inside
// the body, not a path the tester resolves.
func ScriptReference(scriptFileName string) string {
	return "scripts/" + scriptFileName
}

func (s Skill) markdownContents() string {
	var builder strings.Builder

	builder.WriteString("---\n")
	builder.WriteString(fmt.Sprintf("name: %s\n", s.Name))
	builder.WriteString(fmt.Sprintf("description: %s\n", s.Description))

	if s.RunInForkedContext {
		builder.WriteString("context: fork\n")
	}

	builder.WriteString("---\n\n")
	builder.WriteString(s.Body)
	builder.WriteString("\n")

	return builder.String()
}

func (s Skill) asWorkspaceFile() workspace_manager.WorkspaceFile {
	return workspace_manager.WorkspaceFile{
		RelativePath: s.MarkdownPath(),
		Content:      s.markdownContents(),
		FileMode:     0644,
	}
}

// Seed writes a SKILL.md for each skill into the workspace's .claude/skills directory.
func Seed(workspaceManager *workspace_manager.WorkspaceManager, skills []Skill, logger *logger.Logger) {
	files := make([]workspace_manager.WorkspaceFile, 0, len(skills))

	for _, skill := range skills {
		files = append(files, skill.asWorkspaceFile())
	}

	workspaceManager.MustCreateFilesWithLogger(files, logger)
}
