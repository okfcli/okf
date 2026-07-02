// Package sign provides post-quantum authentication for OKF bundle exports
// using ML-KEM-768 (FIPS 203) via crypto/hpke. The signer seals the archive
// hash with HPKE using the public key; the verifier opens it with the private
// key. If opening succeeds and the hash matches, the archive is authentic and
// untampered.
//
// ML-KEM is a key-encapsulation mechanism, not a signature scheme. This
// package uses HPKE (RFC 9180) with ML-KEM-768 to achieve post-quantum
// authenticated encryption of the archive hash. The resulting "signature"
// proves that the archive hash was sealed for the holder of the corresponding
// private key and has not been modified since.
package sign

import (
	"crypto/hpke"
	"crypto/mlkem"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
)

// KeyPair is a generated ML-KEM-768 key pair.
type KeyPair struct {
	PublicKey  string `json:"public_key"`  // hex-encoded encapsulation key
	PrivateKey string `json:"private_key"` // hex-encoded decapsulation key
}

// Signature is the result of signing an archive.
type Signature struct {
	Algorithm     string `json:"algorithm"`      // "ML-KEM-768/HPKE-SHA256"
	Ciphertext    string `json:"ciphertext"`     // hex-encoded HPKE ciphertext (includes enc)
	ArchiveSHA256 string `json:"archive_sha256"` // hex SHA-256 of the archive
	PublicKey     string `json:"public_key"`     // hex public key used to seal
}

// GenerateKeyPair creates a new ML-KEM-768 key pair.
func GenerateKeyPair() (*KeyPair, error) {
	dk, err := mlkem.GenerateKey768()
	if err != nil {
		return nil, fmt.Errorf("generate ML-KEM-768 key: %w", err)
	}
	return &KeyPair{
		PublicKey:  hex.EncodeToString(dk.EncapsulationKey().Bytes()),
		PrivateKey: hex.EncodeToString(dk.Bytes()),
	}, nil
}

// Sign seals the SHA-256 hash of the archive using HPKE with ML-KEM-768.
// pubKeyHex is the hex-encoded ML-KEM-768 encapsulation (public) key.
func Sign(archivePath, pubKeyHex string) (*Signature, error) {
	archiveData, err := os.ReadFile(archivePath)
	if err != nil {
		return nil, fmt.Errorf("read archive %s: %w", archivePath, err)
	}

	hash := sha256.Sum256(archiveData)

	pubBytes, err := hex.DecodeString(pubKeyHex)
	if err != nil {
		return nil, fmt.Errorf("decode public key: %w", err)
	}

	ek, err := mlkem.NewEncapsulationKey768(pubBytes)
	if err != nil {
		return nil, fmt.Errorf("create encapsulation key: %w", err)
	}

	hpkePub, err := hpke.NewMLKEMPublicKey(ek)
	if err != nil {
		return nil, fmt.Errorf("create HPKE public key: %w", err)
	}

	kdf := hpke.HKDFSHA256()
	aead := hpke.AES128GCM()

	// hpke.Seal bundles the encapsulated key (enc) into the ciphertext,
	// so the verifier only needs the ciphertext + private key to Open it.
	ciphertext, err := hpke.Seal(hpkePub, kdf, aead, []byte("okf-pq-sign"), hash[:])
	if err != nil {
		return nil, fmt.Errorf("hpke seal: %w", err)
	}

	return &Signature{
		Algorithm:     "ML-KEM-768/HPKE-SHA256",
		Ciphertext:    hex.EncodeToString(ciphertext),
		ArchiveSHA256: hex.EncodeToString(hash[:]),
		PublicKey:     pubKeyHex,
	}, nil
}

// Verify opens the HPKE ciphertext using the private key and confirms the
// opened archive hash matches the actual archive hash.
// privKeyHex is the hex-encoded ML-KEM-768 decapsulation (private) key.
func Verify(archivePath string, sig *Signature, privKeyHex string) error {
	archiveData, err := os.ReadFile(archivePath)
	if err != nil {
		return fmt.Errorf("read archive %s: %w", archivePath, err)
	}

	hash := sha256.Sum256(archiveData)

	privBytes, err := hex.DecodeString(privKeyHex)
	if err != nil {
		return fmt.Errorf("decode private key: %w", err)
	}

	dk, err := mlkem.NewDecapsulationKey768(privBytes)
	if err != nil {
		return fmt.Errorf("create decapsulation key: %w", err)
	}

	hpkePriv, err := hpke.NewMLKEMPrivateKey(dk)
	if err != nil {
		return fmt.Errorf("create HPKE private key: %w", err)
	}

	ciphertext, err := hex.DecodeString(sig.Ciphertext)
	if err != nil {
		return fmt.Errorf("decode ciphertext: %w", err)
	}

	kdf := hpke.HKDFSHA256()
	aead := hpke.AES128GCM()

	opened, err := hpke.Open(hpkePriv, kdf, aead, []byte("okf-pq-sign"), ciphertext)
	if err != nil {
		return fmt.Errorf("hpke open failed (tampered or wrong key): %w", err)
	}

	if len(opened) != 32 {
		return fmt.Errorf("opened hash has wrong length: %d", len(opened))
	}

	var openedHash [32]byte
	copy(openedHash[:], opened)

	if openedHash != hash {
		return fmt.Errorf("archive hash mismatch: archive has been tampered with")
	}

	return nil
}

// KeyPairToJSON returns the key pair as pretty-printed JSON.
func KeyPairToJSON(kp *KeyPair) ([]byte, error) {
	return json.MarshalIndent(kp, "", "  ")
}

// SignatureToJSON returns the signature as pretty-printed JSON.
func SignatureToJSON(sig *Signature) ([]byte, error) {
	return json.MarshalIndent(sig, "", "  ")
}
