package node

import (
	"bytes"
	"crypto/ecdh"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"p2p-chat/crypto"

	"github.com/google/uuid"
)

const identityFileVersion = 1

type storedIdentity struct {
	Version     int    `json:"version"`
	NodeID      string `json:"node_id"`
	ECDHPrivate string `json:"ecdh_private_key"`
	SignPrivate string `json:"sign_private_key"`
}

// LoadOrCreateNode loads a stable node identity from path, or creates it on
// first startup. The nickname is intentionally not persisted.
func LoadOrCreateNode(path, nick string) (*Node, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		return loadNodeIdentity(data, nick)
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read identity file: %w", err)
	}

	privKey, _, err := crypto.GenerateKeyPair()
	if err != nil {
		return nil, fmt.Errorf("generate ecdh identity key: %w", err)
	}
	signPub, signPriv, err := crypto.GenerateSigningKeyPair()
	if err != nil {
		return nil, fmt.Errorf("generate signing identity key: %w", err)
	}

	identity := storedIdentity{
		Version:     identityFileVersion,
		NodeID:      uuid.New().String(),
		ECDHPrivate: base64.StdEncoding.EncodeToString(privKey.Bytes()),
		SignPrivate: base64.StdEncoding.EncodeToString(signPriv),
	}
	if err := saveIdentity(path, identity); err != nil {
		return nil, err
	}

	return newNodeWithKeys(identity.NodeID, nick, privKey, signPub, signPriv), nil
}

func loadNodeIdentity(data []byte, nick string) (*Node, error) {
	var identity storedIdentity
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&identity); err != nil {
		return nil, fmt.Errorf("decode identity file: %w", err)
	}
	if identity.Version != identityFileVersion {
		return nil, fmt.Errorf("unsupported identity file version %d", identity.Version)
	}
	if identity.NodeID == "" {
		return nil, errors.New("identity file has no node id")
	}

	privateBytes, err := base64.StdEncoding.DecodeString(identity.ECDHPrivate)
	if err != nil {
		return nil, fmt.Errorf("decode ecdh private key: %w", err)
	}
	privKey, err := ecdh.P256().NewPrivateKey(privateBytes)
	if err != nil {
		return nil, fmt.Errorf("restore ecdh private key: %w", err)
	}

	signBytes, err := base64.StdEncoding.DecodeString(identity.SignPrivate)
	if err != nil {
		return nil, fmt.Errorf("decode signing private key: %w", err)
	}
	if len(signBytes) != ed25519.PrivateKeySize {
		return nil, errors.New("invalid signing private key length")
	}
	signPriv := ed25519.PrivateKey(signBytes)
	signPub, ok := signPriv.Public().(ed25519.PublicKey)
	if !ok {
		return nil, errors.New("invalid signing public key")
	}

	return newNodeWithKeys(identity.NodeID, nick, privKey, signPub, signPriv), nil
}

func saveIdentity(path string, identity storedIdentity) error {
	data, err := json.MarshalIndent(identity, "", "  ")
	if err != nil {
		return fmt.Errorf("encode identity file: %w", err)
	}

	dir := filepath.Dir(path)
	temp, err := os.CreateTemp(dir, ".p2p-chat-identity-*")
	if err != nil {
		return fmt.Errorf("create identity file: %w", err)
	}
	tempName := temp.Name()
	defer os.Remove(tempName)

	if err := temp.Chmod(0o600); err != nil {
		temp.Close()
		return fmt.Errorf("protect identity file: %w", err)
	}
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return fmt.Errorf("write identity file: %w", err)
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return fmt.Errorf("sync identity file: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close identity file: %w", err)
	}
	if err := os.Rename(tempName, path); err != nil {
		return fmt.Errorf("install identity file: %w", err)
	}
	return nil
}
