package lsp_manager

import (
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"
)

type diagnosticTextSpan struct{ start, end int }

type diagnosticEntry struct {
	file     string
	position string
	diagnosticTextSpan
}

type diagnosticOutput struct {
	text    string
	entries []diagnosticEntry
}

var diagnosticFilePattern = regexp.MustCompile(`(?:[[:alnum:]_.-]+/)*[[:alnum:]_.-]+\.[[:alpha:]][[:alnum:]_-]*`)
var diagnosticPositionPattern = regexp.MustCompile(`[0-9]+:[0-9]+`)

func parseDiagnosticOutput(text string, known map[diagnosticIssue]bool) diagnosticOutput {
	masked := []byte(text)
	for issue := range known {
		if issue.Message == "" {
			continue
		}
		for offset := 0; offset < len(text); {
			index := strings.Index(text[offset:], issue.Message)
			if index < 0 {
				break
			}
			start := offset + index
			offset = start + len(issue.Message)
			for index := start; index < offset; index++ {
				if masked[index] != '\n' {
					masked[index] = ' '
				}
			}
		}
	}
	output := diagnosticOutput{text: text}
	file := ""
	offset := 0
	active := -1
	for _, line := range strings.Split(string(masked), "\n") {
		positions := diagnosticPositionPattern.FindAllStringIndex(line, -1)
		if len(positions) == 0 {
			files := diagnosticFilePattern.FindAllStringIndex(line, -1)
			if len(files) > 0 {
				last := files[len(files)-1]
				if strings.Trim(line[last[1]:], " \t\r:*`<>.") == "" {
					file = path.Clean(line[last[0]:last[1]])
					active = -1
				}
			}
			if active >= 0 {
				output.entries[active].end = offset + len(line)
			}
			offset += len(line) + 1
			continue
		}
		boundaries := []int{0}
		for index := 1; index < len(positions); index++ {
			start := positions[index-1][1]
			between := line[start:positions[index][0]]
			boundary := positions[index][0]
			if separator := strings.LastIndexAny(between, ";,"); separator >= 0 {
				boundary = start + separator + 1
			} else if nextFile := diagnosticFilePattern.FindStringIndex(between); nextFile != nil {
				boundary = start + nextFile[0]
			}
			boundaries = append(boundaries, boundary)
		}
		boundaries = append(boundaries, len(line))
		for index, position := range positions {
			start, end := boundaries[index], boundaries[index+1]
			segment := line[start:end]
			files := diagnosticFilePattern.FindAllString(segment, -1)
			if len(files) > 0 {
				file = path.Clean(files[len(files)-1])
			}
			parts := strings.Split(line[position[0]:position[1]], ":")
			row, _ := strconv.Atoi(parts[0])
			column, _ := strconv.Atoi(parts[1])
			output.entries = append(output.entries, diagnosticEntry{file, fmt.Sprintf("%d:%d", row, column), diagnosticTextSpan{offset + start, offset + end}})
			active = len(output.entries) - 1
		}
		offset += len(line) + 1
	}
	return output
}

func (o diagnosticOutput) find(issue diagnosticIssue) []diagnosticTextSpan {
	var matches []diagnosticTextSpan
	for _, entry := range o.entries {
		if entry.file != issue.File || entry.position != issue.Position {
			continue
		}
		for offset := entry.start; offset <= entry.end && issue.Message != ""; {
			index := strings.Index(o.text[offset:entry.end], issue.Message)
			if index < 0 {
				break
			}
			start := offset + index
			offset = start + len(issue.Message)
			matches = append(matches, diagnosticTextSpan{start, offset})
		}
	}
	return matches
}
