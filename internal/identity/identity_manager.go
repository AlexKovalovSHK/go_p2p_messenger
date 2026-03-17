// Package identity provides Ed25519-based cryptographic identity management for Aether.
package identity

import (
	"crypto/rand"
	"errors"
	"fmt"
	"os"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

// MarshalPrivateKey provides access to libp2p's marshal function.
func MarshalPrivateKey(priv crypto.PrivKey) ([]byte, error) {
	return crypto.MarshalPrivateKey(priv)
}

var (
	ErrKeyNotFound = errors.New("identity key not found")
	ErrKeyInvalid  = errors.New("identity key is invalid")
)

// IdentityManager defines the contract for managing Ed25519 key pairs.
type IdentityManager interface {
	// Generate creates a new Ed25519 key pair, saves it to keystore, and returns the Identity.
	Generate() (*Identity, error)

	// Load reads an existing key pair from the keystore.
	Load() (*Identity, error)

	// HasKey returns true if a key pair exists in the keystore.
	HasKey() bool
}

// Identity represents the current device's cryptographic identity.
type Identity struct {
	PrivateKey crypto.PrivKey
	PublicKey  crypto.PubKey
	peerID     peer.ID
}

// DeviceID returns the string representation of the PeerID (e.g., "12D3KooW...").
func (id *Identity) DeviceID() string {
	return id.peerID.String()
}

// PeerID returns the raw libp2p PeerID.
func (id *Identity) PeerID() peer.ID {
	return id.peerID
}

// PublicKeyBytes returns the raw bytes of the public key.
func (id *Identity) PublicKeyBytes() ([]byte, error) {
	return id.PublicKey.Raw()
}

// Sign signs the given data with the private key.
func (id *Identity) Sign(data []byte) ([]byte, error) {
	return id.PrivateKey.Sign(data)
}

// Verify checks the signature against the given data and public key.
func Verify(pubKey crypto.PubKey, data, signature []byte) (bool, error) {
	return pubKey.Verify(data, signature)
}

// VerifyBytes checks the signature against the given data and raw public key bytes.
func VerifyBytes(pubKeyBytes, data, signature []byte) (bool, error) {
	pubKey, err := crypto.UnmarshalEd25519PublicKey(pubKeyBytes)
	if err != nil {
		return false, fmt.Errorf("unmarshal public key: %w", err)
	}
	return pubKey.Verify(data, signature)
}

type fileIdentityManager struct {
	path string
}

// NewIdentityManager creates a new IdentityManager bound to the specified file path.
func NewIdentityManager(path string) IdentityManager {
	return &fileIdentityManager{
		path: path,
	}
}

// Generate creates a new Ed25519 key pair, saves it to the keystore, and returns the Identity.
func (m *fileIdentityManager) Generate() (*Identity, error) {
	priv, pub, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}

	bytes, err := crypto.MarshalPrivateKey(priv)
	if err != nil {
		return nil, fmt.Errorf("marshal private key: %w", err)
	}

	if err := os.WriteFile(m.path, bytes, 0600); err != nil {
		return nil, fmt.Errorf("write key to file: %w", err)
	}

	pid, err := peer.IDFromPublicKey(pub)
	if err != nil {
		return nil, fmt.Errorf("derive peer ID: %w", err)
	}

	return &Identity{
		PrivateKey: priv,
		PublicKey:  pub,
		peerID:     pid,
	}, nil
}

// Load reads an existing key pair from the keystore.
func (m *fileIdentityManager) Load() (*Identity, error) {
	bytes, err := os.ReadFile(m.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrKeyNotFound
		}
		return nil, fmt.Errorf("read key file: %w", err)
	}

	priv, err := crypto.UnmarshalPrivateKey(bytes)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrKeyInvalid, err)
	}

	pub := priv.GetPublic()
	pid, err := peer.IDFromPublicKey(pub)
	if err != nil {
		return nil, fmt.Errorf("derive peer ID: %w", err)
	}

	return &Identity{
		PrivateKey: priv,
		PublicKey:  pub,
		peerID:     pid,
	}, nil
}

// HasKey returns true if a key pair exists in the keystore.
func (m *fileIdentityManager) HasKey() bool {
	_, err := os.Stat(m.path)
	return err == nil
}
