package contractprovenance

import (
	"bytes"
	"encoding/hex"
	"os"
	"testing"
)

func TestGeneratedProvenanceMatchesContractLockAndSkill(t *testing.T) {
	for _, path := range []string{
		"../../specs/mosoo-contract.json",
		"../../publish/skills/mosoo/references/provenance.json",
	} {
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, embedded) {
			t.Fatalf("%s does not match embedded contract provenance", path)
		}
	}

	info := Current()
	if len(info.UpstreamCommit) != 40 {
		t.Fatalf("upstream commit = %q, want full SHA", info.UpstreamCommit)
	}
	if _, err := hex.DecodeString(info.UpstreamCommit); err != nil {
		t.Fatalf("upstream commit is not hexadecimal: %v", err)
	}
	if len(info.PublicThreadOpenAPI.SHA256) != 64 {
		t.Fatalf("OpenAPI SHA-256 = %q", info.PublicThreadOpenAPI.SHA256)
	}
	if _, err := hex.DecodeString(info.PublicThreadOpenAPI.SHA256); err != nil {
		t.Fatalf("OpenAPI SHA-256 is not hexadecimal: %v", err)
	}
	if info.PublicThreadOpenAPI.Normalization != "json-sort-keys-v1" {
		t.Fatalf("normalization = %q", info.PublicThreadOpenAPI.Normalization)
	}
}
