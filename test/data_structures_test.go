package test

import (
	"testing"

	"github.com/yiqi-017/blockchain/core"
)

// TestDataStructuresSanity 验证核心数据结构的基础行为
func TestDataStructuresSanity(t *testing.T) {
	// 1) 交易 + Merkle 根
	coinbase := core.NewCoinbaseTx("miner", 50)
	tx2 := &core.Transaction{
		Inputs:     nil,
		Outputs:    []core.TxOutput{{Value: 10, ScriptPubKey: "alice"}},
		IsCoinbase: false,
	}
	tx2.ID = core.ComputeTxID(tx2)
	txs := []*core.Transaction{coinbase, tx2}
	merkle := core.ComputeMerkleRoot(txs)
	if len(merkle) == 0 {
		t.Fatalf("merkle root should not be empty")
	}

	// 2) 区块头与链式结构
	block := core.NewBlock(nil, merkle, txs, 0)
	if block.Header.Height != 0 {
		t.Fatalf("genesis height expected 0, got %d", block.Header.Height)
	}
	if len(block.Transactions) != 2 {
		t.Fatalf("block txs length mismatch")
	}

	bc := core.NewBlockchain(block)
	if bc.Tip() != block {
		t.Fatalf("blockchain tip mismatch")
	}

	// 3) 交易池基本操作
	pool := core.NewTxPool()
	pool.Add("t1", tx2)
	if pool.Size() != 1 {
		t.Fatalf("txpool size expected 1, got %d", pool.Size())
	}
	pool.Clear()
	if pool.Size() != 0 {
		t.Fatalf("txpool clear failed, size=%d", pool.Size())
	}
}

