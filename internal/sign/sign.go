// Package sign provides post-quantum digital signatures for OKF bundle
// exports using ML-DSA-65 (FIPS 204, formerly CRYSTALS-Dilithium). The signer
// holds the private key and signs the archive's SHA-256 hash; anyone with the
// public key can verify that the archive was signed by the key holder and has
// not been modified since.
package sign

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/cloudflare/circl/sign/mldsa/mldsa65"
)

// Algorithm identifies the signature scheme used by this package.
const Algorithm = "ML-DSA-65"

// signContext domain-separates OKF archive signatures from any other use of
// the same key (FIPS 204 context string).
var signContext = []byte("okf-pq-sign")

// KeyPair is a generated ML-DSA-65 key pair. The private key is stored as the
// 32-byte FIPS 204 seed, from which the full key is deterministically derived.
type KeyPair struct {
	PublicKey  string `json:"public_key"`  // hex-encoded ML-DSA-65 public key
	PrivateKey string `json:"private_key"` // hex-encoded 32-byte seed
}

// Signature is the result of signing an archive.
type Signature struct {
	Algorithm     string `json:"algorithm"`      // "ML-DSA-65"
	Signature     string `json:"signature"`      // hex-encoded ML-DSA-65 signature
	ArchiveSHA256 string `json:"archive_sha256"` // hex SHA-256 of the archive
	PublicKey     string `json:"public_key"`     // hex public key of the signer (informational)
}

// GenerateKeyPair creates a new ML-DSA-65 key pair from a random seed.
func GenerateKeyPair() (*KeyPair, error) {
	var seed [mldsa65.SeedSize]byte
	if _, err := rand.Read(seed[:]); err != nil {
		return nil, fmt.Errorf("generate ML-DSA-65 seed: %w", err)
	}
	pk, _ := mldsa65.NewKeyFromSeed(&seed)
	return &KeyPair{
		PublicKey:  hex.EncodeToString(pk.Bytes()),
		PrivateKey: hex.EncodeToString(seed[:]),
	}, nil
}

// Sign signs the SHA-256 hash of the archive with ML-DSA-65.
// privKeyHex is the hex-encoded 32-byte seed from GenerateKeyPair.
func Sign(archivePath, privKeyHex string) (*Signature, error) {
	hash, err := hashFile(archivePath)
	if err != nil {
		return nil, err
	}

	seedBytes, err := hex.DecodeString(privKeyHex)
	if err != nil {
		return nil, fmt.Errorf("decode private key: %w", err)
	}
	if len(seedBytes) != mldsa65.SeedSize {
		return nil, fmt.Errorf("private key must be a %d-byte seed, got %d bytes", mldsa65.SeedSize, len(seedBytes))
	}
	var seed [mldsa65.SeedSize]byte
	copy(seed[:], seedBytes)
	pk, sk := mldsa65.NewKeyFromSeed(&seed)

	sig := make([]byte, mldsa65.SignatureSize)
	if err := mldsa65.SignTo(sk, hash, signContext, false, sig); err != nil {
		return nil, fmt.Errorf("sign archive hash: %w", err)
	}

	return &Signature{
		Algorithm:     Algorithm,
		Signature:     hex.EncodeToString(sig),
		ArchiveSHA256: hex.EncodeToString(hash),
		PublicKey:     hex.EncodeToString(pk.Bytes()),
	}, nil
}

// Verify checks the ML-DSA-65 signature over the archive's SHA-256 hash.
// pubKeyHex is the trusted hex-encoded public key of the expected signer; the
// public key embedded in the signature file is never used for verification.
func Verify(archivePath string, sig *Signature, pubKeyHex string) error {
	if sig.Algorithm != Algorithm {
		return fmt.Errorf("unsupported algorithm %q (expected %q)", sig.Algorithm, Algorithm)
	}

	hash, err := hashFile(archivePath)
	if err != nil {
		return err
	}

	pubBytes, err := hex.DecodeString(pubKeyHex)
	if err != nil {
		return fmt.Errorf("decode public key: %w", err)
	}
	var pk mldsa65.PublicKey
	if err := pk.UnmarshalBinary(pubBytes); err != nil {
		return fmt.Errorf("parse public key: %w", err)
	}

	sigBytes, err := hex.DecodeString(sig.Signature)
	if err != nil {
		return fmt.Errorf("decode signature: %w", err)
	}

	if !mldsa65.Verify(&pk, hash, signContext, sigBytes) {
		return fmt.Errorf("signature verification failed: archive was tampered with or signed by a different key")
	}

	return nil
}

// hashFile streams the file through SHA-256.
func hashFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("read archive %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return nil, fmt.Errorf("hash archive %s: %w", path, err)
	}
	return h.Sum(nil), nil
}

// KeyPairToJSON returns the key pair as pretty-printed JSON.
func KeyPairToJSON(kp *KeyPair) ([]byte, error) {
	return json.MarshalIndent(kp, "", "  ")
}

// SignatureToJSON returns the signature as pretty-printed JSON.
func SignatureToJSON(sig *Signature) ([]byte, error) {
	return json.MarshalIndent(sig, "", "  ")
}
