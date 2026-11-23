package samaccess

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"time"
)

// Keys are the Ed25519 keys user tokens are signed with: one signs, and
// every key in the file is published in the key set, so SAM verifies a
// token signed by a key added before it signed or retired after.
type Keys struct {
	signing   key
	published []key
}

type key struct {
	id      string
	private ed25519.PrivateKey
}

// KeyEntry is one key in the key file:
//
//	{"keys": [{"kid": "20261123-3f9a1c", "seed": "<base64url>", "signs": true}]}
//
// The seed is the key's 32-byte Ed25519 seed. Exactly one key signs.
type KeyEntry struct {
	ID    string `json:"kid"`
	Seed  string `json:"seed"`
	Signs bool   `json:"signs"`
}

type keyFile struct {
	Keys []KeyEntry `json:"keys"`
}

// kidForm is what a key ID may hold: it is sent in every token's header.
var kidForm = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// LoadKeys reads the key file at path.
func LoadKeys(path string) (*Keys, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("the signing keys: %w", err)
	}
	k, err := ParseKeys(raw)
	if err != nil {
		return nil, fmt.Errorf("the signing keys in %s: %w", path, err)
	}
	return k, nil
}

// ParseKeys reads a key file's contents.
func ParseKeys(raw []byte) (*Keys, error) {
	var f keyFile
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil, err
	}
	if len(f.Keys) == 0 {
		return nil, errors.New("no keys")
	}
	k := &Keys{}
	seen := map[string]bool{}
	signers := 0
	for _, e := range f.Keys {
		if !kidForm.MatchString(e.ID) {
			return nil, fmt.Errorf("kid %q must be 1 to 64 letters, digits, - or _", e.ID)
		}
		if seen[e.ID] {
			return nil, fmt.Errorf("kid %q appears twice", e.ID)
		}
		seen[e.ID] = true
		seed, err := base64.RawURLEncoding.DecodeString(e.Seed)
		if err != nil || len(seed) != ed25519.SeedSize {
			return nil, fmt.Errorf("key %q: the seed must be %d bytes in unpadded base64url", e.ID, ed25519.SeedSize)
		}
		kk := key{id: e.ID, private: ed25519.NewKeyFromSeed(seed)}
		k.published = append(k.published, kk)
		if e.Signs {
			signers++
			k.signing = kk
		}
	}
	if signers != 1 {
		return nil, fmt.Errorf("exactly one key must sign, not %d", signers)
	}
	return k, nil
}

// NewKey makes a key for the key file, not yet signing, named by the day
// and a random suffix.
func NewKey(now time.Time) (KeyEntry, error) {
	seed := make([]byte, ed25519.SeedSize)
	suffix := make([]byte, 3)
	if _, err := rand.Read(seed); err != nil {
		return KeyEntry{}, err
	}
	if _, err := rand.Read(suffix); err != nil {
		return KeyEntry{}, err
	}
	return KeyEntry{ID: now.UTC().Format("20060102") + "-" + hex.EncodeToString(suffix),
		Seed: base64.RawURLEncoding.EncodeToString(seed)}, nil
}

// GenerateKeys makes one signing key that lives as long as the process:
// for development and tests only, since a restart changes it.
func GenerateKeys(now time.Time) (*Keys, error) {
	e, err := NewKey(now)
	if err != nil {
		return nil, err
	}
	e.Signs = true
	raw, err := json.Marshal(keyFile{Keys: []KeyEntry{e}})
	if err != nil {
		return nil, err
	}
	return ParseKeys(raw)
}

// SigningKeyID is the kid of the key that signs.
func (k *Keys) SigningKeyID() string { return k.signing.id }

// JWK is a published public key.
type JWK struct {
	KeyType   string `json:"kty"`
	Curve     string `json:"crv"`
	X         string `json:"x"`
	KeyID     string `json:"kid"`
	Algorithm string `json:"alg"`
	Use       string `json:"use"`
}

// KeySet is the JSON Web Key Set SAM fetches.
type KeySet struct {
	Keys []JWK `json:"keys"`
}

// KeySet is every key's public half.
func (k *Keys) KeySet() KeySet {
	set := KeySet{Keys: make([]JWK, 0, len(k.published))}
	for _, p := range k.published {
		pub, _ := p.private.Public().(ed25519.PublicKey)
		set.Keys = append(set.Keys, JWK{KeyType: "OKP", Curve: "Ed25519", X: base64.RawURLEncoding.EncodeToString(pub),
			KeyID: p.id, Algorithm: "EdDSA", Use: "sig"})
	}
	return set
}
