package api

import (
	"github.com/pkg/errors"
	protocolapi "github.com/steemit/steemutil/protocol/api"
	"github.com/steemit/steemutil/rpc"
)

// VerifySignedRequest verifies a received __signed JSON-RPC request against the
// signing account's posting authority, fetched from the Steem node at a.url.
//
// It is the server-side counterpart to SignedCall (which signs and sends).
// conveyor and other JSON-RPC 2.0 servers that authenticate callers by Steem
// account ownership use this to verify incoming signed requests, mirroring the
// behavior of @steemit/koa-jsonrpc.
//
// On success it returns the decoded plaintext params and the authenticated
// account name. The params are returned as interface{} (matching steemutil
// v0.0.26+ rpc.Validate), preserving whatever JSON shape the client signed —
// typically a JSON object (e.g. {"account":"foo"}) for conveyor/koa-jsonrpc
// clients, but arrays and scalars are also valid. On failure the error comes
// from steemutil's rpc.Validate (envelope/freshness) or rpc.VerifySignedRpc
// (cryptographic check).
func (a *API) VerifySignedRequest(signedReq *rpc.SignedRequest) (params interface{}, account string, err error) {
	account = signedReq.Params.Signed.Account

	// accountFetcher implements rpc.AccountFetcher by looking up the account's
	// posting authority via condenser_api.get_accounts. It returns the
	// ExtendedAccount.Posting field directly — its type (protocolapi.Authority)
	// is exactly what AccountFetcher expects, so no conversion glue is needed.
	// A not-found account (or any get_accounts error) is surfaced by
	// VerifySignedRpc as "No such account".
	accountFetcher := func(name string) (protocolapi.Authority, error) {
		accts, ferr := a.GetAccounts([]string{name})
		if ferr != nil {
			return protocolapi.Authority{}, errors.Wrapf(ferr, "failed to fetch account %s", name)
		}
		if len(accts) == 0 {
			return protocolapi.Authority{}, errors.Errorf("no such account: %s", name)
		}
		return accts[0].Posting, nil
	}

	// Validate handles the envelope (jsonrpc/method presence, 60s freshness,
	// nonce decode, message digest) and then calls our verifier, which binds
	// the fetcher into rpc.VerifySignedRpc.
	params, err = rpc.Validate(signedReq, func(message []byte, signatures []string, acct string) error {
		return rpc.VerifySignedRpc(message, signatures, acct, accountFetcher)
	})
	if err != nil {
		return nil, account, err
	}

	return params, account, nil
}
