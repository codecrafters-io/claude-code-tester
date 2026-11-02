"""Reference agent for the LSP extension. Uses only the Python standard library."""

import argparse
import json
import os
from pathlib import Path
import subprocess
import sys
import time
from lsp_client import LSPClient
import urllib.request


class LanguageServer:
    def __init__(self):
        self.transport = LSPClient()
        self.documents = {}
        self.diagnostics = {}
        self.diagnostic_ticks = {}
        self.cursor = 0
        self.languages = self.transport.call("configuration", {})["extensionToLanguage"]

    def send(self, method, params):
        return self.transport.notify(method, params)

    def request(self, method, params):
        return self.transport.request(method, params)

    def receive_pending(self):
        self.receive(0)

    def receive(self, timeout_ms=1000):
        batch = self.transport.notifications(self.cursor, timeout_ms)
        self.cursor = batch["cursor"]
        for message in batch["notifications"]:
            if message["method"] != "textDocument/publishDiagnostics":
                continue
            diagnostic = message["params"]
            uri = diagnostic["uri"]
            version = diagnostic.get("version")
            if version is not None and version < self.documents.get(uri, 0):
                continue
            self.diagnostics[uri] = diagnostic
            self.diagnostic_ticks[uri] = self.diagnostic_ticks.get(uri, 0) + 1

    def open(self, path):
        path = Path(path).resolve()
        uri = path.as_uri()
        if uri not in self.documents:
            language = self.languages.get(path.suffix)
            if language is None:
                raise ValueError("Unsupported file extension: " + path.suffix)
            self.documents[uri] = 1
            self.send("textDocument/didOpen", {"textDocument": {
                "uri": uri, "languageId": language, "version": 1,
                "text": path.read_text(encoding="utf-8"),
            }})
        return uri

    def current_diagnostics(self, path, after=0):
        uri = self.open(path)
        version = self.documents[uri]
        deadline = time.monotonic() + 3
        while self.diagnostic_ticks.get(uri, 0) <= after or self.diagnostics.get(uri, {}).get("version", version) != version:
            if time.monotonic() >= deadline:
                raise TimeoutError("Diagnostic feedback error: no fresh publication")
            self.receive()
        return {"uri": uri, "diagnostics": self.diagnostics[uri]["diagnostics"]}

    def write(self, path, content):
        path = Path(path).resolve()
        uri = path.as_uri()
        self.receive_pending()
        before = self.diagnostic_ticks.get(uri, 0)
        path.write_text(content, encoding="utf-8")
        if uri in self.documents:
            self.documents[uri] += 1
            self.send("textDocument/didChange", {
                "textDocument": {"uri": uri, "version": self.documents[uri]},
                "contentChanges": [{"text": content}],
            })
        return {"written": str(path), **self.current_diagnostics(path, after=before)}

    def tool(self, arguments):
        operation = arguments["operation"]
        uri = self.open(arguments["filePath"])
        methods = {"goToDefinition": "definition", "findReferences": "references"}
        params = {"textDocument": {"uri": uri}, "position": {
            "line": arguments["line"] - 1, "character": arguments["character"] - 1,
        }}
        if operation == "findReferences":
            params["context"] = {"includeDeclaration": True}
        result = self.request("textDocument/" + methods[operation], params)
        return result



TOOLS = [
    {"type": "function", "function": {"name": "LSP", "description": "Returns language-server results", "parameters": {
        "type": "object", "properties": {
            "operation": {"type": "string", "enum": ["goToDefinition", "findReferences"]},
            "filePath": {"type": "string"}, "line": {"type": "integer", "minimum": 1}, "character": {"type": "integer", "minimum": 1},
        }, "required": ["operation", "filePath", "line", "character"],
    }}},
    {"type": "function", "function": {"name": "Read", "description": "Returns file contents", "parameters": {
        "type": "object", "properties": {"file_path": {"type": "string"}}, "required": ["file_path"],
    }}},
    {"type": "function", "function": {"name": "Write", "description": "Writes a file and returns diagnostics", "parameters": {
        "type": "object", "properties": {"file_path": {"type": "string"}, "content": {"type": "string"}},
        "required": ["file_path", "content"],
    }}},
    {"type": "function", "function": {"name": "Bash", "description": "Returns command output", "parameters": {
        "type": "object", "properties": {"command": {"type": "string"}}, "required": ["command"],
    }}},
]


def run_agent(prompt, server):
    messages = [{"role": "system", "content": "Use the LSP tool for code navigation. Tool positions are one-based UTF-16. Use Write for edits."},
                {"role": "user", "content": prompt}]
    for _ in range(50):
        payload = {"model": "anthropic/claude-haiku-4.5", "messages": messages, "tools": [tool for tool in TOOLS if server or tool["function"]["name"] != "LSP"]}
        request = urllib.request.Request(
            os.environ["OPENROUTER_BASE_URL"].rstrip("/") + "/chat/completions",
            data=json.dumps(payload).encode("utf-8"),
            headers={"Content-Type": "application/json", "Authorization": "Bearer " + os.environ["OPENROUTER_API_KEY"]},
        )
        try:
            with urllib.request.urlopen(request, timeout=30) as response:
                message = json.load(response)["choices"][0]["message"]
        except urllib.error.HTTPError as error:
            detail = error.read().decode("utf-8", errors="replace").strip()
            raise RuntimeError(f"Model request failed ({error.code}): {detail}") from error
        messages.append(message)
        if not message.get("tool_calls"):
            return message.get("content", "")
        for call in message["tool_calls"]:
            arguments = json.loads(call["function"]["arguments"])
            try:
                name = call["function"]["name"]
                if name == "LSP":
                    result = server.tool(arguments)
                elif name == "Write":
                    if server:
                        result = server.write(arguments["file_path"], arguments["content"])
                    else:
                        Path(arguments["file_path"]).write_text(arguments["content"],encoding="utf-8")
                        result = {"written":arguments["file_path"]}
                elif name == "Read":
                    result = Path(arguments["file_path"]).read_text(encoding="utf-8")
                elif name == "Bash":
                    output = subprocess.run(arguments["command"], shell=True, capture_output=True, text=True, timeout=10)
                    result = {"stdout": output.stdout, "stderr": output.stderr, "exit_code": output.returncode}
                else:
                    raise ValueError("Unknown tool: " + name)
            except Exception as error:
                result = {"error": str(error)}
            messages.append({"role": "tool", "tool_call_id": call["id"], "content": json.dumps(result)})
    raise RuntimeError("Agent exceeded 50 model requests")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("-p", required=True)
    arguments = parser.parse_args()
    server = LanguageServer() if os.environ.get("CODECRAFTERS_LSP_URL") else None
    print(run_agent(arguments.p, server))


if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        print(str(error), file=sys.stderr)
        sys.exit(1)
