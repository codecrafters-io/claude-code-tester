"""Reference agent for the LSP extension. Uses only the Python standard library."""

import argparse
import json
import os
from pathlib import Path
import queue
import subprocess
import sys
import threading
import urllib.request
from urllib.parse import unquote, urlparse


class LanguageServer:
    def __init__(self, configuration):
        self.process = subprocess.Popen(
            [configuration["command"], *configuration["args"]],
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=None,
        )
        self.language = configuration["languageId"]
        self.incoming = queue.Queue()
        self.next_id = 0
        self.documents = {}
        self.diagnostics = {}
        self.responses = {}
        self.ready = False
        threading.Thread(target=self._read_messages, daemon=True).start()

    def _read_messages(self):
        try:
            while True:
                headers = {}
                while True:
                    line = self.process.stdout.readline()
                    if not line:
                        raise EOFError("Language server closed its output")
                    if line == b"\r\n":
                        break
                    name, value = line.decode("ascii").split(":", 1)
                    headers[name.lower()] = value.strip()
                remaining = int(headers["content-length"])
                chunks = []
                while remaining:
                    chunk = self.process.stdout.read(remaining)
                    if not chunk:
                        raise EOFError("Language server closed an incomplete response")
                    chunks.append(chunk)
                    remaining -= len(chunk)
                self.incoming.put(json.loads(b"".join(chunks)))
        except Exception as error:
            self.incoming.put(error)

    def send(self, method, params=None, request=False):
        message = {"jsonrpc": "2.0", "method": method}
        if params is not None:
            message["params"] = params
        if request:
            self.next_id += 1
            message["id"] = self.next_id
        body = json.dumps(message, ensure_ascii=False, separators=(",", ":")).encode("utf-8")
        self.process.stdin.write(f"Content-Length: {len(body)}\r\n\r\n".encode("ascii") + body)
        self.process.stdin.flush()
        return message.get("id")

    def receive(self):
        try:
            message = self.incoming.get(timeout=5)
        except queue.Empty as error:
            raise TimeoutError("Timed out waiting for the language server") from error
        if isinstance(message, Exception):
            raise message
        if "id" in message:
            self.responses[message["id"]] = message
        elif message.get("method") == "textDocument/publishDiagnostics":
            diagnostic = message["params"]
            uri = diagnostic["uri"]
            # Unversioned publications cannot prove freshness for this exercise.
            if "version" not in diagnostic:
                return
            if diagnostic["version"] >= self.diagnostics.get(uri, {}).get("version", -1):
                self.diagnostics[uri] = diagnostic

    def request(self, method, params=None):
        request_id = self.send(method, params, request=True)
        while request_id not in self.responses:
            self.receive()
        response = self.responses.pop(request_id)
        if "error" in response:
            raise RuntimeError(response["error"]["message"])
        return response["result"]

    def initialize(self):
        self.capabilities = self.request("initialize", {
            "processId": os.getpid(),
            "rootUri": Path.cwd().as_uri(),
            "capabilities": {"textDocument": {"publishDiagnostics": {"versionSupport": True}}},
        })["capabilities"]
        self.send("initialized", {})
        self.ready = True

    def open(self, path):
        path = Path(path).resolve()
        uri = path.as_uri()
        if uri not in self.documents:
            self.documents[uri] = 1
            self.send("textDocument/didOpen", {"textDocument": {
                "uri": uri, "languageId": self.language, "version": 1,
                "text": path.read_text(encoding="utf-8"),
            }})
        return uri

    def current_diagnostics(self, path):
        uri = self.open(path)
        version = self.documents[uri]
        while self.diagnostics.get(uri, {}).get("version") != version:
            self.receive()
        return self.diagnostics[uri]

    def write(self, path, content):
        path = Path(path).resolve()
        path.write_text(content, encoding="utf-8")
        uri = path.as_uri()
        if uri in self.documents:
            self.documents[uri] += 1
            self.send("textDocument/didChange", {
                "textDocument": {"uri": uri, "version": self.documents[uri]},
                "contentChanges": [{"text": content}],
            })
        return {"written": str(path), **self.current_diagnostics(path)}

    @staticmethod
    def offset(text, position):
        lines = text.splitlines(keepends=True)
        line = position["line"]
        if line == len(lines) and position["character"] == 0:
            return len(text)
        prefix = lines[line].encode("utf-16-le")[:position["character"] * 2]
        return sum(len(part) for part in lines[:line]) + len(prefix.decode("utf-16-le"))

    def tool(self, arguments):
        operation = arguments["operation"]
        path = arguments["file_path"]
        if operation == "diagnostics":
            return self.current_diagnostics(path)
        uri = self.open(path)
        methods = {"hover": "hover", "definition": "definition", "references": "references", "rename": "rename"}
        params = {"textDocument": {"uri": uri}, "position": {
            "line": arguments["line"], "character": arguments["character"],
        }}
        if operation == "references":
            params["context"] = {"includeDeclaration": arguments["include_declaration"]}
        if operation == "rename":
            params["newName"] = arguments["new_name"]
        result = self.request("textDocument/" + methods[operation], params)
        if operation != "rename" or result is None:
            return result
        prepared = {}
        for target_uri, edits in result["changes"].items():
            target = Path(unquote(urlparse(target_uri).path))
            text = target.read_text(encoding="utf-8")
            replacements = [(self.offset(text, edit["range"]["start"]),
                             self.offset(text, edit["range"]["end"]), edit["newText"])
                            for edit in edits]
            for start, end, replacement in sorted(replacements, reverse=True):
                text = text[:start] + replacement + text[end:]
            prepared[target] = text
        diagnostics = [self.write(target, text) for target, text in prepared.items()]
        return {"changed": [str(path) for path in prepared], "diagnostics": diagnostics}

    def close(self):
        try:
            if self.ready and self.process.poll() is None:
                for uri in self.documents:
                    self.send("textDocument/didClose", {"textDocument": {"uri": uri}})
                self.request("shutdown")
                self.send("exit")
        finally:
            self.process.stdin.close()
            try:
                self.process.wait(timeout=3)
            except subprocess.TimeoutExpired:
                self.process.kill()
                self.process.wait()


TOOLS = [
    {"type": "function", "function": {"name": "LSP", "description": "Returns language-server results", "parameters": {
        "type": "object", "properties": {
            "operation": {"type": "string", "enum": ["hover", "definition", "references", "diagnostics", "rename"]},
            "file_path": {"type": "string"}, "line": {"type": "integer"}, "character": {"type": "integer"},
            "include_declaration": {"type": "boolean"}, "new_name": {"type": "string"},
        }, "required": ["operation", "file_path"],
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
    messages = [{"role": "system", "content": "Use the LSP tool for code queries and rename. Tool positions are zero-based UTF-16. Use Write for edits."},
                {"role": "user", "content": prompt}]
    for _ in range(50):
        payload = {"model": "anthropic/claude-haiku-4.5", "messages": messages, "tools": TOOLS}
        request = urllib.request.Request(
            os.environ["OPENROUTER_BASE_URL"].rstrip("/") + "/chat/completions",
            data=json.dumps(payload).encode("utf-8"),
            headers={"Content-Type": "application/json", "Authorization": "Bearer " + os.environ["OPENROUTER_API_KEY"]},
        )
        with urllib.request.urlopen(request, timeout=30) as response:
            message = json.load(response)["choices"][0]["message"]
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
            messages.append({"role": "tool", "tool_call_id": call["id"], "content": json.dumps(result)})
    raise RuntimeError("Agent exceeded 50 model requests")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("-p", required=True)
    arguments = parser.parse_args()
    server = None
    try:
        if Path(".lsp.json").exists():
            server = LanguageServer(json.loads(Path(".lsp.json").read_text()))
            server.initialize()
        print(run_agent(arguments.p, server))
    finally:
        if server:
            server.close()


if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        print(str(error), file=sys.stderr)
        sys.exit(1)
