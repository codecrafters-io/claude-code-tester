package skills_manager

import "github.com/codecrafters-io/tester-utils/random"

// Topic pairs a skill description with a request that unambiguously matches it.
//
// Descriptions never mention the skill's own name. That's deliberate: a model can
// only name the right skill if the description actually reached its context,
// rather than by pattern-matching the name out of the question.
type Topic struct {
	Description string
	Question    string

	// AnswerFormat is a %s template for a body that answers Question, used by
	// the stages whose question is matched to a skill rather than naming it.
	AnswerFormat string
}

// descriptionTopics describe what a skill does. Used where the tester asks the
// model to name the skill that fits a task.
var descriptionTopics = []Topic{
	{
		Description: "Deploys the billing service to production.",
		Question:    "Which skill would help me deploy the billing service? Respond with only the skill name.",
	},
	{
		Description: "Runs the search test suite and reports failures.",
		Question:    "Which skill would help me run the search test suite? Respond with only the skill name.",
	},
	{
		Description: "Generates release notes for the analytics project.",
		Question:    "Which skill would help me generate release notes for the analytics project? Respond with only the skill name.",
	},
	{
		Description: "Rotates credentials for the notification database.",
		Question:    "Which skill would help me rotate credentials for the notification database? Respond with only the skill name.",
	},
	{
		Description: "Summarizes recent incidents for the payments team.",
		Question:    "Which skill would help me summarize recent incidents for the payments team? Respond with only the skill name.",
	},
}

// invocationTopics are phrased as "use this skill when ...", which is what lets
// the model pick a skill without the user naming it. Each Question matches
// exactly one topic in this list.
//
// Every Question asks for a fact that isn't in the workspace. A question about
// the files themselves, such as summarizing a changelog, invites the model to
// go looking instead of reaching for the skill, and it answers that there's no
// changelog there rather than invoking anything.
//
// Each AnswerFormat states the fact the Question asks for. A body that answers
// something else, like "respond with exactly one word: X", reads to the model as
// a skill that doesn't fit the question, so it sets the body aside and goes
// searching the workspace instead.
var invocationTopics = []Topic{
	{
		Description:  "Use this skill when the user asks about the on-call rotation.",
		Question:     "Who is on the on-call rotation right now?",
		AnswerFormat: "The engineer on the on-call rotation right now is %s.",
	},
	{
		Description:  "Use this skill when the user asks which region the billing service runs in.",
		Question:     "Which region does the billing service run in?",
		AnswerFormat: "The billing service runs in the %s region.",
	},
	{
		Description:  "Use this skill when the user asks who owns the payments dashboard.",
		Question:     "Who owns the payments dashboard?",
		AnswerFormat: "The payments dashboard is owned by %s.",
	},
	{
		Description:  "Use this skill when the user asks for the support team's escalation contact.",
		Question:     "Who is the escalation contact for the support team?",
		AnswerFormat: "The escalation contact for the support team is %s.",
	},
}

// RandomDescriptionTopics returns n distinct topics phrased as plain descriptions.
func RandomDescriptionTopics(n int) []Topic {
	return random.RandomElementsFromArray(descriptionTopics, n)
}

// RandomInvocationTopics returns n distinct topics phrased as "use this skill when ...".
func RandomInvocationTopics(n int) []Topic {
	return random.RandomElementsFromArray(invocationTopics, n)
}
