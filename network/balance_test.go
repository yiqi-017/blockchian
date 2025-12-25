package network

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yiqi-017/blockchain/core"
)

func TestBalanceHandler(t *testing.T) {
	base := t.TempDir()
	store := mustStore(t, base, "bal")

	// 创世：addr1 获得 50
	addr1 := "addr1"
	genesis := core.MineBlock(nil, []*core.Transaction{core.NewCoinbaseTx(addr1, 50)}, 0)
	if err := store.SaveBlock(genesis); err != nil {
		t.Fatalf("save genesis: %v", err)
	}
	// 第二块：addr1 -> addr2 30，找零 20 给 addr1（为简单起见跳过签名校验，直接写盘）
	tx := &core.Transaction{
		Inputs: []core.TxInput{{TxID: core.ComputeTxID(genesis.Transactions[0]), Vout: 0}},
		Outputs: []core.TxOutput{
			{Value: 30, ScriptPubKey: "addr2"},
			{Value: 20, ScriptPubKey: addr1},
		},
		IsCoinbase: false,
	}
	block1 := core.MineBlock(genesis, []*core.Transaction{tx}, 0)
	if err := store.SaveBlock(block1); err != nil {
		t.Fatalf("save block1: %v", err)
	}

	ns := &NodeServer{NodeID: "bal", Store: store}
	mux := http.NewServeMux()
	mux.HandleFunc("/balance", ns.handleBalance)
	srv := httptest.NewServer(mux)
	t.Cleanup(func() { srv.Close() })

	resp, err := http.Get(srv.URL + "/balance?addr=" + addr1)
	if err != nil {
		t.Fatalf("get balance: %v", err)
	}
	defer resp.Body.Close()
	var out BalanceResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Balance != 20 {
		t.Fatalf("expect balance 20, got %d", out.Balance)
	}
}
