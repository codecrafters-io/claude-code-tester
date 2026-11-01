package lsp_manager

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type Config struct {
	Stage              string `json:"stage"`
	Root               string `json:"root"`
	Log                string `json:"log"`
	Token              string `json:"token"`
	Fragmented         bool   `json:"fragmented"`
	Fault              string `json:"fault,omitempty"`
	Hover              string `json:"hover,omitempty"`
	Definition         string `json:"definition,omitempty"`
	EmptyReferences    bool   `json:"emptyReferences,omitempty"`
	StaleDiagnostics   bool   `json:"staleDiagnostics,omitempty"`
	MissingDiagnostics bool   `json:"missingDiagnostics,omitempty"`
	ReverseEdits       bool   `json:"reverseEdits,omitempty"`
}

type Event struct {
	Direction string  `json:"direction"`
	Message   Message `json:"message"`
	PID       int     `json:"pid"`
	Time      int64   `json:"time"`
	Detail    string  `json:"detail,omitempty"`
}

func appendEvent(path, direction string, message Message, detail string) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	return json.NewEncoder(file).Encode(Event{Direction: direction, Message: message, PID: os.Getpid(), Time: time.Now().UnixNano(), Detail: detail})
}

func Events(path string) ([]Event, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	var events []Event
	for {
		var event Event
		if err := decoder.Decode(&event); err == io.EOF {
			return events, nil
		} else if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
}

func Serve(config Config, input io.Reader, output io.Writer) error {
	state := projectState{Root: config.Root, Documents: map[string]Document{}}
	initialized, ready, shutdown := false, false, false
	defer appendEvent(config.Log, "ended", Message{}, "")
	send := func(value any) error {
		body, _ := json.Marshal(value)
		var message Message
		json.Unmarshal(body, &message)
		if err := appendEvent(config.Log, "sent", message, ""); err != nil {
			return err
		}
		return WriteMessage(output, value, config.Fragmented)
	}
	notify := func(method string, params any) error {
		return send(map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
	}
	reader := bufio.NewReader(input)
	for {
		message, err := ReadMessage(reader)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			appendEvent(config.Log, "failure", Message{}, err.Error())
			return err
		}
		if err := appendEvent(config.Log, "received", message, ""); err != nil {
			return err
		}
		var params struct {
			RootURI      string `json:"rootUri"`
			ProcessID    *int   `json:"processId"`
			Capabilities struct {
				TextDocument struct {
					PublishDiagnostics struct {
						VersionSupport bool `json:"versionSupport"`
					} `json:"publishDiagnostics"`
				} `json:"textDocument"`
			} `json:"capabilities"`
			TextDocument Document `json:"textDocument"`
			Position     Position `json:"position"`
			Context      struct {
				IncludeDeclaration bool `json:"includeDeclaration"`
			} `json:"context"`
			ContentChanges []struct {
				Text string `json:"text"`
			} `json:"contentChanges"`
			NewName string `json:"newName"`
		}
		json.Unmarshal(message.Params, &params)
		result := any(nil)
		var protocolError any
		switch message.Method {
		case "initialize":
			cwd, _ := os.Getwd()
			if initialized || params.RootURI != URI(config.Root) || params.ProcessID == nil || *params.ProcessID != os.Getppid() || filepath.Clean(cwd) != filepath.Clean(config.Root) {
				protocolError = map[string]any{"code": -32602, "message": "initialize must identify the client process and project directory"}
			} else if (config.Stage == "dg6" || config.Stage == "rn7") && !params.Capabilities.TextDocument.PublishDiagnostics.VersionSupport {
				protocolError = map[string]any{"code": -32602, "message": "advertise publishDiagnostics.versionSupport before requesting diagnostics"}
			} else if config.Fault == "initialize-error" {
				protocolError = map[string]any{"code": -32603, "message": "Initialization failed"}
			} else {
				initialized = true
				if config.Fault == "incomplete" {
					_, err := io.WriteString(output, "Content-Length: 100\r\n\r\n{\"jsonrpc\":")
					return err
				}
				if config.Fragmented {
					if err := notify("window/logMessage", map[string]any{"type": 3, "message": "Starting language server: caf\u00e9"}); err != nil {
						return err
					}
				}
				result = map[string]any{"capabilities": map[string]any{"textDocumentSync": map[string]any{"openClose": true, "change": 1}, "hoverProvider": true, "definitionProvider": true, "referencesProvider": true, "renameProvider": true}}
			}
		case "initialized":
			ready = initialized
		case "shutdown":
			shutdown = true
		case "exit":
			if !shutdown {
				return fmt.Errorf("exit arrived before shutdown")
			}
			return nil
		default:
			if !ready {
				return fmt.Errorf("%s arrived before initialized", message.Method)
			}
			uri := params.TextDocument.URI
			path, err := Path(uri)
			if err != nil {
				return err
			}
			relative, err := filepath.Rel(config.Root, path)
			if err != nil || strings.HasPrefix(relative, "..") {
				return fmt.Errorf("document outside project: %s", uri)
			}
			switch message.Method {
			case "textDocument/didOpen":
				if _, exists := state.Documents[uri]; exists {
					return fmt.Errorf("duplicate didOpen for %s", uri)
				}
				if params.TextDocument.Version != 1 || params.TextDocument.LanguageID != "python" {
					return fmt.Errorf("didOpen requires version 1 and languageId python")
				}
				contents, err := os.ReadFile(path)
				if err != nil || string(contents) != params.TextDocument.Text {
					return fmt.Errorf("didOpen text differs from %s", relative)
				}
				state.Documents[uri] = params.TextDocument
			case "textDocument/didChange":
				old, exists := state.Documents[uri]
				if !exists || params.TextDocument.Version <= old.Version || len(params.ContentChanges) != 1 {
					return fmt.Errorf("didChange requires an open document, a newer version, and one full-text change")
				}
				old.Text, old.Version = params.ContentChanges[0].Text, params.TextDocument.Version
				contents, err := os.ReadFile(path)
				if err != nil || string(contents) != old.Text {
					return fmt.Errorf("didChange text differs from the successful write to %s", relative)
				}
				state.Documents[uri] = old
			case "textDocument/didClose":
				delete(state.Documents, uri)
			case "textDocument/hover", "textDocument/definition", "textDocument/references", "textDocument/rename":
				if _, exists := state.Documents[uri]; !exists {
					return fmt.Errorf("open %s before querying it", relative)
				}
				signature := state.signature()
				switch message.Method {
				case "textDocument/hover":
					if config.Hover == "error" {
						protocolError = map[string]any{"code": -32603, "message": "Hover unavailable"}
					} else if config.Hover != "null" && len(signature) > 0 {
						result = map[string]any{"contents": map[string]any{"kind": "plaintext", "value": fmt.Sprintf("(price: %s, quantity: int) -> %s\nDocumentation word: %s", signature[2], signature[3], config.Token)}}
					}
				case "textDocument/definition":
					if config.Definition != "null" {
						result = state.definition()
						if config.Definition == "array" {
							result = []any{result, Location{URI: URI(filepath.Join(config.Root, "pricing.pyi")), Range: Range{Start: Position{Line: 2, Character: 4}, End: Position{Line: 2, Character: 9}}}}
						}
					}
				case "textDocument/references":
					if config.EmptyReferences {
						result = []Location{}
					} else {
						locations := state.references(params.Context.IncludeDeclaration, false)
						if params.Context.IncludeDeclaration {
							for i, j := 0, len(locations)-1; i < j; i, j = i+1, j-1 {
								locations[i], locations[j] = locations[j], locations[i]
							}
						}
						result = locations
					}
				case "textDocument/rename":
					if len(signature) > 0 && params.NewName != signature[1] {
						if !regexp.MustCompile(`^[A-Za-z_][A-Za-z_0-9]*$`).MatchString(params.NewName) {
							return fmt.Errorf("invalid newName")
						}
						changes := state.rename(params.NewName)
						if config.ReverseEdits {
							for uri, edits := range changes {
								for i, j := 0, len(edits)-1; i < j; i, j = i+1, j-1 {
									edits[i], edits[j] = edits[j], edits[i]
								}
								changes[uri] = edits
							}
						}
						result = map[string]any{"changes": changes}
					}
				}
			default:
				protocolError = map[string]any{"code": -32601, "message": "Method not found"}
			}
			if (message.Method == "textDocument/didOpen" || message.Method == "textDocument/didChange") && !config.MissingDiagnostics {
				document := state.Documents[uri]
				if err := notify("textDocument/publishDiagnostics", map[string]any{"uri": uri, "version": document.Version, "diagnostics": state.diagnostics(document)}); err != nil {
					return err
				}
				if config.StaleDiagnostics && document.Version > 1 {
					if err := notify("textDocument/publishDiagnostics", map[string]any{"uri": uri, "version": document.Version - 1, "diagnostics": []map[string]any{{"message": "Stale diagnostic must be ignored"}}}); err != nil {
						return err
					}
				}
			}
		}
		if len(message.ID) > 0 {
			reply := map[string]any{"jsonrpc": "2.0", "id": message.ID}
			if protocolError != nil {
				reply["error"] = protocolError
			} else {
				reply["result"] = result
			}
			if err := send(reply); err != nil {
				return err
			}
		}
	}
}

func RunServer(configPath string) int {
	contents, err := os.ReadFile(configPath)
	var config Config
	if err == nil {
		err = json.Unmarshal(contents, &config)
	}
	if err == nil {
		err = Serve(config, os.Stdin, os.Stdout)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
