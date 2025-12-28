package main

import (
	"path/filepath"
	"testing"

	"github.com/yiqi-017/blockchain/crypto"
	"github.com/yiqi-017/blockchain/storage"
)

// TestCLIFlagFlow 通过 Run(args) 覆盖 init -> tx -> mine 的 flag 流程
func TestCLIFlagFlow(t *testing.T) {
	base := t.TempDir()
	walletPath := filepath.Join(base, "cli1", "wallet.json")
	w, err := storage.LoadOrCreateWallet(walletPath)
	if err != nil {
		t.Fatalf("load wallet: %v", err)
	}
	minerAddr := crypto.PublicKeyHex(w.PublicKey)

	// init（创世固定，奖励不归本钱包）
	if err := Run([]string{
		"-mode", "init",
		"-node", "cli1",
		"-data", base,
		"-miner", minerAddr,
		"-difficulty", "4",
	}); err != nil {
		t.Fatalf("run init: %v", err)
	}

	// 先挖一个块获取余额
	if err := Run([]string{
		"-mode", "mine",
		"-node", "cli1",
		"-data", base,
		"-miner", minerAddr,
		"-difficulty", "4",
	}); err != nil {
		t.Fatalf("run first mine: %v", err)
	}

	// tx
	if err := Run([]string{
		"-mode", "tx",
		"-node", "cli1",
		"-data", base,
		"-to", "alice",
		"-value", "5",
		"-wallet", walletPath,
	}); err != nil {
		t.Fatalf("run tx: %v", err)
	}

	// mine 再挖一个块打包交易并清空池子
	if err := Run([]string{
		"-mode", "mine",
		"-node", "cli1",
		"-data", base,
		"-miner", minerAddr,
		"-difficulty", "4",
	}); err != nil {
		t.Fatalf("run mine: %v", err)
	}

	store, err := storage.NewFileStorage(base, "cli1")
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	heights, err := store.ListBlockHeights()
	if err != nil {
		t.Fatalf("list heights: %v", err)
	}
	if len(heights) != 3 {
		t.Fatalf("expect 3 blocks after two mines, got %d", len(heights))
	}
	pool, err := store.LoadTxPool()
	if err != nil {
		t.Fatalf("load txpool: %v", err)
	}
	if pool.Size() != 0 {
		t.Fatalf("txpool should be cleared after mining, got %d", pool.Size())
	}
}
