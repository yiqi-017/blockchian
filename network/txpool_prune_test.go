package network

import (
	"testing"

	"github.com/yiqi-017/blockchain/core"
	"github.com/yiqi-017/blockchain/storage"
)

// TestPruneTxPool 确认落盘后按交易 ID 移除池内交易
func TestPruneTxPool(t *testing.T) {
	base := t.TempDir()
	store := mustStore(t, base, "prune")

	// 初始交易池含一笔交易
	tx := &core.Transaction{
		Outputs:    []core.TxOutput{{Value: 5, ScriptPubKey: "alice"}},
		IsCoinbase: false,
	}
	tx.ID = core.ComputeTxID(tx)
	pool := core.NewTxPool()
	pool.Add("tx1", tx)
	if err := store.SaveTxPool(pool); err != nil {
		t.Fatalf("save pool: %v", err)
	}

	// 构造包含该交易的区块并落盘
	block := core.MineBlock(nil, []*core.Transaction{core.NewCoinbaseTx("miner", 50), tx}, 4)
	if err := pruneTxPoolAndSave(store, block); err != nil {
		t.Fatalf("save block: %v", err)
	}

	poolAfter, err := store.LoadTxPool()
	if err != nil {
		t.Fatalf("load pool after: %v", err)
	}
	if poolAfter.Size() != 0 {
		t.Fatalf("txpool should be empty after prune, got %d", poolAfter.Size())
	}
}

// pruneTxPoolAndSave 封装 validateAndPersist 的落盘+剪枝路径（简化测试）
func pruneTxPoolAndSave(store *storage.FileStorage, block *core.Block) error {
	if err := store.SaveBlock(block); err != nil {
		return err
	}
	pruneTxPool(store, block.Transactions)
	return nil
}
