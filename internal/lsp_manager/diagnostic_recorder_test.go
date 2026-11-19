package lsp_manager

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/codecrafters-io/claude-code-tester/proxy_server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func recordVersion(t *testing.T, recorder *DiagnosticRecorder, uri string, version int) {
	t.Helper()
	body, err := json.Marshal(map[string]any{"method": "textDocument/didChange", "params": map[string]any{"textDocument": map[string]any{"uri": uri, "version": version}}})
	require.NoError(t, err)
	recorder.record("notify", body, []byte("null"))
}

func diagnosticNotice(uri string, version any, message string) map[string]any {
	diagnostics := []any{}
	if message != "" {
		diagnostics = append(diagnostics, map[string]any{"message": message, "range": Range{Start: Position{Line: 4, Character: 11}, End: Position{Line: 4, Character: 32}}})
	}
	params := map[string]any{"uri": uri, "diagnostics": diagnostics}
	if version != nil {
		params["version"] = version
	}
	return map[string]any{"method": "textDocument/publishDiagnostics", "params": params}
}

func recordRead(t *testing.T, recorder *DiagnosticRecorder, notifications ...any) {
	t.Helper()
	after := recorder.cursor
	batch, err := json.Marshal(map[string]any{"cursor": after + len(notifications), "notifications": notifications})
	require.NoError(t, err)
	poll, err := json.Marshal(map[string]any{"after": after, "timeoutMs": 0})
	require.NoError(t, err)
	recorder.record("notifications", poll, batch)
}

func feedbackRequest(t *testing.T, contents ...string) string {
	t.Helper()
	messages := []any{map[string]any{"role": "user", "content": "Repair the caller"}}
	for index, content := range contents {
		messages = append(messages, map[string]any{"role": "tool", "tool_call_id": fmt.Sprintf("call-%d", index), "content": content})
	}
	body, err := json.Marshal(map[string]any{"messages": messages})
	require.NoError(t, err)
	return string(body)
}

func TestDiagnosticRecorderPreservesReportedHistory(t *testing.T) {
	for _, repeat := range []bool{false, true} {
		t.Run(fmt.Sprintf("repeat-%t", repeat), func(t *testing.T) {
			recorder := &DiagnosticRecorder{}
			uri := "file:///project/checkout.py"
			feedback := "checkout.py 5:12 Current error"
			recordVersion(t, recorder, uri, 1)
			recordRead(t, recorder, diagnosticNotice(uri, 1, "Current error"))
			recorder.RecordModelRequest(feedbackRequest(t, feedback))
			recordVersion(t, recorder, uri, 2)
			recordRead(t, recorder, diagnosticNotice(uri, 1, "Current error"))
			result := "Wrote checkout.py"
			if repeat {
				result += "\n" + feedback
			}
			recorder.RecordModelRequest(feedbackRequest(t, feedback, result))
			err := recorder.Check("/project")
			if repeat {
				require.ErrorContains(t, err, "a stale diagnostic reached the model")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestDiagnosticRecorderUsesLatestReportAcrossReads(t *testing.T) {
	for _, variant := range []string{"clear", "versionless-clear", "newer-version", "stale-after-current"} {
		t.Run(variant, func(t *testing.T) {
			recorder := &DiagnosticRecorder{}
			uri := "file:///project/checkout.py"
			recordVersion(t, recorder, uri, 1)
			recordRead(t, recorder, diagnosticNotice(uri, 1, "Earlier error"))
			content := "Wrote checkout.py"
			switch variant {
			case "clear":
				recordRead(t, recorder, diagnosticNotice(uri, 1, ""))
			case "versionless-clear":
				recordRead(t, recorder, diagnosticNotice(uri, nil, ""))
			case "newer-version":
				recordVersion(t, recorder, uri, 2)
				recordRead(t, recorder, diagnosticNotice(uri, 1, "Earlier error"))
			case "stale-after-current":
				recordRead(t, recorder, diagnosticNotice(uri, 0, "Stale error"))
				content += "\ncheckout.py 5:12 Earlier error"
			}
			recorder.RecordModelRequest(feedbackRequest(t, content))
			require.NoError(t, recorder.Check("/project"))
		})
	}
}

func TestDiagnosticRecorderTracksClearAndReappearance(t *testing.T) {
	for _, clear := range []bool{false, true} {
		t.Run(fmt.Sprintf("clear-%t", clear), func(t *testing.T) {
			recorder := &DiagnosticRecorder{}
			uri := "file:///project/checkout.py"
			feedback := "checkout.py 5:12 Current error"
			recordVersion(t, recorder, uri, 1)
			recordRead(t, recorder, diagnosticNotice(uri, 1, "Current error"))
			recorder.RecordModelRequest(feedbackRequest(t, feedback))
			if clear {
				recordRead(t, recorder, diagnosticNotice(uri, 1, ""))
			} else {
				recordRead(t, recorder)
			}
			recorder.RecordModelRequest(feedbackRequest(t, feedback, "No new issues"))
			recordRead(t, recorder, diagnosticNotice(uri, 1, "Current error"))
			recorder.RecordModelRequest(feedbackRequest(t, feedback, "No new issues", feedback))
			err := recorder.Check("/project")
			if clear {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, "unchanged diagnostic was repeated")
			}
		})
	}
}

func TestDiagnosticRecorderKeepsFilesIndependent(t *testing.T) {
	recorder := &DiagnosticRecorder{}
	checkout := "file:///project/checkout.py"
	reports := "file:///project/reports.py"
	recordVersion(t, recorder, checkout, 2)
	recordVersion(t, recorder, reports, 1)
	recordRead(t, recorder, diagnosticNotice(checkout, 1, "Checkout error"), diagnosticNotice(reports, 1, "Reports error"))
	recorder.RecordModelRequest(feedbackRequest(t, "reports.py 5:12 Reports error"))
	require.NoError(t, recorder.Check("/project"))
}

func TestDiagnosticRecorderIgnoresUnrelatedNotifications(t *testing.T) {
	recorder := &DiagnosticRecorder{}
	recordRead(t, recorder, map[string]any{"method": "custom/progress", "params": []any{"working"}})
	recorder.RecordModelRequest(feedbackRequest(t, "Wrote checkout.py"))
	require.NoError(t, recorder.Check("/project"))
}

func TestDiagnosticRecorderIgnoresReadsAfterFinalRequest(t *testing.T) {
	recorder := &DiagnosticRecorder{}
	recordRead(t, recorder)
	recorder.RecordModelRequest(feedbackRequest(t))
	recordVersion(t, recorder, "file:///project/checkout.py", 1)
	recordRead(t, recorder, diagnosticNotice("file:///project/checkout.py", 1, "Late error"))
	require.NoError(t, recorder.Check("/project"))
}

func TestDiagnosticRecorderRequiresFeedbackInLatestToolResult(t *testing.T) {
	recorder := &DiagnosticRecorder{}
	uri := "file:///project/checkout.py"
	recordVersion(t, recorder, uri, 1)
	recordRead(t, recorder, diagnosticNotice(uri, 1, "Current error"))
	recorder.RecordModelRequest(feedbackRequest(t, "checkout.py 5:12 Current error", "Wrote reports.py"))
	require.ErrorContains(t, recorder.Check("/project"), "append new diagnostics to the latest tool result")
}

func TestDiagnosticRecorderAllowsAugmentingAnExistingToolResult(t *testing.T) {
	recorder := &DiagnosticRecorder{}
	uri := "file:///project/checkout.py"
	recordVersion(t, recorder, uri, 1)
	recordRead(t, recorder)
	recorder.RecordModelRequest(feedbackRequest(t, "Wrote checkout.py"))
	recordRead(t, recorder, diagnosticNotice(uri, 1, "Current error"))
	recorder.RecordModelRequest(feedbackRequest(t, "Wrote checkout.py\ncheckout.py 5:12 Current error"))
	require.NoError(t, recorder.Check("/project"))
}

func TestDiagnosticRecorderRequiresReadingBeforeANewTurn(t *testing.T) {
	recorder := &DiagnosticRecorder{}
	request := feedbackRequest(t)
	recorder.RecordModelRequest(request)
	recorder.RecordModelRequest(request)
	require.NoError(t, recorder.Check("/project"))
	recorder.RecordModelRequest(feedbackRequest(t, "Wrote checkout.py"))
	require.ErrorContains(t, recorder.Check("/project"), "read queued notifications before each model request")
}

func TestDiagnosticRecorderObservesHTTPAndModelRequests(t *testing.T) {
	for _, failedChange := range []bool{false, true} {
		t.Run(fmt.Sprintf("failed-change-%t", failedChange), func(t *testing.T) {
			recorder := &DiagnosticRecorder{}
			uri := "file:///project/checkout.py"
			var forwarded []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				if !assert.NoError(t, err) {
					return
				}
				forwarded = append(forwarded, string(body))
				w.Header().Set("X-Observed", "true")
				if r.URL.Path == "/notify" {
					if failedChange && strings.Contains(string(body), `"version":2`) {
						w.WriteHeader(http.StatusBadGateway)
						fmt.Fprint(w, `{"error":"send failed"}`)
						return
					}
					fmt.Fprint(w, "null")
					return
				}
				notices := []any{diagnosticNotice(uri, 1, "Current error")}
				if !failedChange {
					notices = append(notices, diagnosticNotice(uri, 1, ""))
				}
				assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{"cursor": len(notices), "notifications": notices}))
			}))
			defer server.Close()
			client := &http.Client{Transport: recorder}
			post := func(action, body string) int {
				response, err := client.Post(server.URL+"/"+action, "application/json", strings.NewReader(body))
				require.NoError(t, err)
				defer response.Body.Close()
				data, err := io.ReadAll(response.Body)
				require.NoError(t, err)
				require.True(t, json.Valid(data))
				require.Equal(t, "true", response.Header.Get("X-Observed"))
				require.Equal(t, body, forwarded[len(forwarded)-1])
				return response.StatusCode
			}
			require.Equal(t, http.StatusOK, post("notify", `{"method":"textDocument/didOpen","params":{"textDocument":{"uri":"file:///project/checkout.py","version":1}}}`))
			feedback := "Wrote checkout.py"
			if failedChange {
				require.Equal(t, http.StatusBadGateway, post("notify", `{"method":"textDocument/didChange","params":{"textDocument":{"uri":"file:///project/checkout.py","version":2}}}`))
				feedback += "\ncheckout.py 5:12 Current error"
			}
			require.Equal(t, http.StatusOK, post("notifications", `{"after":0,"timeoutMs":0}`))
			requests := &proxy_server.RequestRecorder{}
			requests.SetObserver(recorder.RecordModelRequest)
			body := feedbackRequest(t, feedback)
			handler := requests.WrapHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				data, err := io.ReadAll(r.Body)
				assert.NoError(t, err)
				assert.Equal(t, body, string(data))
				assert.NoError(t, recorder.Check("/project"))
			}))
			handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/v1/chat/completions", strings.NewReader(body)))
			require.Equal(t, []string{body}, requests.RequestBodies())
			require.NoError(t, recorder.Check("/project"))
		})
	}
}

func TestDiagnosticRecorderKeepsPositionAndFileAssociations(t *testing.T) {
	for _, variant := range []string{"moved-line", "other-file", "wrong-attribution"} {
		t.Run(variant, func(t *testing.T) {
			recorder := &DiagnosticRecorder{}
			checkout := "file:///project/checkout.py"
			reports := "file:///project/reports.py"
			first := "checkout.py 5:12 Shared error"
			recordVersion(t, recorder, checkout, 1)
			recordVersion(t, recorder, reports, 1)
			recordRead(t, recorder, diagnosticNotice(checkout, 1, "Shared error"))
			recorder.RecordModelRequest(feedbackRequest(t, first))
			var content string
			if variant == "moved-line" {
				notice := diagnosticNotice(checkout, 1, "Shared error")
				notice["params"].(map[string]any)["diagnostics"].([]any)[0].(map[string]any)["range"] = Range{Start: Position{Line: 14, Character: 11}}
				recordRead(t, recorder, notice)
				content = "checkout.py 15:12 Shared error"
			} else {
				recordRead(t, recorder, diagnosticNotice(checkout, 1, ""), diagnosticNotice(reports, 1, "Shared error"))
				content = "Wrote checkout.py\nreports.py 5:12 Shared error"
				if variant == "wrong-attribution" {
					content = "Wrote reports.py\ncheckout.py 5:12 Shared error"
				}
			}
			recorder.RecordModelRequest(feedbackRequest(t, first, content))
			err := recorder.Check("/project")
			if variant == "wrong-attribution" {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestDiagnosticRecorderRecognizesReappearanceWithinOneTurn(t *testing.T) {
	for _, sameRead := range []bool{false, true} {
		t.Run(fmt.Sprintf("same-read-%t", sameRead), func(t *testing.T) {
			recorder := &DiagnosticRecorder{}
			uri := "file:///project/checkout.py"
			feedback := "checkout.py 5:12 Current error"
			recordVersion(t, recorder, uri, 1)
			recordRead(t, recorder, diagnosticNotice(uri, 1, "Current error"))
			recorder.RecordModelRequest(feedbackRequest(t, feedback))
			if sameRead {
				recordRead(t, recorder, diagnosticNotice(uri, 1, ""), diagnosticNotice(uri, 1, "Current error"))
			} else {
				recordRead(t, recorder, diagnosticNotice(uri, 1, ""))
				recordRead(t, recorder, diagnosticNotice(uri, 1, "Current error"))
			}
			recorder.RecordModelRequest(feedbackRequest(t, feedback, feedback))
			require.NoError(t, recorder.Check("/project"))
		})
	}
}

func TestDiagnosticRecorderRejectsSkippedNotifications(t *testing.T) {
	recorder := &DiagnosticRecorder{}
	recorder.record("notifications", []byte(`{"after":1,"timeoutMs":0}`), []byte(`{"cursor":1,"notifications":[]}`))
	recorder.RecordModelRequest(feedbackRequest(t))
	require.Error(t, recorder.Check("/project"))
}

func TestDiagnosticRecorderAcceptsNonToolContentBlocks(t *testing.T) {
	recorder := &DiagnosticRecorder{}
	recorder.RecordModelRequest(`{"messages":[{"role":"user","content":[{"type":"text","text":"Repair the caller"}]}]}`)
	require.NoError(t, recorder.Check("/project"))
}

func TestDiagnosticRecorderRejectsUnlocatedStaleMessages(t *testing.T) {
	recorder := &DiagnosticRecorder{}
	uri := "file:///project/checkout.py"
	recordVersion(t, recorder, uri, 2)
	recordRead(t, recorder, diagnosticNotice(uri, 1, "Stale diagnostic must be ignored"))
	recorder.RecordModelRequest(feedbackRequest(t, "Wrote checkout.py\nStale diagnostic must be ignored"))
	require.Error(t, recorder.Check("/project"))
}

func TestDiagnosticRecorderAllowsChangedMessagesWithSharedText(t *testing.T) {
	recorder := &DiagnosticRecorder{}
	uri := "file:///project/checkout.py"
	first := "checkout.py 5:12 Type is incompatible"
	recordVersion(t, recorder, uri, 1)
	recordRead(t, recorder, diagnosticNotice(uri, 1, "Type is incompatible"))
	recorder.RecordModelRequest(feedbackRequest(t, first))
	recordRead(t, recorder, diagnosticNotice(uri, 1, "Type is incompatible with int"))
	recorder.RecordModelRequest(feedbackRequest(t, first, "checkout.py 5:12 Type is incompatible with int"))
	require.NoError(t, recorder.Check("/project"))
}

func TestDiagnosticRecorderAcceptsDiagnosticFormatting(t *testing.T) {
	for _, content := range []string{
		"checkout.py:5:12\n  Current error",
		"[Line 5:12] checkout.py: Current error",
		"checkout.py:\n  [Line 5:12] Current error",
		"Current error at checkout.py 5:12",
		"| checkout.py | 5:12 | Current error |",
	} {
		t.Run(content, func(t *testing.T) {
			recorder := &DiagnosticRecorder{}
			uri := "file:///project/checkout.py"
			recordVersion(t, recorder, uri, 1)
			recordRead(t, recorder, diagnosticNotice(uri, 1, "Current error"))
			recorder.RecordModelRequest(feedbackRequest(t, content))
			require.NoError(t, recorder.Check("/project"))
		})
	}
}

func TestDiagnosticRecorderDoesNotBorrowAnotherEntryMessage(t *testing.T) {
	recorder := &DiagnosticRecorder{}
	checkout := "file:///project/checkout.py"
	reports := "file:///project/reports.py"
	recordVersion(t, recorder, checkout, 1)
	recordVersion(t, recorder, reports, 1)
	recordRead(t, recorder, diagnosticNotice(checkout, 1, "Shared error"), diagnosticNotice(reports, 1, "Shared error"))
	recorder.RecordModelRequest(feedbackRequest(t, "checkout.py 5:12 Shared error; reports.py 5:12 All clear"))
	require.ErrorContains(t, recorder.Check("/project"), "never saw the server's diagnostic for reports.py")
}
