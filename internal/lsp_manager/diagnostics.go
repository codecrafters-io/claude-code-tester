package lsp_manager

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"sync"
)

type diagnosticReport struct {
	URI         string `json:"uri"`
	Version     *int   `json:"version"`
	Diagnostics []struct {
		Range   Range  `json:"range"`
		Message string `json:"message"`
	} `json:"diagnostics"`
}

type diagnosticIssue struct {
	File     string
	Position string
	Message  string
}

type diagnosticRead struct {
	Reports  []diagnosticReport
	Versions map[string]int
}

type diagnosticRequest struct {
	Body  string
	Reads []diagnosticRead
}

type DiagnosticRecorder struct {
	mu       sync.Mutex
	versions map[string]int
	reads    []diagnosticRead
	requests []diagnosticRequest
	cursor   int
	err      error
}

func (r *DiagnosticRecorder) RoundTrip(request *http.Request) (*http.Response, error) {
	action := path.Base(request.URL.Path)
	if request.Method != http.MethodPost || (action != "notify" && action != "notifications") {
		return http.DefaultTransport.RoundTrip(request)
	}
	body, err := io.ReadAll(request.Body)
	if err != nil {
		return nil, err
	}
	request.Body.Close()
	request.Body = io.NopCloser(bytes.NewReader(body))
	response, err := http.DefaultTransport.RoundTrip(request)
	if err != nil || response.StatusCode < 200 || response.StatusCode >= 300 {
		return response, err
	}
	result, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil {
		return nil, err
	}
	response.Body = io.NopCloser(bytes.NewReader(result))
	r.record(action, body, result)
	return response, nil
}

func (r *DiagnosticRecorder) record(action string, body, result []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return
	}
	if action == "notify" {
		var notification struct {
			Method string `json:"method"`
			Params struct {
				Document Document `json:"textDocument"`
			} `json:"params"`
		}
		if r.err = json.Unmarshal(body, &notification); r.err != nil {
			return
		}
		if notification.Method == "textDocument/didOpen" || notification.Method == "textDocument/didChange" {
			if r.versions == nil {
				r.versions = map[string]int{}
			}
			r.versions[notification.Params.Document.URI] = notification.Params.Document.Version
		}
		return
	}
	var poll struct {
		After int `json:"after"`
	}
	if r.err = json.Unmarshal(body, &poll); r.err != nil {
		return
	}
	if poll.After < 0 || poll.After > r.cursor {
		r.err = fmt.Errorf("notification cursor %d skips unread entries after %d", poll.After, r.cursor)
		return
	}
	var batch struct {
		Cursor        int `json:"cursor"`
		Notifications []struct {
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		} `json:"notifications"`
	}
	if r.err = json.Unmarshal(result, &batch); r.err != nil {
		return
	}
	if batch.Cursor < poll.After || batch.Cursor-poll.After != len(batch.Notifications) {
		r.err = fmt.Errorf("notification response cursor does not match its batch")
		return
	}
	read := diagnosticRead{Versions: map[string]int{}}
	for uri, version := range r.versions {
		read.Versions[uri] = version
	}
	for _, notification := range batch.Notifications[min(r.cursor-poll.After, len(batch.Notifications)):] {
		if notification.Method == "textDocument/publishDiagnostics" {
			var report diagnosticReport
			if r.err = json.Unmarshal(notification.Params, &report); r.err != nil {
				return
			}
			read.Reports = append(read.Reports, report)
		}
	}
	r.cursor = max(r.cursor, batch.Cursor)
	r.reads = append(r.reads, read)
}

func (r *DiagnosticRecorder) RecordModelRequest(body string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.requests = append(r.requests, diagnosticRequest{Body: body, Reads: r.reads})
	r.reads = nil
}

func (r *DiagnosticRecorder) Check(root string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return fmt.Errorf("recording diagnostic feedback: %w", r.err)
	}
	shown := map[string]map[diagnosticIssue]bool{}
	known := map[diagnosticIssue]bool{}
	supportedMessages := map[string]bool{}
	previousResults := map[string]string{}
	find := func(text string, issue diagnosticIssue) []diagnosticTextSpan {
		return parseDiagnosticOutput(text, known).find(issue)
	}
	for index, request := range r.requests {
		if index > 0 && len(request.Reads) == 0 && request.Body != r.requests[index-1].Body {
			return fmt.Errorf("read queued notifications before each model request")
		}
		latest := map[string]diagnosticReport{}
		for _, read := range request.Reads {
			for uri, report := range latest {
				version, opened := read.Versions[uri]
				if report.Version != nil && (!opened || *report.Version != version) {
					delete(latest, uri)
				}
			}
			for _, report := range read.Reports {
				for _, diagnostic := range report.Diagnostics {
					known[diagnosticIssue{relativeFile(root, report.URI), onePosition(diagnostic.Range.Start), diagnostic.Message}] = true
				}
				version, opened := read.Versions[report.URI]
				if report.Version != nil && (!opened || *report.Version != version) {
					continue
				}
				current := map[diagnosticIssue]bool{}
				for _, diagnostic := range report.Diagnostics {
					current[diagnosticIssue{relativeFile(root, report.URI), onePosition(diagnostic.Range.Start), diagnostic.Message}] = true
				}
				for issue := range shown[report.URI] {
					if !current[issue] {
						delete(shown[report.URI], issue)
					}
				}
				latest[report.URI] = report
			}
		}
		expected := map[diagnosticIssue]bool{}
		for uri, report := range latest {
			current := map[diagnosticIssue]bool{}
			for _, diagnostic := range report.Diagnostics {
				issue := diagnosticIssue{relativeFile(root, uri), onePosition(diagnostic.Range.Start), diagnostic.Message}
				current[issue] = true
				supportedMessages[issue.Message] = true
				if !shown[uri][issue] {
					expected[issue] = true
				}
			}
			shown[uri] = current
		}
		var body struct {
			Messages []struct {
				Role    string          `json:"role"`
				ID      string          `json:"tool_call_id"`
				Content json.RawMessage `json:"content"`
			} `json:"messages"`
		}
		if err := json.Unmarshal([]byte(request.Body), &body); err != nil {
			return fmt.Errorf("reading model request for diagnostic feedback: %w", err)
		}
		var additions []string
		latestResult := ""
		for _, message := range body.Messages {
			if message.Role != "tool" {
				continue
			}
			var content string
			if json.Unmarshal(message.Content, &content) != nil {
				content = string(message.Content)
			}
			text := content
			if previous, exists := previousResults[message.ID]; exists {
				text = strings.TrimPrefix(text, previous)
			}
			previousResults[message.ID] = content
			latestResult = text
			additions = append(additions, text)
		}
		for issue := range expected {
			if len(find(latestResult, issue)) == 0 {
				return fmt.Errorf("the model never saw the server's diagnostic for %s at %s (1-based): %q; append new diagnostics to the latest tool result before the next model request", issue.File, issue.Position, issue.Message)
			}
		}
		for _, text := range additions {
			var allowed []diagnosticTextSpan
			for issue := range expected {
				allowed = append(allowed, find(text, issue)...)
			}
			for issue := range known {
				if expected[issue] {
					continue
				}
				matches := find(text, issue)
				if !supportedMessages[issue.Message] && issue.Message != "" {
					for offset := 0; offset < len(text); {
						index := strings.Index(text[offset:], issue.Message)
						if index < 0 {
							break
						}
						start := offset + index
						offset = start + len(issue.Message)
						matches = append(matches, diagnosticTextSpan{start, offset})
					}
				}
				for _, match := range matches {
					covered := false
					for _, valid := range allowed {
						if match.start >= valid.start && match.end <= valid.end {
							covered = true
							break
						}
					}
					if !covered {
						return fmt.Errorf("a stale diagnostic reached the model or an unchanged diagnostic was repeated: %s at %s: %q", issue.File, issue.Position, issue.Message)
					}
				}
			}
		}
	}
	return nil
}
