package steemgosdk

import (
	"github.com/steemit/steemgosdk/api"
	"github.com/steemit/steemgosdk/auth"
	"github.com/steemit/steemgosdk/broadcast"
	"github.com/steemit/steemgosdk/client"
)

// GetClient creates a new Client instance.
func GetClient(url string) *client.Client {
	return &client.Client{
		Url:      url,
		MaxRetry: 5,
	}
}

// Client represents the main Steem SDK client.
type Client = client.Client

// API represents the API layer for making RPC calls.
type API = api.API

// Broadcast represents the broadcast layer for signing and broadcasting transactions.
type Broadcast = broadcast.Broadcast

// Auth represents the auth layer for authentication and key management.
type Auth = auth.Auth
