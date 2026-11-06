package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

func GenerateKeyPair() (*ecdh.PrivateKey, string, error) {
	curve := ecdh.P256()
	privKey, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		return nil, "", err
	}

	pubKey := privKey.PublicKey()
	pubKeyBytes := pubKey.Bytes()

	pubKeyHex := hex.EncodeToString(pubKeyBytes)
	return privKey, pubKeyHex, nil
}

func GenerateSigningKeyPair() (ed25519.PublicKey, ed25519.PrivateKey, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	return pub, priv, nil
}

func SignMessage(privKey ed25519.PrivateKey, message []byte) (string, error) {
	sig := ed25519.Sign(privKey, message)
	return base64.StdEncoding.EncodeToString(sig), nil
}

func VerifySignature(pubKeyBase64 string, message []byte, signatureBase64 string) (bool, error) {
	pubKeyBytes, err := base64.StdEncoding.DecodeString(pubKeyBase64)
	if err != nil {
		return false, err
	}
	if len(pubKeyBytes) != ed25519.PublicKeySize {
		return false, nil
	}
	signatureBytes, err := base64.StdEncoding.DecodeString(signatureBase64)
	if err != nil {
		return false, err
	}
	return ed25519.Verify(ed25519.PublicKey(pubKeyBytes), message, signatureBytes), nil
}

func DeriveSharedSecret(privKey *ecdh.PrivateKey, pubKeyHex string) ([]byte, error) {
	pubKeyBytes, err := hex.DecodeString(pubKeyHex)
	if err != nil {
		return nil, err
	}

	curve := ecdh.P256()
	pubKey, err := curve.NewPublicKey(pubKeyBytes)
	if err != nil {
		return nil, err
	}

	shared, err := privKey.ECDH(pubKey)
	if err != nil {
		return nil, err
	}

	// Derivar clave AES-256 desde el secreto compartido
	hash := sha256.Sum256(shared)
	return hash[:], nil
}

func EncryptPayload(plaintext []byte, key []byte) (string, string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", "", err
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	// Retornar ciphertext y nonce en base64
	return base64.StdEncoding.EncodeToString(ciphertext),
		base64.StdEncoding.EncodeToString(nonce), nil
}

func DecryptPayload(ciphertext string, nonce string, key []byte) ([]byte, error) {
	ciphertextBytes, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return nil, err
	}

	nonceBytes, err := base64.StdEncoding.DecodeString(nonce)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	plaintext, err := gcm.Open(nil, nonceBytes, ciphertextBytes, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}
