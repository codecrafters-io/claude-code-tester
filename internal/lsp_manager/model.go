package lsp_manager

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/codecrafters-io/claude-code-tester/proxy_server"
)

type Action struct {
	Operation       string
	File            string
	Line            int
	Character       int
	Content         string
	AfterDiagnostic bool
}

func (a Action) arguments() map[string]any {
	if a.Operation == "write" {
		return map[string]any{"file_path": a.File, "content": a.Content}
	}
	if a.Operation == "read" {
		return map[string]any{"file_path": a.File}
	}
	return map[string]any{"operation": a.Operation, "filePath": a.File, "line": a.Line + 1, "character": a.Character + 1}
}

func protocolMethod(operation string) string {
	switch operation {
	case "goToDefinition":
		return "textDocument/definition"
	case "findReferences":
		return "textDocument/references"
	default:
		return "textDocument/" + operation
	}
}

// ScriptedModel emits tool calls, not tool results. The submitted agent still
// has to run its actual HTTP/tool loop and speak to the actual stdio fixture.
func ScriptedModel(actions []Action) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var body struct {
			Messages []struct {
				Role    string          `json:"role"`
				ID      string          `json:"tool_call_id"`
				Content json.RawMessage `json:"content"`
			} `json:"messages"`
			Tools []struct {
				Function struct {
					Name       string `json:"name"`
					Parameters struct {
						Properties map[string]json.RawMessage `json:"properties"`
					} `json:"parameters"`
				} `json:"function"`
			} `json:"tools"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			http.Error(writer, err.Error(), 400)
			return
		}
		step := 0
		for _, message := range body.Messages {
			if message.Role == "tool" {
				step++
			}
		}
		message := map[string]any{"role": "assistant", "content": "Done."}
		finish := "stop"
		if step < len(actions) {
			action := actions[step]
			if action.AfterDiagnostic {
				hasDiagnostic := false
				for _, message := range body.Messages {
					if message.Role != "tool" || message.ID != fmt.Sprintf("lsp_call_%d", step-1) {
						continue
					}
					var content string
					if json.Unmarshal(message.Content, &content) != nil {
						content = string(message.Content)
					}
					hasDiagnostic = ContainsJSON(content, map[string]any{"severity": 1})
				}
				if !hasDiagnostic {
					http.Error(writer, "Return an error diagnostic from the preceding Write before the model can choose a repair. After synchronizing the file, keep reading publishDiagnostics notifications and return the new list in that Write result. A timeout is a feedback error, not an empty diagnostics list.", http.StatusBadRequest)
					return
				}
			}
			name := ""
			for _, tool := range body.Tools {
				properties := tool.Function.Parameters.Properties
				if action.Operation == "write" && properties["content"] != nil || action.Operation == "read" && properties["file_path"] != nil && properties["content"] == nil || action.Operation != "write" && action.Operation != "read" && properties["operation"] != nil {
					name = tool.Function.Name
					break
				}
			}
			if name == "" {
				http.Error(writer, "Advertise the requested LSP or Write tool", 400)
				return
			}
			arguments := action.arguments()
			// Accept the base course's path spelling as well as file_path.
			for _, tool := range body.Tools {
				if tool.Function.Name == name && tool.Function.Parameters.Properties["file_path"] == nil && tool.Function.Parameters.Properties["path"] != nil {
					arguments["path"] = arguments["file_path"]
					delete(arguments, "file_path")
				}
			}
			encoded, _ := json.Marshal(arguments)
			message["content"] = nil
			message["tool_calls"] = []any{map[string]any{"id": fmt.Sprintf("lsp_call_%d", step), "type": "function", "function": map[string]any{"name": name, "arguments": string(encoded)}}}
			finish = "tool_calls"
		}
		writer.Header().Set("Content-Type", "application/json")
		json.NewEncoder(writer).Encode(map[string]any{"id": "lsp-offline", "object": "chat.completion", "choices": []any{map[string]any{"index": 0, "message": message, "finish_reason": finish}}})
	})
}

func StartOfflineModel(actions []Action) (*proxy_server.RequestRecorder, string, func(), error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, "", nil, err
	}
	recorder := &proxy_server.RequestRecorder{}
	server := &http.Server{Handler: recorder.WrapHandler(ScriptedModel(actions)), ReadHeaderTimeout: 5 * time.Second}
	go server.Serve(listener)
	return recorder, "http://" + listener.Addr().String() + "/api/v1", func() { server.Close() }, nil
}

// ToolResults retains IDs and result boundaries. Searching concatenated API
// bodies would incorrectly accept a stale result from an older tool call.
func ToolResults(requests []string) map[string]string {
	results := map[string]string{}
	for _, body := range requests {
		var request struct {
			Messages []struct {
				Role    string          `json:"role"`
				ID      string          `json:"tool_call_id"`
				Content json.RawMessage `json:"content"`
			} `json:"messages"`
		}
		if json.Unmarshal([]byte(body), &request) != nil {
			continue
		}
		for _, message := range request.Messages {
			if message.Role != "tool" {
				continue
			}
			var content string
			if json.Unmarshal(message.Content, &content) != nil {
				content = string(message.Content)
			}
			if _, exists := results[message.ID]; !exists {
				results[message.ID] = content
			}
		}
	}
	return results
}

func containsResult(results map[string]string, value string) bool {
	for _, result := range results {
		if strings.Contains(result, value) {
			return true
		}
	}
	return false
}
