"""Check the supplied transport and reference integration against real Pyright."""
import argparse
import importlib.util
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import time

PRICE_API = 'from typing import TypedDict\n\n\nclass Price(TypedDict):\n    cents: int\n\n\ndef total(price: int, quantity: int) -> Price:\n    return {"cents": price * quantity}\n'


def inside():
    sys.path.insert(0,str(Path.cwd()))
    source = Path(__file__).parent / "users_code_pass_all/app/main.py"
    sys.path.insert(0,str(source.parent))
    spec = importlib.util.spec_from_file_location("reference",source)
    reference = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(reference)
    server = reference.LanguageServer()
    # Wait for this independent server's project discovery before asserting workspace coverage.
    cursor = 0
    deadline = time.monotonic() + 10
    while True:
        batch = server.transport.notifications(cursor,1000)
        cursor = batch["cursor"]
        if any("source file" in message.get("params",{}).get("message","") for message in batch["notifications"]):
            break
        if time.monotonic() >= deadline:
            raise RuntimeError("Pyright workspace discovery did not finish")
    definition = server.tool({"operation":"goToDefinition","filePath":"checkout.py","line":5,"character":12})
    assert "pricing.py" in json.dumps(definition),definition
    references = server.tool({"operation":"findReferences","filePath":"pricing.py","line":1,"character":5})
    assert "checkout.py" in json.dumps(references),references
    assert "reports.py" in json.dumps(references),references
    assert "preview.py" not in json.dumps(references),references
    checkout_locations = [item["range"]["start"] for item in references if item["uri"].endswith("/checkout.py")]
    assert {"line": 0, "character": 20} in checkout_locations, checkout_locations
    original = Path("checkout.py").read_text()
    imported = original.replace("from pricing","from discounts")
    server.write("checkout.py", imported)
    moved = server.tool({"operation":"goToDefinition","filePath":"checkout.py","line":5,"character":12})
    assert "discounts.py" in json.dumps(moved),moved
    server.write("checkout.py", original)
    server.write("pricing.py",PRICE_API)
    report = Path("reports.py").read_text().replace("total(250, 2)",'total(250, 2)["cents"]').replace("total(100, 1)",'total(100, 1)["cents"]')
    # Shift lines as well as the second call column so stale indexed text cannot pass.
    report = "\n\n" + report
    assert not any(uri.endswith("/reports.py") for uri in server.documents)
    server.write("reports.py", report)
    checkout = original.replace("invoice_total(250, 2)", 'invoice_total(250, 2)["cents"]')
    server.write("checkout.py", checkout)
    current = server.tool({"operation":"goToDefinition","filePath":"checkout.py","line":5,"character":12})
    assert "pricing.py" in json.dumps(current), current
    locations = current if isinstance(current, list) else [current]
    assert any(location["range"]["start"]["line"] == 7 for location in locations), current
    references = server.tool({"operation":"findReferences","filePath":"pricing.py","line":8,"character":5})
    assert "checkout.py" in json.dumps(references), references
    assert "reports.py" in json.dumps(references), references
    assert "preview.py" not in json.dumps(references), references
    report_locations = [item["range"]["start"] for item in references if item["uri"].endswith("/reports.py")]
    assert {"line": 6, "character": 11} in report_locations, report_locations
    assert {"line": 6, "character": 36} in report_locations, report_locations
    assert all(position["line"] in (2, 6) for position in report_locations), report_locations
    print("Pyright: closed-file references moved to line 7, columns 12 and 37")
    command = subprocess.run([sys.executable, "-B", "-m", "unittest", "-v", "test_invoice.py"], capture_output=True, text=True, check=True)
    print(command.stderr, end="")
    from pricing import total
    from checkout import checkout_total
    from reports import report_total
    assert total(37, 4) == {"cents": 148}
    assert type(checkout_total()) is int and checkout_total() == 500
    assert type(report_total()) is int and report_total() == 600
    print("Pyright: definition, references, edited-import synchronization and API migration passed")


def main():
    parser=argparse.ArgumentParser()
    parser.add_argument("--server",default="pyright-langserver")
    parser.add_argument("--inside",action="store_true")
    args=parser.parse_args()
    if args.inside:
        inside()
        return
    server=shutil.which(args.server)
    binary=os.environ.get("CODECRAFTERS_LSP_TRANSPORT")
    if not server or not binary:
        parser.error("Run make test_lsp_with_pyright with Pyright installed")
    with tempfile.TemporaryDirectory(prefix="lsp-pyright-") as directory:
        root=Path(directory)
        (root/"pricing.py").write_text("def total(price: int, quantity: int) -> int:\n    return price * quantity\n")
        (root/"discounts.py").write_text("def total(price: int, quantity: int) -> int:\n    return price * quantity - 50\n")
        (root/"checkout.py").write_text("from pricing import total as invoice_total\n\n\ndef checkout_total() -> int:\n    return invoice_total(250, 2)\n")
        (root/"reports.py").write_text("from pricing import total\n\n\ndef report_total() -> int:\n    return total(250, 2) + total(100, 1)\n")
        (root/"preview.py").write_text('def total(label: str) -> str:\n    return "Preview total: " + label\n')
        tests = Path(__file__).parents[3] / "lsp_manager/testdata/test_invoice.py"
        (root/"test_invoice.py").write_text(tests.read_text())
        (root/"pyrightconfig.json").write_text(json.dumps({"typeCheckingMode":"basic"}))
        config=root/".lsp.json"
        config.write_text(json.dumps({"python":{"command":server,"args":["--stdio"],"extensionToLanguage":{".py":"python"}}}))
        subprocess.run([binary,"--lsp-transport",str(config),"--",sys.executable,str(Path(__file__).resolve()),"--inside"],cwd=root,check=True,timeout=45)

if __name__=="__main__":
    main()
