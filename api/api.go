package api

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	"github.com/steemit/steemutil/jsonrpc2"
	protocolapi "github.com/steemit/steemutil/protocol/api"
)

// API provides methods to call Steem RPC APIs.
type API struct {
	url string
}

// NewAPI creates a new API instance.
func NewAPI(url string) *API {
	return &API{
		url: url,
	}
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
