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
    original = Path("checkout.py").read_text()
    imported = original.replace("from pricing","from discounts")
    assert server.write("checkout.py",imported)["diagnostics"] == []
    moved = server.tool({"operation":"goToDefinition","filePath":"checkout.py","line":5,"character":12})
    assert "discounts.py" in json.dumps(moved),moved
    assert server.write("checkout.py",original)["diagnostics"] == []
    server.write("pricing.py",PRICE_API)
    report = Path("reports.py").read_text().replace("total(250, 2)",'total(250, 2)["cents"]').replace("total(100, 1)",'total(100, 1)["cents"]')
    assert server.write("reports.py",report)["diagnostics"] == []
    partial = original.replace("    return invoice_total(250, 2)","    amount = invoice_total(250, 2)\n    return amount")
    errors = server.write("checkout.py",partial)["diagnostics"]
    assert any(issue.get("severity")==1 and "int" in issue["message"] for issue in errors),errors
    namespace = {}
    exec(compile(partial,"checkout.py","exec"),namespace)
    before = namespace["checkout_total"]()
    assert before == {"cents":500},before
    print("Before feedback: saved caller returns",before,"instead of integer 500")
    print("Pyright reports:",errors[0]["message"])
    repaired = partial.replace("return amount",'return amount["cents"]')
    assert server.write("checkout.py",repaired)["diagnostics"] == []
    namespace = {}
    exec(compile(Path("checkout.py").read_text(),"checkout.py","exec"),namespace)
    assert namespace["checkout_total"]()==500
    from reports import report_total
    assert report_total()==600
    print("Repaired caller: diagnostics []; checkout result: 500; report result: 600")
    print("Pyright: definition, references, edited-import synchronization, API migration, error feedback and clearing passed")


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
        (root/"preview.py").write_text("def total(label: str) -> str:\n    return label\n")
        (root/"pyrightconfig.json").write_text(json.dumps({"typeCheckingMode":"basic"}))
        config=root/".lsp.json"
        config.write_text(json.dumps({"python":{"command":server,"args":["--stdio"],"extensionToLanguage":{".py":"python"}}}))
        subprocess.run([binary,"--lsp-transport",str(config),"--",sys.executable,str(Path(__file__).resolve()),"--inside"],cwd=root,check=True,timeout=45)

if __name__=="__main__":
    main()
