package test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/yiqi-017/blockchain/core"
	"github.com/yiqi-017/blockchain/storage"
)

// TestStorageIsolationAndConsistency 验证不同节点的数据目录隔离，读写一致
func TestStorageIsolationAndConsistency(t *testing.T) {
	base := t.TempDir()

	// 准备两个节点的存储
	store1, err := storage.NewFileStorage(base, "nodeA")
	if err != nil {
		t.Fatalf("new storage A: %v", err)
	}
	store2, err := storage.NewFileStorage(base, "nodeB")
	if err != nil {
		t.Fatalf("new storage B: %v", err)
	}

	// 节点 A 写创世块与空池
	genesis := core.MineBlock(nil, []*core.Transaction{core.NewCoinbaseTx("minerA", 50)}, 4)
	if err := store1.SaveBlock(genesis); err != nil {
		t.Fatalf("save genesis A: %v", err)
	}
	if err := store1.SaveTxPool(core.NewTxPool()); err != nil {
		t.Fatalf("save txpool A: %v", err)
	}

	// 节点 B 写自己的创世块，与 A 不同矿工
	genesisB := core.MineBlock(nil, []*core.Transaction{core.NewCoinbaseTx("minerB", 50)}, 4)
	if err := store2.SaveBlock(genesisB); err != nil {
		t.Fatalf("save genesis B: %v", err)
	}

	// 验证隔离：目录内容不同，互不影响
	checkFileExists(t, filepath.Join(base, "nodeA", "blocks", "0.json"))
	checkFileExists(t, filepath.Join(base, "nodeB", "blocks", "0.json"))
	if _, err := os.Stat(filepath.Join(base, "nodeA", "blocks", "1.json")); err == nil {
		t.Fatalf("nodeA should not have block 1 yet")
	}
	if _, err := os.Stat(filepath.Join(base, "nodeB", "blocks", "1.json")); err == nil {
		t.Fatalf("nodeB should not have block 1 yet")
	}

	// 读回校验一致性
	readA, err := store1.LoadBlock(0)
	if err != nil {
		t.Fatalf("load A block0: %v", err)
	}
	if readA.Header.Height != 0 || readA.Header.PrevHash != nil {
		t.Fatalf("unexpected block A header")
	}
	readB, err := store2.LoadBlock(0)
	if err != nil {
		t.Fatalf("load B block0: %v", err)
	}
	if readB.Header.Height != 0 || readB.Header.PrevHash != nil {
		t.Fatalf("unexpected block B header")
	}
	if string(core.HashBlockHeader(&readA.Header)) == string(core.HashBlockHeader(&readB.Header)) {
		t.Fatalf("different miners should produce different genesis blocks")
	}

	// TxPool 隔离：A 写入一笔交易，不影响 B
	poolA := core.NewTxPool()
	tx := core.NewCoinbaseTx("someone", 1)
	poolA.Add("tx1", tx)
	if err := store1.SaveTxPool(poolA); err != nil {
		t.Fatalf("save txpool A: %v", err)
	}

	poolARead, err := store1.LoadTxPool()
	if err != nil {
		t.Fatalf("load txpool A: %v", err)
	}
	if poolARead.Size() != 1 {
		t.Fatalf("txpool A size expect 1, got %d", poolARead.Size())
	}
	poolBRead, err := store2.LoadTxPool()
	if err != nil {
		t.Fatalf("load txpool B: %v", err)
	}
	if poolBRead.Size() != 0 {
		t.Fatalf("txpool B should stay empty, got %d", poolBRead.Size())
	}
}

func checkFileExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file not found: %s, err=%v", path, err)
	}
}

