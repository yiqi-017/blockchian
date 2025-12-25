package test

import (
	"testing"

	"github.com/yiqi-017/blockchain/core"
)

// TestPOWValidation 通过小难度挖块并校验 POW；篡改后应失败
func TestPOWValidation(t *testing.T) {
	txs := []*core.Transaction{core.NewCoinbaseTx("miner", 50)}
	block := core.MineBlock(nil, txs, 4) // 小难度，快速出块
	if !core.ValidateBlockPOW(block) {
		t.Fatalf("valid block should pass POW validation")
	}
	// 篡改 nonce 破坏 POW
	block.Header.Nonce++
	if core.ValidateBlockPOW(block) {
		t.Fatalf("tampered block should fail POW validation")
	}
}
