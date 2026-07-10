package sign

import (
	"os"
	"path/filepath"
	"testing"
)

func writeArchive(t *testing.T, content string) string {
	t.Helper()
	archivePath := filepath.Join(t.TempDir(), "bundle.okf")
	if err := os.WriteFile(archivePath, []byte(content), 0o644); err != nil {
		t.Fatalf("write archive: %v", err)
	}
	return archivePath
}

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

	archivePath := writeArchive(t, "fake archive content")

	sig, err := Sign(archivePath, kp.PrivateKey)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	if sig.Algorithm != Algorithm {
		t.Errorf("algorithm = %q", sig.Algorithm)
	}
	if sig.Signature == "" {
		t.Error("signature is empty")
	}
	if sig.ArchiveSHA256 == "" {
		t.Error("archive_sha256 is empty")
	}
	if sig.PublicKey != kp.PublicKey {
		t.Error("signature public key does not match key pair")
	}

	if err := Verify(archivePath, sig, kp.PublicKey); err != nil {
		t.Fatalf("Verify: %v", err)
	}
}

func TestVerify_TamperedArchive(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}

	archivePath := writeArchive(t, "original content")

	sig, err := Sign(archivePath, kp.PrivateKey)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	// Tamper with the archive.
	if err := os.WriteFile(archivePath, []byte("TAMPERED content"), 0o644); err != nil {
		t.Fatalf("write tampered archive: %v", err)
	}

	if err := Verify(archivePath, sig, kp.PublicKey); err == nil {
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

	archivePath := writeArchive(t, "archive data")

	sig, err := Sign(archivePath, kp1.PrivateKey)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	// Verify against a different signer's public key.
	if err := Verify(archivePath, sig, kp2.PublicKey); err == nil {
		t.Fatal("expected error for wrong key, got nil")
	}
}

// TestVerify_ForgedSignature is the attack the previous ML-KEM/HPKE design
// could not resist: an attacker who tampers with the archive and knows only
// the signer's PUBLIC key tries to mint a fresh signature over the modified
// archive. With a real signature scheme, signing requires the private key, so
// the best the attacker can do is tamper with the signature bytes.
func TestVerify_ForgedSignature(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}

	archivePath := writeArchive(t, "original content")

	sig, err := Sign(archivePath, kp.PrivateKey)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	// Attacker cannot call Sign with the public key.
	if _, err := Sign(archivePath, kp.PublicKey); err == nil {
		t.Fatal("Sign accepted a public key as the signing key")
	}

	// Attacker flips a byte in the signature instead.
	forged := *sig
	raw := []byte(forged.Signature)
	if raw[0] == 'a' {
		raw[0] = 'b'
	} else {
		raw[0] = 'a'
	}
	forged.Signature = string(raw)

	if err := Verify(archivePath, &forged, kp.PublicKey); err == nil {
		t.Fatal("expected error for forged signature, got nil")
	}
}

func TestVerify_WrongAlgorithm(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}

	archivePath := writeArchive(t, "archive data")

	sig, err := Sign(archivePath, kp.PrivateKey)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	sig.Algorithm = "ML-KEM-768/HPKE-SHA256"
	if err := Verify(archivePath, sig, kp.PublicKey); err == nil {
		t.Fatal("expected error for wrong algorithm, got nil")
	}
}
