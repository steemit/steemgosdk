package client

import (
	sdkapi "github.com/steemit/steemgosdk/api"
	"github.com/steemit/steemgosdk/auth"
	"github.com/steemit/steemgosdk/broadcast"
	"github.com/steemit/steemgosdk/consts"
	protocolapi "github.com/steemit/steemutil/protocol/api"
	"github.com/steemit/steemutil/wif"

	"github.com/pkg/errors"
)

type Client struct {
	Url         string
	MaxRetry    int
	AccountName string
	Wifs        map[string]*wif.PrivateKey
}

func (c *Client) ImportWif(keyType string, privWif string) (err error) {
	if !checkKeyType(keyType) {
		return errors.New("unexpected keyType when import wif\n")
	}
	priv := &wif.PrivateKey{}
	err = priv.FromWif(privWif)
	if err != nil {
		return
	}
	if len(c.Wifs) == 0 {
		c.Wifs = make(map[string]*wif.PrivateKey, 0)
	}
	c.Wifs[keyType] = priv
	return
}

// GetAPI returns an API instance for making RPC calls.
func (c *Client) GetAPI() *sdkapi.API {
	apiClient := sdkapi.NewAPI(c.Url)
	apiClient.SetMaxRetry(c.MaxRetry)
	return apiClient
}

// GetBroadcast returns a Broadcast instance for signing and broadcasting transactions.
func (c *Client) GetBroadcast() *broadcast.Broadcast {
	return broadcast.NewBroadcast(c.Url)
}

// GetAuth returns an Auth instance for authentication and key management.
func (c *Client) GetAuth() *auth.Auth {
	return auth.NewAuth()
}

func checkKeyType(keyType string) (r bool) {
	if keyType == consts.ACTIVE_KEY {
		return true
	}
	if keyType == consts.POSTING_KEY {
		return true
	}
	if keyType == consts.OWNER_KEY {
		return true
	}
	if keyType == consts.MEMO_KEY {
		return true
	}
	return false
}

// SignedCall makes a signed RPC call using the client's stored credentials.
// The keyType parameter specifies which private key to use (e.g., "active", "posting").
func (c *Client) SignedCall(method string, params []interface{}, keyType string) (*protocolapi.RpcResultData, error) {
	if c.AccountName == "" {
		return nil, errors.New("account name not set")
	}

	if !checkKeyType(keyType) {
		return nil, errors.Errorf("invalid key type: %s", keyType)
	}

	privateKey, exists := c.Wifs[keyType]
	if !exists {
		return nil, errors.Errorf("private key for type '%s' not found", keyType)
	}

	// Convert private key to WIF string
	privateKeyWif := privateKey.ToWif()

	// Get API instance and make signed call
	api := c.GetAPI()
	return api.SignedCall(method, params, c.AccountName, privateKeyWif)
}

// SignedCallWithResult makes a signed RPC call and unmarshals the result into the provided result object.
func (c *Client) SignedCallWithResult(method string, params []interface{}, keyType string, result interface{}) error {
	if c.AccountName == "" {
		return errors.New("account name not set")
	}

	if !checkKeyType(keyType) {
		return errors.Errorf("invalid key type: %s", keyType)
	}

	privateKey, exists := c.Wifs[keyType]
	if !exists {
		return errors.Errorf("private key for type '%s' not found", keyType)
	}

	// Convert private key to WIF string
	privateKeyWif := privateKey.ToWif()

	// Get API instance and make signed call
	api := c.GetAPI()
	return api.SignedCallWithResult(method, params, c.AccountName, privateKeyWif, result)
}
