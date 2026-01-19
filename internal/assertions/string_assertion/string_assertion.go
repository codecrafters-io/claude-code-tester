package string_assertion

type StringAssertion interface {
	Run(value string) error
}
