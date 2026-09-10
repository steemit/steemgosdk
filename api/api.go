// Package api is the legacy, context-free face of the SDK. Every method
// delegates to the context-aware api/v2 package with context.Background(),
// and therefore now carries a per-attempt timeout (15s) and bounded
// transport-level retry (3 retries, exponential backoff) that the old
// implementation did not have — keep that in mind if your code wraps these
// calls in its own retry loop.
//
// New code should import github.com/steemit/steemgosdk/api/v2 directly and
// pass a context. The old signatures will be removed once steemdb-web and
// steemdb-sync have migrated.
package api

import (
	"context"

	v2 "github.com/steemit/steemgosdk/api/v2"
	"github.com/steemit/steemutil/protocol"
	protocolapi "github.com/steemit/steemutil/protocol/api"
	"github.com/steemit/steemutil/transaction"
)

// WrapBlock represents a block with its block number.
type WrapBlock = v2.WrapBlock

// WrapOpsInBlock represents operations in a block with its block number.
type WrapOpsInBlock = v2.WrapOpsInBlock

// AccountHistoryEntry models a single element of a
// condenser_api.get_account_history response (a [index, body] tuple).
type AccountHistoryEntry = v2.AccountHistoryEntry

// AccountHistoryOp is the [type, payload] pair carried in each account
// history entry's "op" field.
type AccountHistoryOp = v2.AccountHistoryOp

// API provides methods to call Steem RPC APIs. It is a thin facade over
// api/v2; see the package comment for the behavioral notes.
type API struct {
	v2 *v2.API
}

// NewAPI creates a new API instance.
func NewAPI(url string) *API {
	return &API{v2: v2.NewAPI(url)}
}

// SetMaxRetry sets the number of retries after the first attempt (n <= 0
// selects the default, 3). Unlike before, this now actually takes effect.
// Configure before first use.
func (a *API) SetMaxRetry(maxRetry int) {
	a.v2.SetMaxRetry(maxRetry)
}

// validateTransportForSignedCall delegates to api/v2; kept unexported here
// for the legacy tests that call it directly.
func (a *API) validateTransportForSignedCall() error {
	return a.v2.ValidateTransportForSignedCall()
}

// Call makes a generic RPC call to the specified API and method.
//
// Deprecated: Use api/v2 (*v2.API.Call with a context) instead.
func (a *API) Call(apiName, method string, params []interface{}) (*protocolapi.RpcResultData, error) {
	return a.v2.Call(context.Background(), apiName, method, params)
}

// CallWithResult makes an RPC call and unmarshals the result into the provided result object.
//
// Deprecated: Use api/v2 (*v2.API.CallWithResult with a context) instead.
func (a *API) CallWithResult(apiName, method string, params []interface{}, result interface{}) error {
	return a.v2.CallWithResult(context.Background(), apiName, method, params, result)
}

// SignedCall makes a signed RPC call to the specified API method.
// This method is used for authenticated API calls that require proof of account ownership.
// Only HTTP transport is supported for signed calls.
//
// Deprecated: Use api/v2 (*v2.API.SignedCall with a context) instead.
func (a *API) SignedCall(method string, params []interface{}, account string, privateKey string) (*protocolapi.RpcResultData, error) {
	return a.v2.SignedCall(context.Background(), method, params, account, privateKey)
}

// SignedCallWithResult makes a signed RPC call and unmarshals the result into the provided result object.
//
// Deprecated: Use api/v2 (*v2.API.SignedCallWithResult with a context) instead.
func (a *API) SignedCallWithResult(method string, params []interface{}, account string, privateKey string, result interface{}) error {
	return a.v2.SignedCallWithResult(context.Background(), method, params, account, privateKey, result)
}

// GetDynamicGlobalProperties gets the dynamic global properties from the Steem blockchain.
//
// Deprecated: Use api/v2 (*v2.API.GetDynamicGlobalProperties with a context) instead.
func (a *API) GetDynamicGlobalProperties() (*protocolapi.DynamicGlobalProperties, error) {
	return a.v2.GetDynamicGlobalProperties(context.Background())
}

// GetBlock gets a block by block number.
//
// Note: a block number beyond the current chain head now returns
// v2.ErrBlockNotFound on the first attempt (previously it silently returned
// a zero-value block).
//
// Deprecated: Use api/v2 (*v2.API.GetBlock with a context) instead.
func (a *API) GetBlock(blockNum uint) (*protocolapi.Block, error) {
	return a.v2.GetBlock(context.Background(), blockNum)
}

// GetBlocks gets multiple blocks in the range [from, to).
//
// Note: behavior changed with the api/v2 migration — fetches run on a
// bounded worker pool (16 by default) instead of one goroutine per block,
// and partial failures return the successfully fetched blocks alongside a
// *v2.RangeError instead of aborting with a generic error.
//
// Deprecated: Use api/v2 (*v2.API.GetBlocks with a context) instead.
func (a *API) GetBlocks(from, to uint) ([]*WrapBlock, error) {
	return a.v2.GetBlocks(context.Background(), from, to)
}

// GetOpsInBlock gets operations in a block by block number.
// If onlyVirtual is false, returns all operations (both regular and virtual).
// If onlyVirtual is true, returns only virtual operations.
//
// Deprecated: Use api/v2 (*v2.API.GetOpsInBlock with a context) instead.
func (a *API) GetOpsInBlock(blockNum uint, onlyVirtual bool) ([]*protocol.OperationObject, error) {
	return a.v2.GetOpsInBlock(context.Background(), blockNum, onlyVirtual)
}

// GetOpsInBlocks gets operations in multiple blocks in the range [from, to).
// If onlyVirtual is false, returns all operations (both regular and virtual).
// If onlyVirtual is true, returns only virtual operations.
// Returns a map keyed by block number for easy lookup.
//
// Note: behavior changed with the api/v2 migration — bounded worker pool,
// and partial failures return a map of the successful blocks alongside a
// *v2.RangeError.
//
// Deprecated: Use api/v2 (*v2.API.GetOpsInBlocks with a context) instead.
func (a *API) GetOpsInBlocks(from, to uint, onlyVirtual bool) (map[uint][]*protocol.OperationObject, error) {
	return a.v2.GetOpsInBlocks(context.Background(), from, to, onlyVirtual)
}

// GetTransactionHex gets the hexadecimal representation of a transaction.
//
// Deprecated: Use api/v2 (*v2.API.GetTransactionHex with a context) instead.
func (a *API) GetTransactionHex(tx *transaction.SignedTransaction) (interface{}, error) {
	return a.v2.GetTransactionHex(context.Background(), tx)
}

// GetAccounts calls condenser_api.get_accounts.
// The param is a positional array wrapping the names array: [["n1","n2"]].
//
// Deprecated: Use api/v2 (*v2.API.GetAccounts with a context) instead.
func (a *API) GetAccounts(names []string) ([]*protocolapi.ExtendedAccount, error) {
	return a.v2.GetAccounts(context.Background(), names)
}

// GetContent calls condenser_api.get_content.
// The params form a positional array: [author, permlink].
//
// Deprecated: Use api/v2 (*v2.API.GetContent with a context) instead.
func (a *API) GetContent(author, permlink string) (*protocolapi.Content, error) {
	return a.v2.GetContent(context.Background(), author, permlink)
}

// GetFollowCount calls condenser_api.get_follow_count.
// The param is a positional array: [account].
//
// Deprecated: Use api/v2 (*v2.API.GetFollowCount with a context) instead.
func (a *API) GetFollowCount(account string) (*protocolapi.FollowCountReturn, error) {
	return a.v2.GetFollowCount(context.Background(), account)
}

// GetFollowers calls condenser_api.get_followers.
// followType is "blog" or "ignore". The param is a positional array:
// [account, start, followType, limit].
//
// Deprecated: Use api/v2 (*v2.API.GetFollowers with a context) instead.
func (a *API) GetFollowers(account, start, followType string, limit int) ([]*protocolapi.FollowReturn, error) {
	return a.v2.GetFollowers(context.Background(), account, start, followType, limit)
}

// GetFollowing calls condenser_api.get_following.
// followType is "blog" or "ignore". The param is a positional array:
// [account, start, followType, limit].
//
// Deprecated: Use api/v2 (*v2.API.GetFollowing with a context) instead.
func (a *API) GetFollowing(account, start, followType string, limit int) ([]*protocolapi.FollowReturn, error) {
	return a.v2.GetFollowing(context.Background(), account, start, followType, limit)
}

// GetAccountHistory calls condenser_api.get_account_history.
// from/limit follow conveyor's pagination convention (from=-1 for the
// newest page). The param is a positional array: [account, from, limit].
//
// Deprecated: Use api/v2 (*v2.API.GetAccountHistory with a context) instead.
func (a *API) GetAccountHistory(account string, from int64, limit int) ([]*AccountHistoryEntry, error) {
	return a.v2.GetAccountHistory(context.Background(), account, from, limit)
}

// LookupAccounts calls condenser_api.lookup_accounts for account-name
// autocomplete. The param is a positional array: [lowerBound, limit].
//
// Deprecated: Use api/v2 (*v2.API.LookupAccounts with a context) instead.
func (a *API) LookupAccounts(lowerBound string, limit int) ([]string, error) {
	return a.v2.LookupAccounts(context.Background(), lowerBound, limit)
}

// GetOrderBook calls condenser_api.get_order_book.
// The param is a positional array: [limit].
//
// Deprecated: Use api/v2 (*v2.API.GetOrderBook with a context) instead.
func (a *API) GetOrderBook(limit int) (*protocolapi.OrderBook, error) {
	return a.v2.GetOrderBook(context.Background(), limit)
}

// GetFeedHistory calls condenser_api.get_feed_history. Takes no params.
//
// Deprecated: Use api/v2 (*v2.API.GetFeedHistory with a context) instead.
func (a *API) GetFeedHistory() (*protocolapi.FeedHistory, error) {
	return a.v2.GetFeedHistory(context.Background())
}
