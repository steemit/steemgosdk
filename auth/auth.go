package auth

import (
	"github.com/pkg/errors"
	"github.com/steemit/steemutil/auth"
	"github.com/steemit/steemutil/transaction"
	"github.com/steemit/steemutil/wif"
)

// Auth provides high-level authentication and key management functions.
type Auth struct{}

// NewAuth creates a new Auth instance.
func NewAuth() *Auth {
	return &Auth{}
}

// Verify verifies if the account name and password match the given authorities.
func (a *Auth) Verify(name, password string, auths map[string]interface{}) (bool, error) {
	return auth.Verify(name, password, auths)
}

// GenerateKeys generates public keys for the given roles from account name and password.
func (a *Auth) GenerateKeys(name, password string, roles []string) (map[string]string, error) {
	return auth.GenerateKeys(name, password, roles)
}

// GetPrivateKeys returns private keys and public keys for the given roles.
func (a *Auth) GetPrivateKeys(name, password string, roles []string) (map[string]string, error) {
	return auth.GetPrivateKeys(name, password, roles)
}

// IsWif checks if the given string is a valid WIF format.
func (a *Auth) IsWif(privWif string) bool {
	return auth.IsWif(privWif)
}

// ToWif generates a WIF from account name, password, and role.
func (a *Auth) ToWif(name, password, role string) (string, error) {
	return auth.ToWif(name, password, role)
}

// WifIsValid checks if the given WIF corresponds to the given public key.
func (a *Auth) WifIsValid(privWif, pubKey string) bool {
	return auth.WifIsValid(privWif, pubKey)
}

// WifToPublic converts a WIF to a public key string.
func (a *Auth) WifToPublic(privWif string) (string, error) {
	return auth.WifToPublic(privWif)
}

// IsPubkey checks if the given string is a valid public key format.
func (a *Auth) IsPubkey(pubkey string) bool {
	return auth.IsPubkey(pubkey)
}

// SignTransaction signs a transaction with the given private keys.
func (a *Auth) SignTransaction(tx *transaction.SignedTransaction, keys map[string]string, chain *transaction.Chain) error {
	// Convert WIF strings to PrivateKey objects
	privKeys := make([]*wif.PrivateKey, 0, len(keys))
	for _, wifStr := range keys {
		privKey := &wif.PrivateKey{}
		if err := privKey.FromWif(wifStr); err != nil {
			return errors.Wrap(err, "failed to decode WIF")
		}
		privKeys = append(privKeys, privKey)
	}

	// Sign the transaction
	return tx.Sign(privKeys, chain)
}

// EncodeMemo encrypts a memo if it starts with '#', otherwise returns it as-is.
func (a *Auth) EncodeMemo(privateKey interface{}, publicKey interface{}, memo string) (string, error) {
	return auth.Encode(privateKey, publicKey, memo)
}

// DecodeMemo decrypts a memo if it starts with '#', otherwise returns it as-is.
func (a *Auth) DecodeMemo(privateKey interface{}, memo string) (string, error) {
	return auth.Decode(privateKey, memo)
}
