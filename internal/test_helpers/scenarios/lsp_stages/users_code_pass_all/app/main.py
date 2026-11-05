"""Reference agent for the LSP extension. Uses only the Python standard library."""

import argparse
import json
import os
from pathlib import Path
import subprocess
import sys
from lsp_client import LSPClient
import urllib.parse
import urllib.request


class LanguageServer:
    def __init__(self):
        self.transport = LSPClient()
        self.documents = {}
        self.cursor = 0
        self.shown = {}
        self.languages = self.transport.call("configuration", {})["extensionToLanguage"]

    def send(self, method, params):
        return self.transport.notify(method, params)

    def request(self, method, params):
        return self.transport.request(method, params)

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

    def write(self, path, content):
        path = Path(path).resolve()
        uri = path.as_uri()
        path.write_text(content, encoding="utf-8")
        if uri in self.documents:
            self.documents[uri] += 1
            self.send("textDocument/didChange", {
                "textDocument": {"uri": uri, "version": self.documents[uri]},
                "contentChanges": [{"text": content}],
            })
        elif path.suffix in self.languages:
            self.open(path)
        return "Wrote " + relative(path)

    def new_diagnostics(self):
        """Drain queued notifications and return a block of issues the model has not seen yet."""
        batch = self.transport.notifications(self.cursor, 0)
        self.cursor = batch["cursor"]
        latest = {}
        for notification in batch["notifications"]:
            if notification["method"] != "textDocument/publishDiagnostics":
                continue
            params = notification["params"]
            if params.get("version", self.documents.get(params["uri"])) != self.documents.get(params["uri"]):
                continue
            latest[params["uri"]] = params["diagnostics"]
        sections = []
        for uri, diagnostics in latest.items():
            issues = [
                f"  \u2718 [Line {d['range']['start']['line'] + 1}:{d['range']['start']['character'] + 1}] {d['message']}"
                for d in diagnostics
            ]
            lines = [issue for issue in issues if issue not in self.shown.get(uri, set())]
            self.shown[uri] = set(issues)
            if lines:
                sections.append(relative(uri_path(uri)) + ":\n" + "\n".join(lines))
        if not sections:
            return ""
        return "\n\n<new-diagnostics>The following new diagnostic issues were detected:\n\n" + "\n\n".join(sections) + "</new-diagnostics>"

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
        if isinstance(result, dict):
            result = [result]
        locations = [(relative(uri_path(item["uri"])), item["range"]["start"]) for item in result or []]
        if operation == "goToDefinition":
            if not locations:
                return "No definition found."
            file, start = locations[0]
            return f"Defined in {file}:{start['line'] + 1}:{start['character'] + 1}"
        if not locations:
            return "No references found."
        files = {}
        for file, start in locations:
            files.setdefault(file, []).append(f"  Line {start['line'] + 1}:{start['character'] + 1}")
        sections = [file + ":\n" + "\n".join(lines) for file, lines in sorted(files.items())]
        return f"Found {len(locations)} references across {len(files)} files:\n\n" + "\n\n".join(sections)


def uri_path(uri):
    return Path(urllib.request.url2pathname(urllib.parse.urlparse(uri).path))


def relative(path):
    return os.path.relpath(path, os.getcwd()).replace(os.sep, "/")


TOOLS = [
    {"type": "function", "function": {"name": "LSP", "description": "Find definitions and references of a symbol at a given file position. Positions are 1-based line and character, as shown in editors.", "parameters": {
        "type": "object", "properties": {
            "operation": {"type": "string", "enum": ["goToDefinition", "findReferences"]},
            "filePath": {"type": "string"}, "line": {"type": "integer", "minimum": 1}, "character": {"type": "integer", "minimum": 1},
        }, "required": ["operation", "filePath", "line", "character"],
    }}},
    {"type": "function", "function": {"name": "Read", "description": "Returns file contents", "parameters": {
        "type": "object", "properties": {"file_path": {"type": "string"}}, "required": ["file_path"],
    }}},
    {"type": "function", "function": {"name": "Write", "description": "Writes a file and synchronizes open documents", "parameters": {
        "type": "object", "properties": {"file_path": {"type": "string"}, "content": {"type": "string"}},
        "required": ["file_path", "content"],
    }}},
    {"type": "function", "function": {"name": "Bash", "description": "Returns command output", "parameters": {
        "type": "object", "properties": {"command": {"type": "string"}}, "required": ["command"],
    }}},
]


def run_agent(prompt, server):
    messages = [{"role": "system", "content": "Use the LSP tool for code navigation. Positions are 1-based. Use Write for edits. Fix the issues reported in new diagnostics."},
                {"role": "user", "content": prompt}]
    for _ in range(50):
        diagnostics = server.new_diagnostics()
        if diagnostics and messages[-1]["role"] == "tool":
            messages[-1]["content"] += diagnostics
        payload = {"model": "anthropic/claude-haiku-4.5", "messages": messages, "tools": TOOLS}
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
                    result = server.write(arguments["file_path"], arguments["content"])
                elif name == "Read":
                    result = Path(arguments["file_path"]).read_text(encoding="utf-8")
                elif name == "Bash":
                    output = subprocess.run(arguments["command"], shell=True, capture_output=True, text=True, timeout=10)
                    result = {"stdout": output.stdout, "stderr": output.stderr, "exit_code": output.returncode}
                else:
                    raise ValueError("Unknown tool: " + name)
            except Exception as error:
                result = {"error": str(error)}
            content = result if isinstance(result, str) else json.dumps(result)
            messages.append({"role": "tool", "tool_call_id": call["id"], "content": content})
    raise RuntimeError("Agent exceeded 50 model requests")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("-p", required=True)
    arguments = parser.parse_args()
    server = LanguageServer()
    print(run_agent(arguments.p, server))


if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        print(str(error), file=sys.stderr)
        sys.exit(1)
