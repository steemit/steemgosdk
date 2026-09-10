package v2

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

func TestGetBlock_NullResultMapsToErrBlockNotFound(t *testing.T) {
	node := newFakeNode(t, func(hit int64, w http.ResponseWriter) {
		writeJSON(w, http.StatusOK, `{"id":1,"jsonrpc":"2.0","result":null}`)
	})
	a := fastAPI(t, node.srv.URL)

	_, err := a.GetBlock(context.Background(), 111000000)
	if !errors.Is(err, ErrBlockNotFound) {
		t.Fatalf("expected ErrBlockNotFound, got: %v", err)
	}
	if node.hitCount() != 1 {
		t.Errorf("null result must not retry, got %d attempts", node.hitCount())
	}
}

func TestGetBlock_Success(t *testing.T) {
	node := newFakeNode(t, func(hit int64, w http.ResponseWriter) {
		writeJSON(w, http.StatusOK, `{"id":1,"jsonrpc":"2.0","result":{"block_id":"0685b900","previous":"0685b8ff","timestamp":"2026-09-09T00:00:00","witness":"ety001"}}`)
	})
	a := fastAPI(t, node.srv.URL)

	block, err := a.GetBlock(context.Background(), 20000000)
	if err != nil {
		t.Fatalf("GetBlock: %v", err)
	}
	if block.Witness != "ety001" {
		t.Errorf("unexpected witness %q", block.Witness)
	}
}

func TestGetOpsInBlock_EmptyArrayIsNotNotFound(t *testing.T) {
	node := newFakeNode(t, func(hit int64, w http.ResponseWriter) {
		writeJSON(w, http.StatusOK, `{"id":1,"jsonrpc":"2.0","result":[]}`)
	})
	a := fastAPI(t, node.srv.URL)

	ops, err := a.GetOpsInBlock(context.Background(), 20000000, false)
	if err != nil {
		t.Fatalf("GetOpsInBlock: %v", err)
	}
	if ops == nil {
		t.Error("success must return a non-nil (possibly empty) slice")
	}
	if len(ops) != 0 {
		t.Errorf("expected empty ops, got %d", len(ops))
	}
	if errors.Is(err, ErrBlockNotFound) {
		t.Error("empty ops must not be reported as block-not-found")
	}
}

func TestGetOpsInBlock_OpsDecoded(t *testing.T) {
	node := newFakeNode(t, func(hit int64, w http.ResponseWriter) {
		writeJSON(w, http.StatusOK, `{"id":1,"jsonrpc":"2.0","result":[{"trx_id":"abc","block":20000000,"trx_in_block":0,"op_in_trx":0,"virtual_op":0,"timestamp":"2026-09-09T00:00:00","op":["vote",{"voter":"ety001","author":"alice","permlink":"hello","weight":10000}]}]}`)
	})
	a := fastAPI(t, node.srv.URL)

	ops, err := a.GetOpsInBlock(context.Background(), 20000000, false)
	if err != nil {
		t.Fatalf("GetOpsInBlock: %v", err)
	}
	if len(ops) != 1 {
		t.Fatalf("expected 1 op, got %d", len(ops))
	}
	if ops[0].BlockNumber != 20000000 {
		t.Errorf("block number = %d", ops[0].BlockNumber)
	}
	if ops[0].TransactionID != "abc" {
		t.Errorf("trx id = %q", ops[0].TransactionID)
	}
}

func TestGetDynamicGlobalProperties(t *testing.T) {
	node := newFakeNode(t, func(hit int64, w http.ResponseWriter) {
		writeJSON(w, http.StatusOK, `{"id":1,"jsonrpc":"2.0","result":{"head_block_number":109434545,"last_irreversible_block_num":109434530,"current_aslot":110040068}}`)
	})
	a := fastAPI(t, node.srv.URL)

	dgp, err := a.GetDynamicGlobalProperties(context.Background())
	if err != nil {
		t.Fatalf("GetDynamicGlobalProperties: %v", err)
	}
	if dgp.HeadBlockNumber != 109434545 {
		t.Errorf("head = %d", dgp.HeadBlockNumber)
	}
	if uint(dgp.LastIrreversibleBlockNum) != 109434530 {
		t.Errorf("LIB = %d", dgp.LastIrreversibleBlockNum)
	}
}

func TestCallWithResult_DecodesIntoTarget(t *testing.T) {
	node := newFakeNode(t, func(hit int64, w http.ResponseWriter) {
		writeJSON(w, http.StatusOK, `{"id":1,"jsonrpc":"2.0","result":["ety001","ety002"]}`)
	})
	a := fastAPI(t, node.srv.URL)

	var names []string
	if err := a.CallWithResult(context.Background(), "condenser_api", "lookup_accounts", []interface{}{"ety", 2}, &names); err != nil {
		t.Fatalf("CallWithResult: %v", err)
	}
	if len(names) != 2 || names[0] != "ety001" {
		t.Errorf("unexpected names %v", names)
	}
}

func TestSignedCall_SingleAttemptOnly(t *testing.T) {
	node := newFakeNode(t, func(hit int64, w http.ResponseWriter) {
		writeJSON(w, http.StatusInternalServerError, `{"id":1,"jsonrpc":"2.0","result":null}`)
	})
	a := fastAPI(t, node.srv.URL)

	_, err := a.SignedCall(context.Background(), "condenser_api.get_follow_list", []interface{}{"ety001"}, "ety001", testWIF)
	if err == nil {
		t.Fatal("expected error")
	}
	if node.hitCount() != 1 {
		t.Errorf("signed calls must not retry, got %d attempts", node.hitCount())
	}
}

func TestSignedCall_RejectsNonHTTPTransport(t *testing.T) {
	a := NewAPI("ws://example.com")
	if _, err := a.SignedCall(context.Background(), "m", []interface{}{"p"}, "acct", testWIF); err == nil {
		t.Fatal("expected transport validation error")
	}
}

func TestSignedCall_Success(t *testing.T) {
	node := newFakeNode(t, func(hit int64, w http.ResponseWriter) {
		writeJSON(w, http.StatusOK, `{"id":1,"jsonrpc":"2.0","result":{"ok":true}}`)
	})
	a := fastAPI(t, node.srv.URL)

	resp, err := a.SignedCall(context.Background(), "condenser_api.get_follow_list", []interface{}{"ety001"}, "ety001", testWIF)
	if err != nil {
		t.Fatalf("SignedCall: %v", err)
	}
	if resp.Result == nil {
		t.Error("expected non-nil result")
	}
}

const testWIF = "5JLw5dgQAx6rhZEgNN5C2ds1V47RweGshynFSWFbaMohsYsBvE8"
