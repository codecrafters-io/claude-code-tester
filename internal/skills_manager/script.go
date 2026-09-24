package skills_manager

import (
	"crypto/sha256"
	"encoding/hex"
)

const DataFileName = "data.txt"
const ChecksumScriptFileName = "checksum.sh"

// ChecksumScriptContents resolves the data file from the script's own location,
// so it prints the same value from any working directory. The `..` only lands on
// the data file because Skill.DataFilePath puts it one level above the script.
//
// Uses sha256sum (coreutils) rather than shasum, since the tester runs inside
// Linux images where shasum isn't guaranteed to exist.
const ChecksumScriptContents = `#!/usr/bin/env bash
cd "$(dirname "$0")/.." && sha256sum ` + DataFileName + ` | cut -c1-8
`

// ChecksumOf mirrors what ChecksumScriptContents prints for a file holding
// contents. These two must agree, so they live together and are tested together.
func ChecksumOf(contents string) string {
	sum := sha256.Sum256([]byte(contents))
	return hex.EncodeToString(sum[:])[:8]
}
