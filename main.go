package steemgosdk

import (
	"github.com/steemit/steemgosdk/api"
	"github.com/steemit/steemgosdk/auth"
	"github.com/steemit/steemgosdk/broadcast"
	"github.com/steemit/steemgosdk/client"
	"github.com/steemit/steemgosdk/steemuri"
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

// SteemURI type aliases for the steemuri package.
type SteemURIParameters = steemuri.Parameters
type SteemURIDecodeResult = steemuri.DecodeResult
type SteemURIResolveOptions = steemuri.ResolveOptions
type SteemURIResolveResult = steemuri.ResolveResult
type SteemURITransactionConfirmation = steemuri.TransactionConfirmation
type SteemURIEncodeProtocol = steemuri.EncodeProtocol

// SteemURI function wrappers.
var (
	SteemURIDecode             = steemuri.Decode
	SteemURIResolveTransaction = steemuri.ResolveTransaction
	SteemURIResolveCallback    = steemuri.ResolveCallback
	SteemURIEncodeTx           = steemuri.EncodeTx
	SteemURIEncodeOp           = steemuri.EncodeOp
	SteemURIEncodeOps          = steemuri.EncodeOps
)
