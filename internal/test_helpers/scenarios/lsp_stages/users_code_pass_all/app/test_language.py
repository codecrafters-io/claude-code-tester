"""Verify document language selection independently of a language analyzer."""

import tempfile
from pathlib import Path
import unittest
from unittest import mock

import main as agent


class DocumentLanguageTests(unittest.TestCase):
    # Opening a TypeScript file uses the configured language ID.
    def test_opens_with_configured_language(self):
        # Arrange
        with tempfile.TemporaryDirectory() as folder, mock.patch.object(agent, "LSPClient") as client:
            path = Path(folder) / "invoice.ts"
            text = "export const cents: number = 500;\n"
            path.write_text(text, encoding="utf-8")
            client.return_value.call.return_value = {
                "extensionToLanguage": {".ts": "typescript"}
            }
            server = agent.LanguageServer()

            # Act
            uri = server.open(path)

            # Assert
            client.return_value.notify.assert_called_once_with(
                "textDocument/didOpen",
                {"textDocument": {
                    "uri": uri, "languageId": "typescript", "version": 1, "text": text
                }},
            )

    # An unmapped extension is rejected before sending a document notification.
    def test_rejects_unmapped_extension(self):
        # Arrange
        with tempfile.TemporaryDirectory() as folder, mock.patch.object(agent, "LSPClient") as client:
            path = Path(folder) / "invoice.go"
            path.write_text("package invoice\n", encoding="utf-8")
            client.return_value.call.return_value = {
                "extensionToLanguage": {".ts": "typescript"}
            }
            server = agent.LanguageServer()

            # Act and Assert
            with self.assertRaisesRegex(ValueError, "Unsupported file extension: .go"):
                server.open(path)
            client.return_value.notify.assert_not_called()
            self.assertEqual(server.documents, {})


if __name__ == "__main__":
    unittest.main()
