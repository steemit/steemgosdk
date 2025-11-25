package broadcast

import (
	"encoding/json"
	"time"

	"github.com/pkg/errors"
	"github.com/steemit/steemgosdk/api"
	"github.com/steemit/steemutil/jsonrpc2"
	"github.com/steemit/steemutil/protocol"
	"github.com/steemit/steemutil/transaction"
	"github.com/steemit/steemutil/wif"
)

// Broadcast provides methods to sign and broadcast transactions.
type Broadcast struct {
	url string
	rpc *jsonrpc2.JsonRpc
	api *api.API
}

// NewBroadcast creates a new Broadcast instance.
func NewBroadcast(url string) *Broadcast {
	return &Broadcast{
		url: url,
		rpc: jsonrpc2.NewClient(url),
		api: api.NewAPI(url), // Create API instance internally for getting dynamic global properties
	}
}

// BroadcastSync broadcasts a transaction synchronously to the Steem blockchain.
func (b *Broadcast) BroadcastSync(params []interface{}) (resultJson []byte, err error) {
	err = b.rpc.BuildSendData(
		"condenser_api.broadcast_transaction_synchronous",
		params,
	)
	if err != nil {
		return
	}
	rpcResponse, err := b.rpc.Send()
	if err != nil {
		return
	}
	if rpcResponse.Error != nil {
		return resultJson, errors.Errorf("failed to broadcast:%v\n", rpcResponse.Error)
	}
	resultJson, err = json.Marshal(rpcResponse.Result)
	return
}

// Send signs and broadcasts a transaction.
func (b *Broadcast) Send(ops []protocol.Operation, privKeys map[string]string) ([]byte, error) {
	// Prepare transaction
	tx, err := b.prepareTransaction(ops)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare transaction")
	}

	// Convert WIF strings to PrivateKey objects
	privKeyObjs := make([]*wif.PrivateKey, 0, len(privKeys))
	for _, wifStr := range privKeys {
		privKey := &wif.PrivateKey{}
		if err := privKey.FromWif(wifStr); err != nil {
			return nil, errors.Wrap(err, "failed to decode WIF")
		}
		privKeyObjs = append(privKeyObjs, privKey)
	}

	// Sign transaction
	if err := tx.Sign(privKeyObjs, transaction.SteemChain); err != nil {
		return nil, errors.Wrap(err, "failed to sign transaction")
	}

	// Broadcast transaction
	result, err := b.BroadcastSync([]interface{}{tx})
	if err != nil {
		return nil, errors.Wrap(err, "failed to broadcast transaction")
	}

	return result, nil
}

// prepareTransaction prepares a transaction with proper ref_block_num, ref_block_prefix, and expiration.
func (b *Broadcast) prepareTransaction(ops []protocol.Operation) (*transaction.SignedTransaction, error) {
	if len(ops) == 0 {
		return nil, errors.New("no operations provided")
	}

	// Get dynamic global properties
	dgp, err := b.api.GetDynamicGlobalProperties()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get dynamic global properties")
	}

	// Calculate ref_block_num
	refBlockNum := transaction.RefBlockNum(dgp.HeadBlockNumber)

	// Calculate ref_block_prefix
	refBlockPrefix, err := transaction.RefBlockPrefix(dgp.HeadBlockId)
	if err != nil {
		return nil, errors.Wrap(err, "failed to calculate ref_block_prefix")
	}

	// Set expiration (default: 10 minutes from now)
	expiration := time.Now().Add(600 * time.Second)

	// Create transaction
	tx := transaction.NewSignedTransaction(&transaction.Transaction{
		RefBlockNum:    refBlockNum,
		RefBlockPrefix: refBlockPrefix,
		Expiration:     &protocol.Time{Time: &expiration},
	})

	// Add operations
	for _, op := range ops {
		tx.PushOperation(op)
	}

	return tx, nil
}

// SendWith prepares and sends a transaction with the given operation and private key.
func (b *Broadcast) SendWith(op protocol.Operation, privKeyWif string) ([]byte, error) {
	privKeys := map[string]string{
		"key": privKeyWif,
	}
	return b.Send([]protocol.Operation{op}, privKeys)
}
