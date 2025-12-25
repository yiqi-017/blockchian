package network

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yiqi-017/blockchain/core"
	"github.com/yiqi-017/blockchain/storage"
)

// TestNetworkBlockAndTxSync 覆盖区块同步和交易池同步
func TestNetworkBlockAndTxSync(t *testing.T) {
	base := t.TempDir()
	storeA, err := storage.NewFileStorage(base, "nodeA")
	if err != nil {
		t.Fatalf("storeA: %v", err)
	}
	storeB, err := storage.NewFileStorage(base, "nodeB")
	if err != nil {
		t.Fatalf("storeB: %v", err)
	}

	// 准备节点 A：写入创世块与交易池
	genesis := core.MineBlock(nil, []*core.Transaction{core.NewCoinbaseTx("minerA", 50)}, 4)
	if err := storeA.SaveBlock(genesis); err != nil {
		t.Fatalf("save genesis A: %v", err)
	}
	poolA := core.NewTxPool()
	poolA.Add("tx1", &core.Transaction{
		Outputs:    []core.TxOutput{{Value: 7, ScriptPubKey: "alice"}},
		IsCoinbase: false,
	})
	if err := storeA.SaveTxPool(poolA); err != nil {
		t.Fatalf("save txpool A: %v", err)
	}

	srvA := startNodeServerSimple(t, storeA)

	// B 同步区块
	syncer := NewSyncer(srvA.URL)
	if err := syncer.SyncBlocks(storeB); err != nil {
		t.Fatalf("sync blocks: %v", err)
	}
	blocksB, err := storeB.ListBlockHeights()
	if err != nil {
		t.Fatalf("list heights B: %v", err)
	}
	if len(blocksB) != 1 {
		t.Fatalf("expect 1 block after sync, got %d", len(blocksB))
	}
	bA, _ := storeA.LoadBlock(0)
	bB, _ := storeB.LoadBlock(0)
	if string(core.HashBlockHeader(&bA.Header)) != string(core.HashBlockHeader(&bB.Header)) {
		t.Fatalf("block hash mismatch after sync")
	}

	// B 同步交易池
	if err := syncer.SyncTxPool(storeB); err != nil {
		t.Fatalf("sync txpool: %v", err)
	}
	poolB, err := storeB.LoadTxPool()
	if err != nil {
		t.Fatalf("load pool B: %v", err)
	}
	if poolB.Size() != 1 {
		t.Fatalf("expect txpool size 1 after sync, got %d", poolB.Size())
	}
}

// startNodeServerSimple 启动基于 httptest 的节点服务（无 peers，同步用）
func startNodeServerSimple(t *testing.T, store *storage.FileStorage) *httptest.Server {
	t.Helper()
	ns := &NodeServer{
		NodeID: "test",
		Store:  store,
		Addr:   "",
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/status", ns.handleStatus)
	mux.HandleFunc("/block", ns.handleBlock)
	mux.HandleFunc("/txpool", ns.handleTxPool)
	srv := httptest.NewServer(mux)
	t.Cleanup(func() { srv.Close() })
	return srv
}
