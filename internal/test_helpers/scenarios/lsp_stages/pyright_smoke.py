"""Smoke-check the reference transport against a separately installed Pyright."""

import argparse
import importlib.util
import json
import os
from pathlib import Path
import shutil
import tempfile


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--server", default="pyright-langserver")
    arguments = parser.parse_args()
    command = shutil.which(arguments.server)
    if command is None:
        parser.error("Install pyright@1.1.414 and pass --server /path/to/pyright-langserver")
    source = Path(__file__).parent / "users_code_pass_all/app/main.py"
    spec = importlib.util.spec_from_file_location("reference", source)
    reference = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(reference)
    original_directory = Path.cwd()
    with tempfile.TemporaryDirectory(prefix="lsp-pyright-") as folder:
        try:
            os.chdir(folder)
            Path("pricing.py").write_text("def total(price: int, quantity: int) -> int:\n    return price * quantity\n")
            Path("checkout.py").write_text("from pricing import total as invoice_total\n\namount = invoice_total(250, 2)\n")
            server = reference.LanguageServer({"command": command, "args": ["--stdio"], "languageId": "python"})
            try:
                server.initialize()
                hover = server.tool({"operation": "hover", "file_path": "pricing.py", "line": 0, "character": 4})
                assert "price: int" in json.dumps(hover), hover
                definition = server.tool({"operation": "definition", "file_path": "checkout.py", "line": 2, "character": 9})
                assert "pricing.py" in json.dumps(definition), definition
                references = server.tool({"operation": "references", "file_path": "pricing.py", "line": 0, "character": 4, "include_declaration": True})
                assert len(references) >= 2, references
                rename = server.request("textDocument/rename", {
                    "textDocument": {"uri": Path("pricing.py").resolve().as_uri()},
                    "position": {"line": 0, "character": 4}, "newName": "total_cents",
                })
                assert rename and ("changes" in rename or "documentChanges" in rename), rename
            finally:
                server.close()
            print("Pyright: initialize, hover, alias definition, references, rename response and shutdown passed")
        finally:
            os.chdir(original_directory)


if __name__ == "__main__":
    main()
