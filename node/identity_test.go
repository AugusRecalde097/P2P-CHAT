package node

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadOrCreateNodePersistsIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "identity.json")

	first, err := LoadOrCreateNode(path, "Alice")
	if err != nil {
		t.Fatalf("create identity: %v", err)
	}
	second, err := LoadOrCreateNode(path, "Bob")
	if err != nil {
		t.Fatalf("load identity: %v", err)
	}

	if first.ID != second.ID {
		t.Fatalf("expected stable node id, got %q and %q", first.ID, second.ID)
	}
	if first.PublicKey != second.PublicKey {
		t.Fatal("expected stable ecdh public key")
	}
	if first.SignPublicKey != second.SignPublicKey {
		t.Fatal("expected stable signing public key")
	}
	if second.Nick != "Bob" {
		t.Fatalf("expected nickname to remain runtime-configurable, got %q", second.Nick)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("identity file was not created: %v", err)
	}
}

func TestLoadOrCreateNodeRejectsCorruptIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "identity.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"node_id":"broken"}`), 0o600); err != nil {
		t.Fatalf("write corrupt identity: %v", err)
	}

	if _, err := LoadOrCreateNode(path, "Alice"); err == nil {
		t.Fatal("expected corrupt identity to be rejected")
	}
}
