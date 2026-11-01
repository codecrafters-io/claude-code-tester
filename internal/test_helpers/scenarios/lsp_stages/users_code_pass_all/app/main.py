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


class LanguageServer:
    def __init__(self, configuration):
        self.process = subprocess.Popen(
            [configuration["command"], *configuration["args"]],
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=None,
        )
        self.language = configuration["extensionToLanguage"][".py"]
        self.incoming = queue.Queue()
        self.next_id = 0
        self.documents = {}
        self.diagnostics = {}
        self.diagnostic_ticks = {}
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
            version = diagnostic.get("version")
            if version is not None and version < self.documents.get(uri, 0):
                return
            previous = self.diagnostics.get(uri, {}).get("version")
            if version is None or previous is None or version >= previous:
                self.diagnostics[uri] = diagnostic
                self.diagnostic_ticks[uri] = self.diagnostic_ticks.get(uri, 0) + 1

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

    def current_diagnostics(self, path, after=0):
        uri = self.open(path)
        version = self.documents[uri]
        while self.diagnostic_ticks.get(uri, 0) <= after or self.diagnostics.get(uri, {}).get("version", version) != version:
            self.receive()
        return {"uri": uri, "diagnostics": self.diagnostics[uri]["diagnostics"]}

    def write(self, path, content):
        path = Path(path).resolve()
        uri = path.as_uri()
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
        methods = {"hover": "hover", "goToDefinition": "definition", "findReferences": "references"}
        params = {"textDocument": {"uri": uri}, "position": {
            "line": arguments["line"] - 1, "character": arguments["character"] - 1,
        }}
        if operation == "findReferences":
            params["context"] = {"includeDeclaration": True}
        result = self.request("textDocument/" + methods[operation], params)
        return result

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
            "operation": {"type": "string", "enum": ["hover", "goToDefinition", "findReferences"]},
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
            messages.append({"role": "tool", "tool_call_id": call["id"], "content": json.dumps(result)})
    raise RuntimeError("Agent exceeded 50 model requests")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("-p", required=True)
    arguments = parser.parse_args()
    server = None
    try:
        if Path(".lsp.json").exists():
            configuration = json.loads(Path(".lsp.json").read_text())
            server = LanguageServer(next(iter(configuration.values())))
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
