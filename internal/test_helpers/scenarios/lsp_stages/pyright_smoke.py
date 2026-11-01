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
            Path("pyrightconfig.json").write_text(json.dumps({"typeCheckingMode": "basic"}))
            server = reference.LanguageServer({"command": command, "args": ["--stdio"], "extensionToLanguage": {".py": "python"}})
            try:
                server.initialize()
                hover = server.tool({"operation": "hover", "filePath": "pricing.py", "line": 1, "character": 5})
                assert "price: int" in json.dumps(hover), hover
                definition = server.tool({"operation": "goToDefinition", "filePath": "checkout.py", "line": 3, "character": 10})
                assert "pricing.py" in json.dumps(definition), definition
                references = server.tool({"operation": "findReferences", "filePath": "pricing.py", "line": 1, "character": 5})
                assert len(references) >= 2, references
                assert server.current_diagnostics("checkout.py")["diagnostics"] == []
                good = Path("checkout.py").read_text()
                bad = good.replace("(250,", '("250",')
                errors = server.write("checkout.py", bad)["diagnostics"]
                assert any("cannot be assigned" in issue["message"] for issue in errors), errors
                assert server.write("checkout.py", good)["diagnostics"] == []
            finally:
                server.close()
            print("Pyright: initialize, hover, alias definition, references, write synchronization, diagnostic feedback, repair and shutdown passed")
        finally:
            os.chdir(original_directory)


if __name__ == "__main__":
    main()
