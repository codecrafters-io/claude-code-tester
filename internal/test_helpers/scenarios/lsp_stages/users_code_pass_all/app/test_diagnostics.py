"""Verify diagnostic collection and result formatting without a language analyzer."""

import os
from pathlib import Path
import tempfile
import unittest
from unittest import mock

import main as agent


def server_for(documents):
    server = agent.LanguageServer.__new__(agent.LanguageServer)
    server.transport = mock.Mock()
    server.documents = dict(documents)
    server.languages = {".py": "python"}
    server.cursor = 0
    server.shown = {}
    return server


def notification(uri, diagnostics, version=None):
    params = {"uri": uri, "diagnostics": diagnostics}
    if version is not None:
        params["version"] = version
    return {"method": "textDocument/publishDiagnostics", "params": params}


def error(line, character, message):
    position = {"line": line, "character": character}
    return {"range": {"start": position, "end": position}, "severity": 1, "message": message}


class ProjectTestCase(unittest.TestCase):
    def setUp(self):
        self.folder = tempfile.TemporaryDirectory()
        self.root = Path(self.folder.name).resolve()
        self.previous = os.getcwd()
        os.chdir(self.root)
        self.checkout = (self.root / "checkout.py").as_uri()
        self.pricing = (self.root / "pricing.py").as_uri()

    def tearDown(self):
        os.chdir(self.previous)
        self.folder.cleanup()


class NewDiagnosticsTests(ProjectTestCase):
    # Errors in a file that was not written reach the model with 1-based positions.
    def test_reports_errors_in_unwritten_callers(self):
        # Arrange
        server = server_for({self.pricing: 2, self.checkout: 1})
        server.transport.notifications.return_value = {"cursor": 2, "notifications": [
            notification(self.pricing, [], 2),
            notification(self.checkout, [error(4, 11, "Price is not int")], 1),
        ]}

        # Act
        block = server.new_diagnostics()

        # Assert
        self.assertIn("checkout.py:\n  ✘ [Line 5:12] Price is not int", block)
        self.assertNotIn("pricing.py", block)
        server.transport.notifications.assert_called_once_with(0, 0)
        self.assertEqual(server.cursor, 2)

    # Reports for an older version are ignored; unversioned reports are current.
    def test_ignores_stale_and_accepts_unversioned_reports(self):
        # Arrange
        server = server_for({self.checkout: 2})
        server.transport.notifications.return_value = {"cursor": 3, "notifications": [
            notification(self.checkout, [error(4, 11, "Current error")]),
            notification(self.checkout, [error(0, 0, "Old error")], 1),
        ]}

        # Act
        block = server.new_diagnostics()

        # Assert
        self.assertIn("Current error", block)
        self.assertNotIn("Old error", block)

    # An issue is shown once, and again only after it was fixed and came back.
    def test_reports_only_new_issues(self):
        # Arrange
        server = server_for({self.checkout: 2})
        report = notification(self.checkout, [error(4, 11, "Price is not int")], 2)
        server.transport.notifications.side_effect = [
            {"cursor": 1, "notifications": [report]},
            {"cursor": 2, "notifications": [report]},
            {"cursor": 3, "notifications": [notification(self.checkout, [], 2)]},
            {"cursor": 4, "notifications": [report]},
        ]

        # Act
        blocks = [server.new_diagnostics() for _ in range(4)]

        # Assert
        self.assertIn("Price is not int", blocks[0])
        self.assertEqual(blocks[1:3], ["", ""])
        self.assertIn("Price is not int", blocks[3])

    # Write saves and synchronizes the file without waiting for diagnostics.
    def test_write_returns_without_waiting(self):
        # Arrange
        path = self.root / "checkout.py"
        path.write_text("old = 1\n")
        server = server_for({self.checkout: 1})

        # Act
        result = server.write(path, "new = 2\n")

        # Assert
        self.assertEqual(result, "Wrote checkout.py")
        self.assertEqual(path.read_text(), "new = 2\n")
        server.transport.notify.assert_called_once_with("textDocument/didChange", {
            "textDocument": {"uri": self.checkout, "version": 2},
            "contentChanges": [{"text": "new = 2\n"}],
        })
        server.transport.notifications.assert_not_called()


class FormattedResultTests(ProjectTestCase):
    # Definitions and references show relative files and 1-based positions.
    def test_formats_locations_like_claude_code(self):
        # Arrange
        (self.root / "checkout.py").write_text("x = 1\n")
        server = server_for({})
        pricing = {"uri": self.pricing, "range": {"start": {"line": 0, "character": 4}, "end": {"line": 0, "character": 9}}}
        checkout = {"uri": self.checkout, "range": {"start": {"line": 0, "character": 20}, "end": {"line": 0, "character": 25}}}
        server.transport.request.side_effect = [pricing, [pricing, checkout], []]
        arguments = {"filePath": "checkout.py", "line": 5, "character": 12}

        # Act
        definition = server.tool({"operation": "goToDefinition", **arguments})
        references = server.tool({"operation": "findReferences", **arguments})
        empty = server.tool({"operation": "findReferences", **arguments})

        # Assert
        self.assertEqual(definition, "Defined in pricing.py:1:5")
        self.assertEqual(references, "Found 2 references across 2 files:\n\ncheckout.py:\n  Line 1:21\n\npricing.py:\n  Line 1:5")
        self.assertEqual(empty, "No references found.")


if __name__ == "__main__":
    unittest.main()
