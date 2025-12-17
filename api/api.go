package api

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/steemit/steemutil/jsonrpc2"
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
