package test

import (
	"fmt"
	"testing"

	"github.com/yiqi-017/blockchain/core"
	"github.com/yiqi-017/blockchain/crypto"
	"github.com/yiqi-017/blockchain/storage"
)

// TestCommandFlow 使用存储目录模拟 init -> tx -> mine 流程
func TestCommandFlow(t *testing.T) {
	base := t.TempDir()
	store, err := storage.NewFileStorage(base, "nodeCmd")
	if err != nil {
		t.Fatalf("new storage: %v", err)
	}

	// 准备钱包并用其地址作为矿工，使其拥有创世奖励
	wallet, err := crypto.GenerateWallet()
	if err != nil {
		t.Fatalf("wallet: %v", err)
	}
	minerAddr := crypto.PublicKeyHex(wallet.PublicKey)

	// init 创世
	if err := initChainLocal(store, minerAddr, 4); err != nil {
		t.Fatalf("init chain: %v", err)
	}
	blocks := mustHeights(t, store)
	if len(blocks) != 1 {
		t.Fatalf("expect genesis only, got %d blocks", len(blocks))
	}

	// 提交一笔签名交易（花费创世奖励）
	to := "alice"
	tx, err := buildSignedTxLocal(store, wallet, to, 5)
	if err != nil {
		t.Fatalf("build signed tx: %v", err)
	}
	pool, err := store.LoadTxPool()
	if err != nil {
		t.Fatalf("load pool: %v", err)
	}
	pool.Add("tx1", tx)
	if err := store.SaveTxPool(pool); err != nil {
		t.Fatalf("save pool: %v", err)
	}

	// 挖块后高度+1，池被清空
	if err := mineOnceLocal(store, minerAddr, 4); err != nil {
		t.Fatalf("mine once: %v", err)
	}
	blocks = mustHeights(t, store)
	if len(blocks) != 2 {
		t.Fatalf("expect 2 blocks after mining, got %d", len(blocks))
	}
	poolAfter, err := store.LoadTxPool()
	if err != nil {
		t.Fatalf("load pool after mine: %v", err)
	}
	if poolAfter.Size() != 0 {
		t.Fatalf("tx pool should be cleared after mining, got %d", poolAfter.Size())
	}
}

// initChainLocal 创建创世块并保存
func initChainLocal(store *storage.FileStorage, miner string, difficulty uint32) error {
	heights, err := store.ListBlockHeights()
	if err != nil {
		return err
	}
	if len(heights) > 0 {
		return nil
	}
	genesisTx := core.NewCoinbaseTx(miner, 50)
	block := core.MineBlock(nil, []*core.Transaction{genesisTx}, difficulty)
	if err := store.SaveBlock(block); err != nil {
		return err
	}
	return store.SaveTxPool(core.NewTxPool())
}

// buildSignedTxLocal 全链扫描 UTXO，构造签名交易
func buildSignedTxLocal(store *storage.FileStorage, wallet *crypto.Wallet, to string, value int64) (*core.Transaction, error) {
	if value <= 0 {
		return nil, fmt.Errorf("value must be positive")
	}
	blocks, err := loadAllBlocksLocal(store)
	if err != nil {
		return nil, err
	}
	utxoSet := core.BuildUTXOSet(blocks)
	fromAddr := crypto.PublicKeyHex(wallet.PublicKey)

	var selected []core.UTXO
	var total int64
	for _, list := range utxoSet {
		for _, u := range list {
			if u.Output.ScriptPubKey == fromAddr {
				selected = append(selected, u)
				total += u.Output.Value
				if total >= value {
					break
				}
			}
		}
		if total >= value {
			break
		}
	}
	if total < value {
		return nil, fmt.Errorf("余额不足，需 %d 实有 %d", value, total)
	}

	var inputs []core.TxInput
	for _, u := range selected {
		inputs = append(inputs, core.TxInput{
			TxID:   u.TxID,
			Vout:   u.Index,
			PubKey: wallet.PublicKey,
		})
	}
	outputs := []core.TxOutput{
		{Value: value, ScriptPubKey: to},
	}
	change := total - value
	if change > 0 {
		outputs = append(outputs, core.TxOutput{Value: change, ScriptPubKey: fromAddr})
	}

	tx := &core.Transaction{
		Inputs:     inputs,
		Outputs:    outputs,
		IsCoinbase: false,
	}
	signHash := core.TxSigningHash(tx)
	for i := range tx.Inputs {
		sig, err := wallet.Sign(signHash)
		if err != nil {
			return nil, err
		}
		tx.Inputs[i].Signature = sig
	}
	tx.ID = core.ComputeTxID(tx)
	return tx, nil
}

// mineOnceLocal 模拟挖块：取 txpool 交易+coinbase，挖出后清空池
func mineOnceLocal(store *storage.FileStorage, miner string, difficulty uint32) error {
	heights, err := store.ListBlockHeights()
	if err != nil {
		return err
	}
	var tip *core.Block
	if len(heights) > 0 {
		tip, err = store.LoadBlock(heights[len(heights)-1])
		if err != nil {
			return err
		}
	}
	pool, err := store.LoadTxPool()
	if err != nil {
		return err
	}
	pending := pool.Pending()
	coinbase := core.NewCoinbaseTx(miner, 50)
	var baseTxs []*core.Transaction
	baseTxs = append(baseTxs, coinbase)
	baseTxs = append(baseTxs, pending...)

	block := core.MineBlock(tip, baseTxs, difficulty)
	if err := store.SaveBlock(block); err != nil {
		return err
	}
	return store.SaveTxPool(core.NewTxPool())
}

// loadAllBlocksLocal 按高度顺序加载所有区块
func loadAllBlocksLocal(store *storage.FileStorage) ([]*core.Block, error) {
	heights, err := store.ListBlockHeights()
	if err != nil {
		return nil, err
	}
	var blocks []*core.Block
	for _, h := range heights {
		b, err := store.LoadBlock(h)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, b)
	}
	return blocks, nil
}

// mustHeights 返回当前存储的高度列表
func mustHeights(t *testing.T, store *storage.FileStorage) []uint64 {
	t.Helper()
	h, err := store.ListBlockHeights()
	if err != nil {
		t.Fatalf("list heights: %v", err)
	}
	return h
}
