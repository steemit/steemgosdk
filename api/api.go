package api

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/steemit/steemutil/jsonrpc2"
	"github.com/steemit/steemutil/protocol"
	protocolapi "github.com/steemit/steemutil/protocol/api"
	"github.com/steemit/steemutil/rpc"
	"github.com/steemit/steemutil/transaction"
)

// API provides methods to call Steem RPC APIs.
type API struct {
	url      string
	maxRetry int
	seqNo    int // Sequence number for RPC requests
}

// WrapBlock represents a block with its block number.
type WrapBlock struct {
	BlockNum uint
	Block    *protocolapi.Block
}

// WrapOpsInBlock represents operations in a block with its block number.
type WrapOpsInBlock struct {
	BlockNum   uint
	Operations []*protocol.OperationObject
}

// NewAPI creates a new API instance.
func NewAPI(url string) *API {
	return &API{
		url:      url,
		maxRetry: 5, // default max retry
	}
}

// SetMaxRetry sets the maximum number of retries for API calls.
func (a *API) SetMaxRetry(maxRetry int) {
	a.maxRetry = maxRetry
}

// Call makes a generic RPC call to the specified API and method.
func (a *API) Call(apiName, method string, params []interface{}) (*protocolapi.RpcResultData, error) {
	rpc := jsonrpc2.NewClient(a.url)
	fullMethod := fmt.Sprintf("%s.%s", apiName, method)

	err := rpc.BuildSendData(fullMethod, params)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to build RPC data for %s", fullMethod)
	}

	rpcResponse, err := rpc.Send()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to send RPC request for %s", fullMethod)
	}

	if rpcResponse.Error != nil {
		return nil, errors.Errorf("RPC error for %s: %v", fullMethod, rpcResponse.Error)
	}

	return rpcResponse, nil
}

// SignedCall makes a signed RPC call to the specified API method.
// This method is used for authenticated API calls that require proof of account ownership.
// Only HTTP transport is supported for signed calls.
func (a *API) SignedCall(method string, params []interface{}, account string, privateKey string) (*protocolapi.RpcResultData, error) {
	// Validate that we're using HTTP transport
	if err := a.validateTransportForSignedCall(); err != nil {
		return nil, err
	}

	// Increment sequence number for unique request ID
	a.seqNo++

	// Create RPC request
	request := &rpc.RpcRequest{
		Method: method,
		Params: params,
		ID:     a.seqNo,
	}

	// Sign the request
	signedRequest, err := rpc.Sign(request, account, []string{privateKey})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to sign request for method %s", method)
	}

	// Create JSON-RPC client
	rpcClient := jsonrpc2.NewClient(a.url)

	// Marshal signed request to get the params
	signedParams := map[string]interface{}{
		"__signed": signedRequest.Params.Signed,
	}

	// Build and send the signed request
	err = rpcClient.BuildSendData(method, []interface{}{signedParams})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to build signed RPC data for %s", method)
	}

	rpcResponse, err := rpcClient.Send()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to send signed RPC request for %s", method)
	}

	if rpcResponse.Error != nil {
		return nil, errors.Errorf("signed RPC error for %s: %v", method, rpcResponse.Error)
	}

	return rpcResponse, nil
}

// SignedCallWithResult makes a signed RPC call and unmarshals the result into the provided result object.
func (a *API) SignedCallWithResult(method string, params []interface{}, account string, privateKey string, result interface{}) error {
	rpcResponse, err := a.SignedCall(method, params, account, privateKey)
	if err != nil {
		return err
	}

	resultBytes, err := json.Marshal(rpcResponse.Result)
	if err != nil {
		return errors.Wrapf(err, "failed to marshal signed RPC result for %s", method)
	}

	if err := json.Unmarshal(resultBytes, result); err != nil {
		return errors.Wrapf(err, "failed to unmarshal signed RPC result for %s", method)
	}

	return nil
}

// validateTransportForSignedCall ensures that signed calls are only made over HTTP.
// WebSocket transport is not supported for signed calls due to the nature of the signing protocol.
func (a *API) validateTransportForSignedCall() error {
	// Check if URL uses HTTP/HTTPS
	if !strings.HasPrefix(a.url, "http://") && !strings.HasPrefix(a.url, "https://") {
		return errors.New("signed calls can only be made when using HTTP transport")
	}
	return nil
}

// CallWithResult makes an RPC call and unmarshals the result into the provided result object.
func (a *API) CallWithResult(apiName, method string, params []interface{}, result interface{}) error {
	rpcResponse, err := a.Call(apiName, method, params)
	if err != nil {
		return err
	}

	// Marshal and unmarshal to convert to the target type
	tmp, err := json.Marshal(rpcResponse.Result)
	if err != nil {
		return errors.Wrap(err, "failed to marshal RPC result")
	}

	if err := json.Unmarshal(tmp, result); err != nil {
		return errors.Wrap(err, "failed to unmarshal RPC result")
	}

	return nil
}

// GetDynamicGlobalProperties gets the dynamic global properties from the Steem blockchain.
func (a *API) GetDynamicGlobalProperties() (dgp *protocolapi.DynamicGlobalProperties, err error) {
	rpc := jsonrpc2.NewClient(a.url)
	err = rpc.BuildSendData(
		"condenser_api.get_dynamic_global_properties",
		[]any{},
	)
	if err != nil {
		return
	}
	rpcResponse, err := rpc.Send()
	if err != nil {
		return
	}
	if rpcResponse.Error != nil {
		return dgp, errors.Errorf("failed to GetDynamicGlobalProperties:%v\n", rpcResponse.Error)
	}
	tmp, err := json.Marshal(rpcResponse.Result)
	if err != nil {
		return
	}
	dgp = &protocolapi.DynamicGlobalProperties{}
	json.Unmarshal(tmp, dgp)
	return
}

// GetBlock gets a block by block number.
func (a *API) GetBlock(blockNum uint) (block *protocolapi.Block, err error) {
	rpc := jsonrpc2.NewClient(a.url)
	err = rpc.BuildSendData(
		"condenser_api.get_block",
		[]any{blockNum},
	)
	if err != nil {
		return
	}
	rpcResponse, err := rpc.Send()
	if err != nil {
		return
	}
	if rpcResponse.Error != nil {
		return block, errors.Errorf("failed to GetBlock:%v\n", rpcResponse.Error)
	}
	tmp, err := json.Marshal(rpcResponse.Result)
	if err != nil {
		return
	}
	block = &protocolapi.Block{}
	json.Unmarshal(tmp, block)
	return
}

// wrapGetBlock is a helper function that gets a block with retry logic.
// This will ALWAYS eventually return, at all costs (similar to steem-python's reliable_query).
func (a *API) wrapGetBlock(blockNum uint, ch chan<- *WrapBlock) {
	var (
		err   error
		block *protocolapi.Block
	)

	for {
		block, err = a.GetBlock(blockNum)
		if err == nil {
			break
		}
		fmt.Printf("Retry get block {%+v}: %v\n", blockNum, err)
	}
	ch <- &WrapBlock{
		BlockNum: blockNum,
		Block:    block,
	}
}

// GetBlocks gets multiple blocks in the range [from, to).
func (a *API) GetBlocks(from, to uint) (blocks []*WrapBlock, err error) {
	// check params
	if from >= to {
		return blocks, errors.Errorf("unexpected params {from: %v}, {to: %v}\n", from, to)
	}
	// init
	ch := make(chan *WrapBlock, to-from)
	blocksMap := make(map[uint]*WrapBlock, to-from)
	blocks = make([]*WrapBlock, 0, to-from)
	// get blocks
	for i := from; i < to; i++ {
		go a.wrapGetBlock(i, ch)
	}
	// get results
	for i := from; i < to; i++ {
		result := <-ch
		blocksMap[result.BlockNum] = result
		if blocksMap[result.BlockNum] == nil {
			return blocks, errors.Errorf("get block {%v} error\n", result.BlockNum)
		}
	}
	// sort result
	for i := from; i < to; i++ {
		blocks = append(blocks, blocksMap[i])
	}
	return
}

// GetTransactionHex gets the hexadecimal representation of a transaction.
func (a *API) GetTransactionHex(tx *transaction.SignedTransaction) (result any, err error) {
	rpc := jsonrpc2.NewClient(a.url)
	err = rpc.BuildSendData(
		"condenser_api.get_transaction_hex",
		[]any{tx.Transaction},
	)
	if err != nil {
		return
	}
	rpcResponse, err := rpc.Send()
	if err != nil {
		return
	}
	if rpcResponse.Error != nil {
		return result, errors.Errorf("failed to GetTransactionHex:%v\n", rpcResponse.Error)
	}
	result = rpcResponse.Result
	return
}

// GetOpsInBlock gets operations in a block by block number.
// If onlyVirtual is false, returns all operations (both regular and virtual).
// If onlyVirtual is true, returns only virtual operations.
func (a *API) GetOpsInBlock(blockNum uint, onlyVirtual bool) (ops []*protocol.OperationObject, err error) {
	rpc := jsonrpc2.NewClient(a.url)
	err = rpc.BuildSendData(
		"condenser_api.get_ops_in_block",
		[]any{blockNum, onlyVirtual},
	)
	if err != nil {
		return
	}
	rpcResponse, err := rpc.Send()
	if err != nil {
		return
	}
	if rpcResponse.Error != nil {
		return ops, errors.Errorf("failed to GetOpsInBlock:%v\n", rpcResponse.Error)
	}
	tmp, err := json.Marshal(rpcResponse.Result)
	if err != nil {
		return
	}
	err = json.Unmarshal(tmp, &ops)
	return
}

// wrapGetOpsInBlock is a helper function that gets operations in a block with retry logic.
// This will ALWAYS eventually return, at all costs (similar to steem-python's reliable_query).
func (a *API) wrapGetOpsInBlock(blockNum uint, onlyVirtual bool, ch chan<- *WrapOpsInBlock) {
	var (
		err error
		ops []*protocol.OperationObject
	)

	for {
		ops, err = a.GetOpsInBlock(blockNum, onlyVirtual)
		if err == nil {
			break
		}
		fmt.Printf("Retry get ops in block {%+v}: %v\n", blockNum, err)
	}
	ch <- &WrapOpsInBlock{
		BlockNum:   blockNum,
		Operations: ops,
	}
}

// GetOpsInBlocks gets operations in multiple blocks in the range [from, to).
// If onlyVirtual is false, returns all operations (both regular and virtual).
// If onlyVirtual is true, returns only virtual operations.
// Returns a map keyed by block number for easy lookup.
func (a *API) GetOpsInBlocks(from, to uint, onlyVirtual bool) (opsMap map[uint][]*protocol.OperationObject, err error) {
	// check params
	if from >= to {
		return opsMap, errors.Errorf("unexpected params {from: %v}, {to: %v}\n", from, to)
	}
	// init
	ch := make(chan *WrapOpsInBlock, to-from)
	opsMap = make(map[uint][]*protocol.OperationObject, to-from)
	// get operations
	for i := from; i < to; i++ {
		go a.wrapGetOpsInBlock(i, onlyVirtual, ch)
	}
	// get results
	for i := from; i < to; i++ {
		result := <-ch
		opsMap[result.BlockNum] = result.Operations
	}
	return
}

// ---------------------------------------------------------------------------
// Conveyor-facing convenience wrappers (G2).
//
// Each method below is a typed wrapper over CallWithResult for the
// condenser_api / database_api calls that conveyor relies on heavily
// (user-search, prices). Result structs are reused from steemutil's
// protocol/api package; only AccountHistoryEntry (a [index, body] tuple) is
// defined locally because its wire form is not a plain object.
//
// Note on parameter shape: condenser_api takes positional array params, not
// named objects — e.g. get_accounts takes [["n1","n2"]] (array wrapping an
// array), get_followers takes [account, start, followType, limit]. This is
// verified against conveyor/src/user-search/client.ts.
// ---------------------------------------------------------------------------

// GetAccounts calls condenser_api.get_accounts.
// The param is a positional array wrapping the names array: [["n1","n2"]].
func (a *API) GetAccounts(names []string) ([]*protocolapi.ExtendedAccount, error) {
	var result []*protocolapi.ExtendedAccount
	if err := a.CallWithResult(
		"condenser_api", "get_accounts",
		[]interface{}{names},
		&result,
	); err != nil {
		return nil, errors.Wrap(err, "failed to GetAccounts")
	}
	return result, nil
}

// GetFollowCount calls condenser_api.get_follow_count.
// The param is a positional array: [account].
func (a *API) GetFollowCount(account string) (*protocolapi.FollowCountReturn, error) {
	var result protocolapi.FollowCountReturn
	if err := a.CallWithResult(
		"condenser_api", "get_follow_count",
		[]interface{}{account},
		&result,
	); err != nil {
		return nil, errors.Wrap(err, "failed to GetFollowCount")
	}
	return &result, nil
}

// GetFollowers calls condenser_api.get_followers.
// followType is "blog" or "ignore". The param is a positional array:
// [account, start, followType, limit].
func (a *API) GetFollowers(account, start, followType string, limit int) ([]*protocolapi.FollowReturn, error) {
	var result []*protocolapi.FollowReturn
	if err := a.CallWithResult(
		"condenser_api", "get_followers",
		[]interface{}{account, start, followType, limit},
		&result,
	); err != nil {
		return nil, errors.Wrap(err, "failed to GetFollowers")
	}
	return result, nil
}

// GetFollowing calls condenser_api.get_following.
// followType is "blog" or "ignore". The param is a positional array:
// [account, start, followType, limit].
func (a *API) GetFollowing(account, start, followType string, limit int) ([]*protocolapi.FollowReturn, error) {
	var result []*protocolapi.FollowReturn
	if err := a.CallWithResult(
		"condenser_api", "get_following",
		[]interface{}{account, start, followType, limit},
		&result,
	); err != nil {
		return nil, errors.Wrap(err, "failed to GetFollowing")
	}
	return result, nil
}

// GetAccountHistory calls condenser_api.get_account_history.
//
// from/limit follow conveyor's accountHistoryGenerator pagination convention
// (src/user-search/client.ts): the first page uses from=-1 (newest), limit=1000;
// subsequent pages step pointer backwards by limit, flooring at limit. This SDK
// method returns a single page; the paging loop itself is driven by the caller.
//
// The param is a positional array: [account, from, limit].
func (a *API) GetAccountHistory(account string, from int64, limit int) ([]*AccountHistoryEntry, error) {
	var result []*AccountHistoryEntry
	if err := a.CallWithResult(
		"condenser_api", "get_account_history",
		[]interface{}{account, from, limit},
		&result,
	); err != nil {
		return nil, errors.Wrap(err, "failed to GetAccountHistory")
	}
	return result, nil
}

// LookupAccounts calls condenser_api.lookup_accounts for account-name
// autocomplete. lowerBound is the prefix to search after; limit caps the
// number of names returned. The param is a positional array:
// [lowerBound, limit]. Returns the matched account names.
func (a *API) LookupAccounts(lowerBound string, limit int) ([]string, error) {
	var result []string
	if err := a.CallWithResult(
		"condenser_api", "lookup_accounts",
		[]interface{}{lowerBound, limit},
		&result,
	); err != nil {
		return nil, errors.Wrap(err, "failed to LookupAccounts")
	}
	return result, nil
}

// GetOrderBook calls condenser_api.get_order_book.
//
// condenser_api is used (rather than database_api) because condenser_api's
// handlers take positional-array params, which is what this SDK's jsonrpc2
// layer emits. database_api's get_order_book takes a named object {"limit":N},
// which the current RpcSendData.Params ([]any) cannot represent.
//
// The param is a positional array: [limit].
//
// Convention note: steemd's json_rpc plugin supports two wire forms — the
// dotted "api.method" form (used here, the current/preferred path per
// json_rpc_plugin.cpp:288) and the legacy "call" form where method=="call"
// and api/method are packed into params. New code uses the dotted form; the
// "call" form is retained only for backwards compatibility and is slated for
// retirement when steemutil's jsonrpc2 is cleaned up.
func (a *API) GetOrderBook(limit int) (*protocolapi.OrderBook, error) {
	var result protocolapi.OrderBook
	if err := a.CallWithResult(
		"condenser_api", "get_order_book",
		[]interface{}{limit},
		&result,
	); err != nil {
		return nil, errors.Wrap(err, "failed to GetOrderBook")
	}
	return &result, nil
}

// GetFeedHistory calls condenser_api.get_feed_history.
//
// condenser_api is used (rather than database_api) for the same reason as
// GetOrderBook: condenser_api takes positional-array params (here an empty
// array), matching what this SDK's jsonrpc2 layer emits. database_api's
// get_feed_history takes a named object, which the current Params ([]any)
// cannot represent. Takes no params.
//
// See GetOrderBook for the dotted-form-vs-call-form convention note.
func (a *API) GetFeedHistory() (*protocolapi.FeedHistory, error) {
	var result protocolapi.FeedHistory
	if err := a.CallWithResult(
		"condenser_api", "get_feed_history",
		[]interface{}{},
		&result,
	); err != nil {
		return nil, errors.Wrap(err, "failed to GetFeedHistory")
	}
	return &result, nil
}
