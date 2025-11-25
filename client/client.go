package client

import (
	sdkapi "github.com/steemit/steemgosdk/api"
	"github.com/steemit/steemgosdk/auth"
	"github.com/steemit/steemgosdk/broadcast"
	"github.com/steemit/steemgosdk/consts"
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
