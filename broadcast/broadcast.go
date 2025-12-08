package broadcast

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"
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

	// Print debug information if DEBUG environment variable is set
	if os.Getenv("DEBUG") != "" {
		// Print transaction for testing
		txJSON, err := json.MarshalIndent(tx.Transaction, "", "  ")
		if err != nil {
			return nil, errors.Wrap(err, "failed to marshal transaction to JSON")
		}
		fmt.Printf("=== Transaction (before signing) ===\n%s\n", string(txJSON))

		// Serialize and print transaction bytes for testing
		txBytes, err := tx.Serialize()
		if err != nil {
			return nil, errors.Wrap(err, "failed to serialize transaction")
		}
		fmt.Printf("=== Transaction Bytes (hex) ===\n%s\n", hex.EncodeToString(txBytes))

		// Compute and print digest for testing
		digest, err := tx.Digest(transaction.SteemChain)
		if err != nil {
			return nil, errors.Wrap(err, "failed to compute digest")
		}
		fmt.Printf("=== Digest (hex) ===\n%s\n", hex.EncodeToString(digest))
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

	// Debug: Verify signature recovery if DEBUG is set
	if os.Getenv("DEBUG") != "" && len(tx.Transaction.Signatures) > 0 {
		digest, err := tx.Digest(transaction.SteemChain)
		if err == nil {
			// Decode first signature
			sigHex := tx.Transaction.Signatures[0]
			sigBytes, err := hex.DecodeString(sigHex)
			if err == nil {
				// Recover public key from signature
				recoveredPubKey, err := wif.RecoverPublicKeyFromSignature(digest, sigBytes)
				if err == nil {
					recoveredPubKeyStr := recoveredPubKey.ToStr()
					fmt.Printf("=== Signature Recovery ===\n")
					fmt.Printf("Recovered Public Key from Signature: %s\n", recoveredPubKeyStr)

					// Compare with expected public key from first private key
					if len(privKeyObjs) > 0 {
						expectedPubKeyStr := privKeyObjs[0].ToPubKeyStr()
						fmt.Printf("Expected Public Key (from WIF): %s\n", expectedPubKeyStr)
						if recoveredPubKeyStr == expectedPubKeyStr {
							fmt.Printf("✅ Recovered public key matches expected public key\n")
						} else {
							fmt.Printf("❌ Recovered public key does NOT match expected public key\n")
						}
					}
				}
			}
		}
	}

	// Debug: Print signed transaction before broadcast
	if os.Getenv("DEBUG") != "" {
		txJSON, err := json.MarshalIndent(tx.Transaction, "", "  ")
		if err == nil {
			fmt.Printf("=== Transaction (after signing, before broadcast) ===\n%s\n", string(txJSON))
		}
		// Print signatures
		if len(tx.Transaction.Signatures) > 0 {
			fmt.Printf("=== Signatures ===\n")
			for i, sig := range tx.Transaction.Signatures {
				fmt.Printf("Signature %d: %s\n", i, sig)
			}
		}
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

	// Calculate ref_block_num from last_irreversible_block_num
	// ref_block_num = (last_irreversible_block_num - 1) & 0xFFFF
	refBlockNum := transaction.RefBlockNum(protocol.UInt32((dgp.LastIrreversibleBlockNum - 1) & 0xFFFF))

	// Get the block at last_irreversible_block_num to get its previous block ID
	block, err := b.api.GetBlock(uint(dgp.LastIrreversibleBlockNum))
	if err != nil {
		return nil, errors.Wrap(err, "failed to get block for ref_block_prefix calculation")
	}

	// Calculate ref_block_prefix from the previous block ID (not the current block ID)
	// This matches steemjs behavior: block.previous
	previousBlockId := block.Previous
	if previousBlockId == "" {
		// Fallback to all zeros if previous is not available
		previousBlockId = "0000000000000000000000000000000000000000"
	}
	refBlockPrefix, err := transaction.RefBlockPrefix(previousBlockId)
	if err != nil {
		return nil, errors.Wrap(err, "failed to calculate ref_block_prefix")
	}

	// Set expiration (default: 10 minutes from now)
	// Use UTC time to match steemjs behavior
	expiration := time.Now().UTC().Add(600 * time.Second)

	// Create transaction
	tx := transaction.NewSignedTransaction(&transaction.Transaction{
		RefBlockNum:    refBlockNum,
		RefBlockPrefix: refBlockPrefix,
		Expiration:     &protocol.Time{Time: &expiration},
		Extensions:     []interface{}{}, // Initialize empty extensions
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

// CustomJson creates and broadcasts a custom_json operation.
// This method matches the steem-js broadcast.customJson() interface.
//
// Parameters:
//   - requiredAuths: List of accounts that must provide active authority (can be empty)
//   - requiredPostingAuths: List of accounts that must provide posting authority (can be empty)
//   - id: Custom operation ID (e.g., "follow", "notify", etc.)
//   - json: JSON string containing the operation data
//   - privKeyWif: Private key in WIF format (posting or active key depending on required auths)
//
// Returns the broadcast result or an error.
func (b *Broadcast) CustomJson(requiredAuths, requiredPostingAuths []string, id, json, privKeyWif string) ([]byte, error) {
	// Sort required_auths and required_posting_auths to ensure consistent serialization
	// This matches steem-js behavior where flat_set fields are sorted
	sortedRequiredAuths := make([]string, len(requiredAuths))
	copy(sortedRequiredAuths, requiredAuths)
	sort.Strings(sortedRequiredAuths)

	sortedRequiredPostingAuths := make([]string, len(requiredPostingAuths))
	copy(sortedRequiredPostingAuths, requiredPostingAuths)
	sort.Strings(sortedRequiredPostingAuths)

	op := &protocol.CustomJSONOperation{
		RequiredAuths:        sortedRequiredAuths,
		RequiredPostingAuths: sortedRequiredPostingAuths,
		ID:                   id,
		JSON:                 json,
	}

	// Determine which key type to use based on required auths
	// If requiredAuths is not empty, we need active key
	// If only requiredPostingAuths is set, we need posting key
	keyType := "posting"
	if len(requiredAuths) > 0 {
		keyType = "active"
	}

	privKeys := map[string]string{
		keyType: privKeyWif,
	}

	return b.Send([]protocol.Operation{op}, privKeys)
}
