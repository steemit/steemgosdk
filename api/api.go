package api

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/steemit/steemutil/jsonrpc2"
	protocolapi "github.com/steemit/steemutil/protocol/api"
	"github.com/steemit/steemutil/transaction"
)

// API provides methods to call Steem RPC APIs.
type API struct {
	url      string
	maxRetry int
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
func (a *API) wrapGetBlock(blockNum uint, ch chan<- *WrapBlock) {
	var (
		err   error
		block *protocolapi.Block
	)

	for i := 0; i < a.maxRetry; i++ {
		block, err = a.GetBlock(blockNum)
		if err == nil {
			break
		}
		fmt.Printf("Retry get block {%+v} after 1 second.\n", blockNum)
		time.Sleep(time.Second)
	}
	if err != nil {
		fmt.Printf("wrapGetBlock err: %+v\n", err)
		ch <- nil
		return
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
