package broadcast

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"github.com/steemit/steemutil/protocol"
	"github.com/steemit/steemutil/transaction"
	"github.com/steemit/steemutil/wif"
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

// Same transaction, digests computed through each Broadcast instance's
// chain: the mainnet digest must match the well-known steem vector
// (regression: existing mainnet callers keep the exact same signatures),
// and the TestChain digest must differ because the chain ID is part of the
// digest input.
func TestDigestDiffersPerChain(t *testing.T) {
	const wantMainnetDigest = "582176b1daf89984bc8b4fdcb24ff1433d1eb114a8c4bf20fb22ad580d035889"

	mainnetB := NewBroadcast("http://127.0.0.1:1/")
	testnetB := NewBroadcastWithChain("http://127.0.0.1:1/", transaction.TestChain)

	// Fixed transaction identical to steemutil's test vectors, so the
	// mainnet digest is a known constant.
	expiration := time.Date(2016, 8, 8, 12, 24, 17, 0, time.UTC)
	tx := &transaction.SignedTransaction{
		Transaction: &transaction.Transaction{
			RefBlockNum:    36029,
			RefBlockPrefix: 1164960351,
			Expiration:     &protocol.Time{Time: &expiration},
		},
	}
	tx.PushOperation(&protocol.VoteOperation{
		Voter:    "xeroc",
		Author:   "xeroc",
		Permlink: "piston",
		Weight:   10000,
	})

	digestMain, err := tx.Digest(mainnetB.chain)
	if err != nil {
		t.Fatalf("digest mainnet: %v", err)
	}
	if got := hex.EncodeToString(digestMain); got != wantMainnetDigest {
		t.Fatalf("mainnet digest = %s, want %s", got, wantMainnetDigest)
	}

	digestTest, err := tx.Digest(testnetB.chain)
	if err != nil {
		t.Fatalf("digest testnet: %v", err)
	}
	if bytes.Equal(digestMain, digestTest) {
		t.Fatal("digest identical across mainnet/testnet chain IDs; chain ID not mixed into the digest")
	}

	// Ground the difference: digest must equal sha256(chainID || serializedTx).
	rawChainID, err := hex.DecodeString(transaction.TestChain.ID)
	if err != nil {
		t.Fatalf("decode test chain ID: %v", err)
	}
	serialized, err := tx.Serialize()
	if err != nil {
		t.Fatalf("serialize transaction: %v", err)
	}
	want := sha256.Sum256(append(append([]byte{}, rawChainID...), serialized...))
	if !bytes.Equal(digestTest, want[:]) {
		t.Fatalf("testnet digest = %x, want sha256(chainID||tx) = %x", digestTest, want)
	}
}

// A signature made under one chain ID must not verify under another — the
// exact failure mode that motivated making the chain injectable.
func TestSignatureIsChainBound(t *testing.T) {
	const wifStr = "5JLw5dgQAx6rhZEgNN5C2ds1V47RweGshynFSWFbaMohsYsBvE8"

	privKey := &wif.PrivateKey{}
	if err := privKey.FromWif(wifStr); err != nil {
		t.Fatalf("decode WIF: %v", err)
	}
	pubKey := &wif.PublicKey{}
	if err := pubKey.FromWif(wifStr); err != nil {
		t.Fatalf("derive public key: %v", err)
	}
	privKeys := []*wif.PrivateKey{privKey}
	pubKeys := []*wif.PublicKey{pubKey}

	newTx := func() *transaction.SignedTransaction {
		expiration := time.Date(2016, 8, 8, 12, 24, 17, 0, time.UTC)
		tx := &transaction.SignedTransaction{
			Transaction: &transaction.Transaction{
				RefBlockNum:    36029,
				RefBlockPrefix: 1164960351,
				Expiration:     &protocol.Time{Time: &expiration},
			},
		}
		tx.PushOperation(&protocol.VoteOperation{
			Voter:    "xeroc",
			Author:   "xeroc",
			Permlink: "piston",
			Weight:   10000,
		})
		return tx
	}

	mainnetB := NewBroadcast("http://127.0.0.1:1/")
	testnetB := NewBroadcastWithChain("http://127.0.0.1:1/", transaction.TestChain)

	mainTx := newTx()
	if err := mainTx.Sign(privKeys, mainnetB.chain); err != nil {
		t.Fatalf("sign via mainnet instance: %v", err)
	}
	if ok, err := mainTx.Verify(pubKeys, transaction.SteemChain); err != nil || !ok {
		t.Fatalf("mainnet signature must verify on mainnet: ok=%v err=%v", ok, err)
	}
	if ok, _ := mainTx.Verify(pubKeys, transaction.TestChain); ok {
		t.Fatal("mainnet signature must NOT verify under the testnet chain ID")
	}

	testTx := newTx()
	if err := testTx.Sign(privKeys, testnetB.chain); err != nil {
		t.Fatalf("sign via testnet instance: %v", err)
	}
	if ok, err := testTx.Verify(pubKeys, transaction.TestChain); err != nil || !ok {
		t.Fatalf("testnet signature must verify on testnet: ok=%v err=%v", ok, err)
	}
	if ok, _ := testTx.Verify(pubKeys, transaction.SteemChain); ok {
		t.Fatal("testnet signature must NOT verify under the mainnet chain ID")
	}

	if mainTx.Transaction.Signatures[0] == testTx.Transaction.Signatures[0] {
		t.Fatal("signatures over the same transaction under different chain IDs must differ")
	}
}
