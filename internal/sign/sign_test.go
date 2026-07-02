package sign

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateKeyPair(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	if kp.PublicKey == "" {
		t.Error("public key is empty")
	}
	if kp.PrivateKey == "" {
		t.Error("private key is empty")
	}
	if kp.PublicKey == kp.PrivateKey {
		t.Error("public and private keys are identical")
	}
}

func TestSignAndVerify(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}

	archivePath := filepath.Join(t.TempDir(), "bundle.okf")
	if err := os.WriteFile(archivePath, []byte("fake archive content"), 0o644); err != nil {
		t.Fatalf("write archive: %v", err)
	}

	sig, err := Sign(archivePath, kp.PublicKey)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	if sig.Algorithm != "ML-KEM-768/HPKE-SHA256" {
		t.Errorf("algorithm = %q", sig.Algorithm)
	}
	if sig.Ciphertext == "" {
		t.Error("ciphertext is empty")
	}
	if sig.ArchiveSHA256 == "" {
		t.Error("archive_sha256 is empty")
	}

	err = Verify(archivePath, sig, kp.PrivateKey)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
}

func TestVerify_TamperedArchive(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}

	archivePath := filepath.Join(t.TempDir(), "bundle.okf")
	if err := os.WriteFile(archivePath, []byte("original content"), 0o644); err != nil {
		t.Fatalf("write archive: %v", err)
	}

	sig, err := Sign(archivePath, kp.PublicKey)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	// Tamper with the archive.
	if err := os.WriteFile(archivePath, []byte("TAMPERED content"), 0o644); err != nil {
		t.Fatalf("write tampered archive: %v", err)
	}

	err = Verify(archivePath, sig, kp.PrivateKey)
	if err == nil {
		t.Fatal("expected error for tampered archive, got nil")
	}
}

func TestVerify_WrongKey(t *testing.T) {
	kp1, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair 1: %v", err)
	}
	kp2, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair 2: %v", err)
	}

	archivePath := filepath.Join(t.TempDir(), "bundle.okf")
	if err := os.WriteFile(archivePath, []byte("archive data"), 0o644); err != nil {
		t.Fatalf("write archive: %v", err)
	}

	sig, err := Sign(archivePath, kp1.PublicKey)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	// Verify with wrong key.
	err = Verify(archivePath, sig, kp2.PrivateKey)
	if err == nil {
		t.Fatal("expected error for wrong key, got nil")
	}
}
