package network

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/yiqi-017/blockchain/core"
	"github.com/yiqi-017/blockchain/storage"
)

// TestServerThreeNodesSync 启动三个节点（不同端口），验证区块和交易池最终一致
func TestServerThreeNodesSync(t *testing.T) {
	base := t.TempDir()

	// 建立三个节点存储
	storeA := mustStore(t, base, "nA")
	storeB := mustStore(t, base, "nB")
	storeC := mustStore(t, base, "nC")

	// 节点 A 写入创世块和一笔交易
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

	// 启动 3 个 HTTP server
	sA := startNodeServerForTest(t, "nA", storeA)
	sB := startNodeServerForTest(t, "nB", storeB)
	sC := startNodeServerForTest(t, "nC", storeC)

	peers := []string{sA.URL, sB.URL, sC.URL}

	// 为每个节点启动同步循环，持续短时间
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var wg sync.WaitGroup
	startSyncLoop := func(selfURL string, store *storage.FileStorage) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}
				for _, p := range peers {
					if p == selfURL {
						continue
					}
					syncer := NewSyncer(p)
					_ = syncer.SyncBlocks(store)
					_ = syncer.SyncTxPool(store)
				}
				time.Sleep(100 * time.Millisecond)
			}
		}()
	}
	// A 作为源节点，不主动同步；B/C 负责从 peers 拉取
	startSyncLoop(sB.URL, storeB)
	startSyncLoop(sC.URL, storeC)

	// 等待同步完成
	wg.Wait()

	// 校验区块高度和哈希一致
	expectHash := core.HashBlockHeader(&genesis.Header)
	for idx, s := range []struct {
		name  string
		store *storage.FileStorage
	}{
		{"A", storeA},
		{"B", storeB},
		{"C", storeC},
	} {
		heights, err := s.store.ListBlockHeights()
		if err != nil {
			t.Fatalf("node %s list heights: %v", s.name, err)
		}
		if len(heights) != 1 {
			t.Fatalf("node %s expect height count 1, got %d", s.name, len(heights))
		}
		b, err := s.store.LoadBlock(0)
		if err != nil {
			t.Fatalf("node %s load block: %v", s.name, err)
		}
		if string(core.HashBlockHeader(&b.Header)) != string(expectHash) {
			t.Fatalf("node %s block hash mismatch", s.name)
		}
		_ = idx
	}

	// 校验交易池一致
	for _, s := range []struct {
		name  string
		store *storage.FileStorage
	}{
		{"A", storeA},
		{"B", storeB},
		{"C", storeC},
	} {
		pool, err := s.store.LoadTxPool()
		if err != nil {
			t.Fatalf("node %s load pool: %v", s.name, err)
		}
		if pool.Size() != 1 {
			t.Fatalf("node %s expect pool size 1, got %d", s.name, pool.Size())
		}
	}
}

func mustStore(t *testing.T, base, node string) *storage.FileStorage {
	t.Helper()
	s, err := storage.NewFileStorage(base, node)
	if err != nil {
		t.Fatalf("new storage %s: %v", node, err)
	}
	return s
}

// startNodeServerForTest 启动基于 httptest 的节点服务
func startNodeServerForTest(t *testing.T, nodeID string, store *storage.FileStorage) *httptest.Server {
	t.Helper()
	ns := &NodeServer{
		NodeID: nodeID,
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
