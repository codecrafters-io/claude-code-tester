"""Verify Claude Code's LSP behavior with Pyright and a local scripted model."""

import argparse
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import threading
import urllib.request
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer


def proxy(server, events_url):
    process = subprocess.Popen([server, "--stdio"], stdin=subprocess.PIPE, stdout=subprocess.PIPE)

    def forward(source, target, direction):
        while True:
            headers = b""
            while not headers.endswith(b"\r\n\r\n"):
                part = source.read(1)
                if not part:
                    return
                headers += part
            length = int(next(line.split(b":", 1)[1] for line in headers.split(b"\r\n") if line.lower().startswith(b"content-length:")))
            chunks = []
            remaining = length
            while remaining:
                chunk = source.read(remaining)
                if not chunk:
                    raise EOFError("Incomplete language-server frame")
                chunks.append(chunk)
                remaining -= len(chunk)
            body = b"".join(chunks)
            event = {"direction": direction, "message": json.loads(body)}
            request = urllib.request.Request(events_url, data=json.dumps(event).encode(), headers={"Content-Type": "application/json"})
            with urllib.request.urlopen(request, timeout=5) as response:
                response.read()
            target.write(headers + body)
            target.flush()

    output = threading.Thread(target=forward, args=(process.stdout, sys.stdout.buffer, "server"), daemon=True)
    output.start()
    try:
        forward(sys.stdin.buffer, process.stdin, "client")
    finally:
        process.stdin.close()
        try:
            process.wait(timeout=3)
        except subprocess.TimeoutExpired:
            process.kill()
            process.wait()
        output.join(timeout=1)


VERSION = "2.1.318"


def tool_results(request):
    """The text of each tool result in a model request, in order."""
    texts = []
    for message in request.get("messages", []):
        for block in message.get("content", []) if isinstance(message.get("content"), list) else []:
            if isinstance(block, dict) and block.get("type") == "tool_result":
                body = block.get("content")
                texts.append(body if isinstance(body, str) else "".join(part.get("text", "") for part in body if isinstance(part, dict)))
    return texts


# The pinned CLI converts editor coordinates and delivers diagnostics after edits.
def verify(claude, server):
    # Arrange
    version = subprocess.check_output([claude, "--version"], text=True).strip()
    if not version.startswith(VERSION + " "):
        raise RuntimeError(f"Expected the CI-pinned Claude Code {VERSION}, got {version}")
    events, requests, failures = [], [], []
    condition = threading.Condition()
    with tempfile.TemporaryDirectory(prefix="lsp-claude-smoke-") as directory:
        root = Path(directory).resolve()
        plugin = root / "plugin"
        (plugin / ".claude-plugin").mkdir(parents=True)
        (plugin / ".claude-plugin/plugin.json").write_text(json.dumps({"name": "lsp-course-smoke", "version": "1.0.0"}))
        pricing = "def total(price: int, quantity: int) -> int:\n    return price * quantity\n"
        original = "from pricing import total as invoice_total\n\n\ndef checkout_total() -> int:\n    return invoice_total(250, 2)\n"
        bad = original.replace("    return invoice_total(250, 2)", "    amount = invoice_total(250, 2)\n    return amount")
        good = bad.replace("return amount", 'return amount["cents"]')
        migrated = 'from typing import TypedDict\n\nclass Price(TypedDict):\n    cents: int\n\ndef total(price: int, quantity: int) -> Price:\n    return {"cents": price * quantity}\n'
        (root / "pricing.py").write_text(pricing)
        (root / "checkout.py").write_text(original)
        (root / "pyrightconfig.json").write_text(json.dumps({"typeCheckingMode": "basic"}))
        actions = [
            ("Read", {"file_path":str(root/"pricing.py")}),
            ("Read", {"file_path":str(root/"checkout.py")}),
            ("LSP", {"operation":"goToDefinition","filePath":"checkout.py","line":5,"character":12}),
            ("LSP", {"operation":"findReferences","filePath":"pricing.py","line":1,"character":5}),
            ("Write", {"file_path":str(root/"pricing.py"),"content":migrated}),
            ("Write", {"file_path":str(root/"checkout.py"),"content":bad}),
            ("LSP", {"operation":"goToDefinition","filePath":"checkout.py","line":5,"character":14}),
            ("Write", {"file_path":str(root/"checkout.py"),"content":good}),
            ("LSP", {"operation":"goToDefinition","filePath":"checkout.py","line":5,"character":14}),
        ]

        def publication_after_edit(content):
            changed = False
            for event in events:
                message = event["message"]
                params = message.get("params", {})
                if event["direction"] == "client" and message.get("method") == "textDocument/didChange":
                    changed = params.get("contentChanges", [{}])[0].get("text") == content
                if changed and event["direction"] == "server" and message.get("method") == "textDocument/publishDiagnostics":
                    if params.get("uri", "").endswith("/checkout.py"):
                        issues = params.get("diagnostics", [])
                        if (content == bad and issues) or (content == good and not issues):
                            return True
            return False

        class Handler(BaseHTTPRequestHandler):
            def log_message(self, *args):
                pass

            def do_POST(self):
                body = json.loads(self.rfile.read(int(self.headers.get("Content-Length", 0))) or "{}")
                if self.path == "/wire":
                    with condition:
                        events.append(body)
                        condition.notify_all()
                    payload = {}
                elif "count_tokens" in self.path:
                    payload = {"input_tokens": 1}
                else:
                    requests.append(body)
                    completed = sum(1 for message in body.get("messages", []) for block in message.get("content", []) if isinstance(block, dict) and block.get("type") == "tool_result")
                    if completed in [6, 8]:
                        content = bad if completed == 6 else good
                        with condition:
                            if not condition.wait_for(lambda: publication_after_edit(content), timeout=10):
                                failures.append(f"No diagnostic publication after edit {completed}")
                    repair_ready = completed != 7 or any(word in json.dumps(body.get("messages", [])) for word in ["not assignable", "cannot be assigned"])
                    if not repair_ready:
                        failures.append("Type error must reach the model before it selects the repair")
                    content = [{"type": "text", "text": "Done."}]
                    reason = "end_turn"
                    if completed < len(actions) and repair_ready:
                        name, arguments = actions[completed]
                        content = [{"type": "tool_use", "id": f"lsp_smoke_{completed}", "name": name, "input": arguments}]
                        reason = "tool_use"
                    payload = {"id": "lsp-smoke", "type": "message", "role": "assistant", "model": "claude-haiku-4-5", "content": content, "stop_reason": reason, "stop_sequence": None, "usage": {"input_tokens": 1, "output_tokens": 1}}
                self.send_response(200)
                self.send_header("Content-Type", "application/json")
                self.end_headers()
                self.wfile.write(json.dumps(payload).encode())

        model = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        threading.Thread(target=model.serve_forever, daemon=True).start()
        address = f"http://127.0.0.1:{model.server_address[1]}"
        configuration = {"python": {"command": sys.executable, "args": ["-u", str(Path(__file__).resolve()), "--proxy", "--server", server, "--events-url", address + "/wire"], "extensionToLanguage": {".py": "python"}}}
        (plugin / ".lsp.json").write_text(json.dumps(configuration))
        environment = os.environ.copy()
        environment.update({"CLAUDE_CONFIG_DIR": str(root / "config"), "ANTHROPIC_BASE_URL": address, "ANTHROPIC_API_KEY": "local-smoke-only", "ANTHROPIC_AUTH_TOKEN": "", "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1", "DISABLE_AUTOUPDATER": "1"})
        command = [claude, "--debug-file", str(root / "debug.log"), "--setting-sources", "", "--strict-mcp-config", "--no-session-persistence", "--plugin-dir", str(plugin), "--model", "claude-haiku-4-5", "--tools", "LSP,Read,Write,Edit,Bash", "--allowedTools", "LSP,Read,Write,Edit", "-p", "Migrate pricing.total to a Price TypedDict with cents, update its caller, and repair any missed return conversion using diagnostic feedback."]
        # Act
        try:
            process = subprocess.run(command, cwd=root, env=environment, stdin=subprocess.DEVNULL, capture_output=True, text=True, timeout=60)
        finally:
            model.shutdown()
        # Assert
        assert process.returncode == 0, (process.stdout, process.stderr)
        assert not failures, failures
        client = [e["message"] for e in events if e["direction"] == "client"]
        methods = [m.get("method") for m in client]
        for method in ["initialize", "initialized", "textDocument/definition", "textDocument/references", "textDocument/didChange"]:
            assert method in methods, (method, methods)
        definition = next(m for m in client if m.get("method")=="textDocument/definition")
        assert definition["params"]["position"] == {"line":4,"character":11}
        references = next(m for m in client if m.get("method") == "textDocument/references")
        assert references["params"]["context"] == {"includeDeclaration": True}
        assert publication_after_edit(bad) and publication_after_edit(good)
        assert (root / "checkout.py").read_text() == good
        if not any(word in json.dumps(request.get("messages", [])) for request in requests for word in ["not assignable", "cannot be assigned"]):
            debug = (root / "debug.log").read_text()
            print("\n".join(line for line in debug.splitlines() if "diagnostic" in line.lower())[-8000:])
            raise AssertionError("Type error never reached the model")
        schema = next(tool["input_schema"] for request in requests for tool in request.get("tools", []) if tool.get("name") == "LSP")
        assert "rename" not in schema["properties"]["operation"]["enum"]
        # The tester grades learners against these formats: relative files and 1-based positions,
        # and new diagnostics appended to a later tool result rather than returned by Write.
        results = tool_results(requests[-1])
        assert results[2].startswith("Defined in pricing.py:1:5"), results[2]
        assert "checkout.py:\n  Line 1:21" in results[3] and "pricing.py:\n  Line 1:5" in results[3], results[3]
        assert "<new-diagnostics>" not in results[4] and "<new-diagnostics>" not in results[5], results[4:6]
        reported = [text for text in results if "<new-diagnostics>" in text]
        assert reported and "checkout.py:" in reported[0] and "[Line 6:12]" in reported[0], results
        print(f"Claude Code {VERSION} + Pyright: initialization, navigation, synchronization, diagnostic feedback and repair passed")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--claude", default="claude")
    parser.add_argument("--server", default="pyright-langserver")
    parser.add_argument("--proxy", action="store_true")
    parser.add_argument("--events-url")
    arguments = parser.parse_args()
    server = shutil.which(arguments.server)
    if server is None:
        parser.error("Install pyright@1.1.414 and provide --server")
    if arguments.proxy:
        proxy(server, arguments.events_url)
    else:
        claude = shutil.which(arguments.claude)
        if claude is None:
            parser.error(f"Install Claude Code {VERSION} and provide --claude")
        verify(claude, server)


if __name__ == "__main__":
    main()
