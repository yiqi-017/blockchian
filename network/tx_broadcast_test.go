package network

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yiqi-017/blockchain/core"
)

// TestTxBroadcast 提交到节点 A 后会推送到节点 B 的交易池
func TestTxBroadcast(t *testing.T) {
	base := t.TempDir()
	storeA := mustStore(t, base, "nA")
	storeB := mustStore(t, base, "nB")

	// 启动节点 B
	srvB := startNodeServerSimple(t, storeB)

	// 节点 A 配置 peers 包含 B
	nsA := &NodeServer{
		NodeID: "A",
		Store:  storeA,
		Peers:  []string{srvB.URL},
	}
	muxA := http.NewServeMux()
	muxA.HandleFunc("/tx", nsA.handleSubmitTx)
	srvA := httptest.NewServer(muxA)
	t.Cleanup(func() { srvA.Close() })

	// 构造交易并 POST 到 A
	tx := core.NewCoinbaseTx("alice", 5) // coinbase 免输入校验，便于测试
	body, _ := json.Marshal(tx)
	resp, err := http.Post(srvA.URL+"/tx", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("post tx to A: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("unexpected status %d", resp.StatusCode)
	}

	// 交易应出现在 B 的池中
	poolB, err := storeB.LoadTxPool()
	if err != nil {
		t.Fatalf("load pool B: %v", err)
	}
	if poolB.Size() != 1 {
		t.Fatalf("expected pool size 1 at B, got %d", poolB.Size())
	}
}

