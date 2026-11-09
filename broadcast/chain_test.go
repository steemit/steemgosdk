package broadcast

import (
	"bytes"
	"testing"
	"time"

	"github.com/steemit/steemutil/protocol"
	"github.com/steemit/steemutil/transaction"
)

func TestBroadcastChainDefaultsToMainnet(t *testing.T) {
	b := NewBroadcast("http://127.0.0.1:1/")
	if b.chain == nil || b.chain.ID != transaction.SteemChain.ID {
		t.Fatalf("NewBroadcast default chain = %+v, want SteemChain", b.chain)
	}

	b2 := NewBroadcastWithChain("http://127.0.0.1:1/", nil)
	if b2.chain.ID != transaction.SteemChain.ID {
		t.Fatalf("NewBroadcastWithChain(nil) chain = %s, want SteemChain", b2.chain.ID)
	}

	b3 := NewBroadcastWithChain("http://127.0.0.1:1/", transaction.TestChain)
	if b3.chain.ID != transaction.TestChain.ID {
		t.Fatalf("NewBroadcastWithChain(TestChain) chain = %s, want TestChain", b3.chain.ID)
	}

	b3.SetChain(nil)
	if b3.chain.ID != transaction.SteemChain.ID {
		t.Fatalf("SetChain(nil) chain = %s, want SteemChain", b3.chain.ID)
	}
}

func TestDigestDiffersPerChain(t *testing.T) {
	expTime := time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC)
	exp := protocol.Time{Time: &expTime}
	tx := &transaction.SignedTransaction{
		Transaction: &transaction.Transaction{
			RefBlockNum:    1,
			RefBlockPrefix: 2,
			Expiration:     &exp,
			Operations: []protocol.Operation{
				&protocol.TransferOperation{
					From:   "initminer",
					To:     "tester01",
					Amount: "100.000 TESTS",
				},
			},
		},
	}

	digestMain, err := tx.Digest(transaction.SteemChain)
	if err != nil {
		t.Fatalf("digest mainnet: %v", err)
	}
	digestTest, err := tx.Digest(transaction.TestChain)
	if err != nil {
		t.Fatalf("digest testnet: %v", err)
	}
	if bytes.Equal(digestMain, digestTest) {
		t.Fatal("digest identical across mainnet/testnet chain IDs; chain ID not mixed into the digest")
	}
}
