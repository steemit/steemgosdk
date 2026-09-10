package v2

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/steemit/steemutil/jsonrpc2"
	"github.com/steemit/steemutil/protocol"
	protocolapi "github.com/steemit/steemutil/protocol/api"
	"github.com/steemit/steemutil/rpc"
	"github.com/steemit/steemutil/transaction"
)

// ---------------------------------------------------------------------------
// Generic RPC surface
// ---------------------------------------------------------------------------

// Call makes a generic RPC call to the specified API and method, with bounded
// transport-level retry for transient failures.
func (a *API) Call(ctx context.Context, apiName, method string, params []interface{}) (*protocolapi.RpcResultData, error) {
	return a.call(ctx, fmt.Sprintf("%s.%s", apiName, method), params)
}

// CallWithResult makes an RPC call and unmarshals the result into result.
func (a *API) CallWithResult(ctx context.Context, apiName, method string, params []interface{}, result interface{}) error {
	rpcResponse, err := a.Call(ctx, apiName, method, params)
	if err != nil {
		return err
	}

	// Marshal and unmarshal to convert to the target type
	tmp, err := json.Marshal(rpcResponse.Result)
	if err != nil {
		return fmt.Errorf("steemgosdk: failed to marshal RPC result: %w", err)
	}
	if err := json.Unmarshal(tmp, result); err != nil {
		return fmt.Errorf("steemgosdk: failed to unmarshal RPC result: %w", err)
	}
	return nil
}

// SignedCall makes a signed RPC call for authenticated APIs that require
// proof of account ownership. Only HTTP transport is supported.
//
// Unlike the rest of this package, signed calls are sent exactly once — no
// transport-level retry — because a signed request may be non-idempotent on
// the node (a request that landed but whose response was lost would be
// re-executed by a naive retry). Per-attempt timeout and ctx cancellation
// still apply.
func (a *API) SignedCall(ctx context.Context, method string, params []interface{}, account string, privateKey string) (*protocolapi.RpcResultData, error) {
	if err := a.validateTransportForSignedCall(); err != nil {
		return nil, err
	}

	seq := atomic.AddInt64(&a.seqNo, 1)

	request := &rpc.RpcRequest{
		Method: method,
		Params: params,
		ID:     int(seq),
	}

	signedRequest, err := rpc.Sign(request, account, []string{privateKey})
	if err != nil {
		return nil, fmt.Errorf("steemgosdk: failed to sign request for method %s: %w", method, err)
	}

	rpcClient := jsonrpc2.NewClientWithOptions(a.url, jsonrpc2.WithHTTPClient(a.httpClient))
	signedParams := map[string]interface{}{
		"__signed": signedRequest.Params.Signed,
	}
	if err := rpcClient.BuildSendData(method, []interface{}{signedParams}); err != nil {
		return nil, fmt.Errorf("steemgosdk: failed to build signed RPC data for %s: %w", method, err)
	}

	rpcResponse, err := rpcClient.SendWithContext(ctx)
	if err != nil {
		if status, ok := httpStatusFromTransportError(err); ok {
			rpcErr := &RPCError{HTTPStatus: status, Message: err.Error()}
			if status == 429 {
				return nil, fmt.Errorf("%w: %v", ErrRateLimited, rpcErr)
			}
			return nil, rpcErr
		}
		if asTimeoutError(err) {
			return nil, fmt.Errorf("%w: %v", ErrTimeout, err)
		}
		return nil, err
	}
	if rpcResponse.Error != nil {
		rpcErr := rpcErrorFromObject(rpcResponse.Error)
		return nil, fmt.Errorf("steemgosdk: signed RPC error for %s: %w", method, rpcErr)
	}
	return rpcResponse, nil
}

// SignedCallWithResult makes a signed RPC call and unmarshals the result into
// the provided result object.
func (a *API) SignedCallWithResult(ctx context.Context, method string, params []interface{}, account string, privateKey string, result interface{}) error {
	rpcResponse, err := a.SignedCall(ctx, method, params, account, privateKey)
	if err != nil {
		return err
	}

	resultBytes, err := json.Marshal(rpcResponse.Result)
	if err != nil {
		return fmt.Errorf("steemgosdk: failed to marshal signed RPC result for %s: %w", method, err)
	}
	if err := json.Unmarshal(resultBytes, result); err != nil {
		return fmt.Errorf("steemgosdk: failed to unmarshal signed RPC result for %s: %w", method, err)
	}
	return nil
}

// validateTransportForSignedCall ensures that signed calls are only made over
// HTTP. WebSocket transport is not supported for signed calls due to the
// nature of the signing protocol.
func (a *API) validateTransportForSignedCall() error {
	if !strings.HasPrefix(a.url, "http://") && !strings.HasPrefix(a.url, "https://") {
		return fmt.Errorf("steemgosdk: signed calls can only be made when using HTTP transport")
	}
	return nil
}

// ---------------------------------------------------------------------------
// Typed block/ops API
// ---------------------------------------------------------------------------

// GetDynamicGlobalProperties gets the dynamic global properties from the
// Steem blockchain. The catch-up loop convention is to use
// LastIrreversibleBlockNum from this response as the fetch window end —
// never probe for chain head via error paths.
func (a *API) GetDynamicGlobalProperties(ctx context.Context) (*protocolapi.DynamicGlobalProperties, error) {
	resp, err := a.call(ctx, "condenser_api.get_dynamic_global_properties", []interface{}{})
	if err != nil {
		return nil, err
	}
	tmp, err := json.Marshal(resp.Result)
	if err != nil {
		return nil, fmt.Errorf("steemgosdk: failed to marshal DGP result: %w", err)
	}
	dgp := &protocolapi.DynamicGlobalProperties{}
	if err := json.Unmarshal(tmp, dgp); err != nil {
		return nil, fmt.Errorf("steemgosdk: failed to unmarshal DGP result: %w", err)
	}
	return dgp, nil
}

// GetBlock gets a block by number.
//
// A `null` result (block number beyond the current head — Steem block numbers
// are contiguous, so null means "not produced yet", never "gap") returns
// ErrBlockNotFound on the first attempt without retrying. Prefer
// GetDynamicGlobalProperties for chain-head detection; this error is a
// defensive signal.
func (a *API) GetBlock(ctx context.Context, blockNum uint) (*protocolapi.Block, error) {
	resp, err := a.call(ctx, "condenser_api.get_block", []interface{}{blockNum})
	if err != nil {
		return nil, err
	}
	if resp.Result == nil {
		return nil, fmt.Errorf("%w: condenser_api.get_block returned null for block %d", ErrBlockNotFound, blockNum)
	}
	tmp, err := json.Marshal(resp.Result)
	if err != nil {
		return nil, fmt.Errorf("steemgosdk: failed to marshal block result: %w", err)
	}
	block := &protocolapi.Block{}
	if err := json.Unmarshal(tmp, block); err != nil {
		return nil, fmt.Errorf("steemgosdk: failed to unmarshal block result: %w", err)
	}
	return block, nil
}

// GetOpsInBlock gets operations in a block by block number.
// If onlyVirtual is false, returns all operations (both regular and virtual).
// If onlyVirtual is true, returns only virtual operations.
//
// Note: an empty result is ambiguous — a block with no operations and a
// block number beyond the head both return an empty slice. Chain-head
// detection must not rely on this method.
func (a *API) GetOpsInBlock(ctx context.Context, blockNum uint, onlyVirtual bool) ([]*protocol.OperationObject, error) {
	resp, err := a.call(ctx, "condenser_api.get_ops_in_block", []interface{}{blockNum, onlyVirtual})
	if err != nil {
		return nil, err
	}
	tmp, err := json.Marshal(resp.Result)
	if err != nil {
		return nil, fmt.Errorf("steemgosdk: failed to marshal ops result: %w", err)
	}
	var ops []*protocol.OperationObject
	if err := json.Unmarshal(tmp, &ops); err != nil {
		return nil, fmt.Errorf("steemgosdk: failed to unmarshal ops result: %w", err)
	}
	if ops == nil {
		// Normalize a null result to an empty slice: success must always
		// yield a non-nil (possibly empty) result.
		ops = []*protocol.OperationObject{}
	}
	return ops, nil
}

// GetTransactionHex gets the hexadecimal representation of a transaction.
func (a *API) GetTransactionHex(ctx context.Context, tx *transaction.SignedTransaction) (interface{}, error) {
	resp, err := a.call(ctx, "condenser_api.get_transaction_hex", []interface{}{tx.Transaction})
	if err != nil {
		return nil, err
	}
	return resp.Result, nil
}

// ---------------------------------------------------------------------------
// Conveyor-facing convenience wrappers (G2).
//
// Each method below is a typed wrapper over CallWithResult for the
// condenser_api calls that conveyor relies on heavily (user-search, prices).
// Result structs are reused from steemutil's protocol/api package; only
// AccountHistoryEntry (a [index, body] tuple) is defined locally because its
// wire form is not a plain object.
//
// Note on parameter shape: condenser_api takes positional array params, not
// named objects — e.g. get_accounts takes [["n1","n2"]] (array wrapping an
// array), get_followers takes [account, start, followType, limit]. This is
// verified against conveyor/src/user-search/client.ts.
// ---------------------------------------------------------------------------

// GetAccounts calls condenser_api.get_accounts.
// The param is a positional array wrapping the names array: [["n1","n2"]].
func (a *API) GetAccounts(ctx context.Context, names []string) ([]*protocolapi.ExtendedAccount, error) {
	var result []*protocolapi.ExtendedAccount
	if err := a.CallWithResult(ctx, "condenser_api", "get_accounts", []interface{}{names}, &result); err != nil {
		return nil, fmt.Errorf("steemgosdk: GetAccounts: %w", err)
	}
	return result, nil
}

// GetContent calls condenser_api.get_content.
// The params form a positional array: [author, permlink].
func (a *API) GetContent(ctx context.Context, author, permlink string) (*protocolapi.Content, error) {
	var result protocolapi.Content
	if err := a.CallWithResult(ctx, "condenser_api", "get_content", []interface{}{author, permlink}, &result); err != nil {
		return nil, fmt.Errorf("steemgosdk: GetContent: %w", err)
	}
	return &result, nil
}

// GetFollowCount calls condenser_api.get_follow_count.
// The param is a positional array: [account].
func (a *API) GetFollowCount(ctx context.Context, account string) (*protocolapi.FollowCountReturn, error) {
	var result protocolapi.FollowCountReturn
	if err := a.CallWithResult(ctx, "condenser_api", "get_follow_count", []interface{}{account}, &result); err != nil {
		return nil, fmt.Errorf("steemgosdk: GetFollowCount: %w", err)
	}
	return &result, nil
}

// GetFollowers calls condenser_api.get_followers.
// followType is "blog" or "ignore". The param is a positional array:
// [account, start, followType, limit].
func (a *API) GetFollowers(ctx context.Context, account, start, followType string, limit int) ([]*protocolapi.FollowReturn, error) {
	var result []*protocolapi.FollowReturn
	if err := a.CallWithResult(ctx, "condenser_api", "get_followers", []interface{}{account, start, followType, limit}, &result); err != nil {
		return nil, fmt.Errorf("steemgosdk: GetFollowers: %w", err)
	}
	return result, nil
}

// GetFollowing calls condenser_api.get_following.
// followType is "blog" or "ignore". The param is a positional array:
// [account, start, followType, limit].
func (a *API) GetFollowing(ctx context.Context, account, start, followType string, limit int) ([]*protocolapi.FollowReturn, error) {
	var result []*protocolapi.FollowReturn
	if err := a.CallWithResult(ctx, "condenser_api", "get_following", []interface{}{account, start, followType, limit}, &result); err != nil {
		return nil, fmt.Errorf("steemgosdk: GetFollowing: %w", err)
	}
	return result, nil
}

// GetAccountHistory calls condenser_api.get_account_history.
//
// from/limit follow conveyor's accountHistoryGenerator pagination convention:
// the first page uses from=-1 (newest), limit=1000; subsequent pages step the
// pointer backwards by limit, flooring at limit. This SDK method returns a
// single page; the paging loop itself is driven by the caller.
//
// The param is a positional array: [account, from, limit].
func (a *API) GetAccountHistory(ctx context.Context, account string, from int64, limit int) ([]*AccountHistoryEntry, error) {
	var result []*AccountHistoryEntry
	if err := a.CallWithResult(ctx, "condenser_api", "get_account_history", []interface{}{account, from, limit}, &result); err != nil {
		return nil, fmt.Errorf("steemgosdk: GetAccountHistory: %w", err)
	}
	return result, nil
}

// LookupAccounts calls condenser_api.lookup_accounts for account-name
// autocomplete. lowerBound is the prefix to search after; limit caps the
// number of names returned. The param is a positional array:
// [lowerBound, limit]. Returns the matched account names.
func (a *API) LookupAccounts(ctx context.Context, lowerBound string, limit int) ([]string, error) {
	var result []string
	if err := a.CallWithResult(ctx, "condenser_api", "lookup_accounts", []interface{}{lowerBound, limit}, &result); err != nil {
		return nil, fmt.Errorf("steemgosdk: LookupAccounts: %w", err)
	}
	return result, nil
}

// GetOrderBook calls condenser_api.get_order_book.
//
// condenser_api is used (rather than database_api) because condenser_api's
// handlers take positional-array params, which is what this SDK's jsonrpc2
// layer emits. database_api's get_order_book takes a named object {"limit":N},
// which RpcSendData.Params ([]any) cannot represent.
//
// The param is a positional array: [limit].
func (a *API) GetOrderBook(ctx context.Context, limit int) (*protocolapi.OrderBook, error) {
	var result protocolapi.OrderBook
	if err := a.CallWithResult(ctx, "condenser_api", "get_order_book", []interface{}{limit}, &result); err != nil {
		return nil, fmt.Errorf("steemgosdk: GetOrderBook: %w", err)
	}
	return &result, nil
}

// GetFeedHistory calls condenser_api.get_feed_history.
//
// condenser_api is used (rather than database_api) for the same reason as
// GetOrderBook: condenser_api takes positional-array params (here an empty
// array), matching what this SDK's jsonrpc2 layer emits. Takes no params.
func (a *API) GetFeedHistory(ctx context.Context) (*protocolapi.FeedHistory, error) {
	var result protocolapi.FeedHistory
	if err := a.CallWithResult(ctx, "condenser_api", "get_feed_history", []interface{}{}, &result); err != nil {
		return nil, fmt.Errorf("steemgosdk: GetFeedHistory: %w", err)
	}
	return &result, nil
}
