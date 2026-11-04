"""Verify feedback selection without a language analyzer or wall-clock delays."""

from pathlib import Path
import tempfile
import unittest
from unittest import mock

import main as agent


def server_for(uri, version=2):
    server = agent.LanguageServer.__new__(agent.LanguageServer)
    server.transport = mock.Mock()
    server.documents = {uri: version}
    server.languages = {".py": "python"}
    server.cursor = 0
    return server


def notification(uri, diagnostics, version=None):
    params = {"uri": uri, "diagnostics": diagnostics}
    if version is not None:
        params["version"] = version
    return {"method": "textDocument/publishDiagnostics", "params": params}


class DiagnosticFeedbackTests(unittest.TestCase):
    # An empty notification batch does not mean the document is clean.
    def test_waits_after_empty_batch(self):
        # Arrange
        uri = "file:///project/checkout.py"
        server = server_for(uri)
        report = notification(uri, [{"message": "Return type mismatch"}], 2)
        server.transport.notifications.side_effect = [
            {"cursor": 1, "notifications": []},
            {"cursor": 2, "notifications": [report]},
        ]

        # Act
        result = server.wait_for_diagnostics(uri)

        # Assert
        self.assertEqual(result, report["params"])
        self.assertEqual(server.transport.notifications.call_args_list[1].args[0], 1)

    # Old versions and other files cannot replace the current report.
    def test_ignores_stale_and_unrelated_reports(self):
        # Arrange
        uri = "file:///project/checkout.py"
        server = server_for(uri)
        report = notification(uri, [{"message": "Current error"}], 2)
        server.transport.notifications.return_value = {"cursor": 5, "notifications": [
            notification(uri, [], 1),
            notification("file:///project/preview.py", [], 2),
            report,
            notification(uri, [{"message": "Old error"}], 1),
        ]}

        # Act
        result = server.wait_for_diagnostics(uri)

        # Assert
        self.assertEqual(result, report["params"])

    # The latest current report replaces an earlier report in the same batch.
    def test_uses_latest_matching_report(self):
        # Arrange
        uri = "file:///project/checkout.py"
        server = server_for(uri)
        clean = notification(uri, [], 2)
        server.transport.notifications.return_value = {"cursor": 2, "notifications": [
            notification(uri, [{"message": "Error before reanalysis"}], 2), clean,
        ]}

        # Act
        result = server.wait_for_diagnostics(uri)

        # Assert
        self.assertEqual(result, clean["params"])

    # A report without a version can supply feedback for the current edit.
    def test_accepts_unversioned_report(self):
        # Arrange
        uri = "file:///project/checkout.py"
        server = server_for(uri)
        report = notification(uri, [{"message": "Return type mismatch"}])
        server.transport.notifications.return_value = {"cursor": 1, "notifications": [report]}

        # Act
        result = server.wait_for_diagnostics(uri)

        # Assert
        self.assertEqual(result, report["params"])
        self.assertNotIn("version", result)

    # A queued unversioned report from before the save cannot satisfy Write.
    def test_discards_feedback_queued_before_save(self):
        # Arrange
        with tempfile.TemporaryDirectory() as folder:
            path = (Path(folder) / "checkout.py").resolve()
            path.write_text("old = 1\n")
            uri = path.as_uri()
            server = server_for(uri, 1)
            clean = notification(uri, [])
            server.transport.notifications.side_effect = [
                {"cursor": 1, "notifications": [notification(uri, [{"message": "Old error"}])]},
                {"cursor": 2, "notifications": [clean]},
            ]

            # Act
            result = server.write(path, "new = 2\n")

            # Assert
            self.assertEqual(result["diagnostics"], clean["params"])
            self.assertEqual(server.transport.notifications.call_args_list[1].args[0], 1)

    # A diagnostic timeout preserves the successful save without fabricating a clean report.
    def test_timeout_preserves_successful_write(self):
        # Arrange
        with tempfile.TemporaryDirectory() as folder:
            path = (Path(folder) / "checkout.py").resolve()
            path.write_text("old = 1\n")
            server = server_for(path.as_uri(), 1)
            server.transport.notifications.return_value = {"cursor": 0, "notifications": []}
            with mock.patch.object(agent.time, "monotonic", side_effect=[0, 0.1, 0.2, 3]):
                # Act
                result = server.write(path, "new = 2\n")

            # Assert
            self.assertEqual(path.read_text(), "new = 2\n")
            self.assertEqual(result["written"], str(path))
            self.assertIn("Timed out", result["diagnosticsError"])
            self.assertNotIn("diagnostics", result)


if __name__ == "__main__":
    unittest.main()
