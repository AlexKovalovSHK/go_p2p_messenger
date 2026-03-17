package identity_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/user/aether/internal/identity"
)

func TestIdentity_GenerateAndReload(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "identity.key")
	mgr := identity.NewIdentityManager(keyPath)

	// AC-S1-01: Generate
	id1, err := mgr.Generate()
	if err != nil {
		t.Fatalf("Failed to generate identity: %v", err)
	}

	if id1.DeviceID() == "" {
		t.Fatal("DeviceID should not be empty")
	}

	// Ensure file is created with 0600
	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("Failed to stat key file: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("Expected permissions 0600, got %v", info.Mode().Perm())
	}

	// AC-S1-01: Reload
	id2, err := mgr.Load()
	if err != nil {
		t.Fatalf("Failed to load identity: %v", err)
	}

	if id1.DeviceID() != id2.DeviceID() {
		t.Fatalf("Expected DeviceID %s, got %s", id1.DeviceID(), id2.DeviceID())
	}
}

func TestIdentity_SignVerify(t *testing.T) {
	dir := t.TempDir()
	mgr := identity.NewIdentityManager(filepath.Join(dir, "identity.key"))

	id1, err := mgr.Generate()
	if err != nil {
		t.Fatalf("Failed to generate identity: %v", err)
	}

	data := []byte("hello, aether!")
	sig, err := id1.Sign(data)
	if err != nil {
		t.Fatalf("Failed to sign data: %v", err)
	}

	// AC-S1-02: Sign and Verify -> true
	valid, err := identity.Verify(id1.PublicKey, data, sig)
	if err != nil {
		t.Fatalf("Verify failed with error: %v", err)
	}
	if !valid {
		t.Fatal("Expected true for valid signature, got false")
	}

	pubBytes, err := id1.PublicKeyBytes()
	if err != nil {
		t.Fatalf("Failed to get public key bytes: %v", err)
	}

	validBytes, err := identity.VerifyBytes(pubBytes, data, sig)
	if err != nil {
		t.Fatalf("VerifyBytes failed with error: %v", err)
	}
	if !validBytes {
		t.Fatal("Expected true for valid signature bytes, got false")
	}

	// AC-S1-03: Verify with modified data -> false
	modifiedData := []byte("hello, world!")
	valid, err = identity.Verify(id1.PublicKey, modifiedData, sig)
	if err != nil {
		t.Fatalf("Verify with modified data failed with error: %v", err)
	}
	if valid {
		t.Fatal("Expected false for modified data, got true")
	}
}
