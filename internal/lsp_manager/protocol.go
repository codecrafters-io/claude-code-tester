package lsp_manager

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf16"
)

type Message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   json.RawMessage `json:"error,omitempty"`
}

func ReadMessage(reader *bufio.Reader) (Message, error) {
	length := -1
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return Message{}, err
		}
		if line == "\r\n" {
			break
		}
		key, value, ok := strings.Cut(strings.TrimSuffix(line, "\r\n"), ":")
		if !ok {
			return Message{}, fmt.Errorf("invalid LSP header %q", line)
		}
		if strings.EqualFold(key, "Content-Length") {
			length, err = strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				return Message{}, fmt.Errorf("invalid Content-Length: %w", err)
			}
		}
	}
	if length < 0 || length > 4*1024*1024 {
		return Message{}, fmt.Errorf("invalid Content-Length %d", length)
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(reader, body); err != nil {
		return Message{}, fmt.Errorf("incomplete LSP body: %w", err)
	}
	var message Message
	if err := json.Unmarshal(body, &message); err != nil {
		return Message{}, err
	}
	if message.JSONRPC != "2.0" {
		return Message{}, fmt.Errorf("expected jsonrpc 2.0")
	}
	return message, nil
}

func WriteMessage(writer io.Writer, value any, fragmented bool) error {
	body, err := json.Marshal(value)
	if err != nil {
		return err
	}
	frame := append([]byte(fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))), body...)
	for len(frame) > 0 {
		size := len(frame)
		if fragmented && size > 7 {
			size = 7
		}
		n, err := writer.Write(frame[:size])
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		frame = frame[n:]
	}
	return nil
}

type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}
type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

func position(text string, offset int) Position {
	prefix := text[:offset]
	line := strings.Count(prefix, "\n")
	column := prefix[strings.LastIndex(prefix, "\n")+1:]
	return Position{Line: line, Character: len(utf16.Encode([]rune(column)))}
}

func textRange(text string, start, end int) Range {
	return Range{Start: position(text, start), End: position(text, end)}
}
