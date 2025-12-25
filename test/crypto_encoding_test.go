package test

import (
	"encoding/hex"
	"testing"

	"github.com/yiqi-017/blockchain/core"
	"github.com/yiqi-017/blockchain/crypto"
)

// TestCryptoAndEncoding 覆盖 HASH、Merkle 根、公私钥签名验签
func TestCryptoAndEncoding(t *testing.T) {
	// HASH 与 Double HASH
	data := []byte("abc")
	wantHash, _ := hex.DecodeString("ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad")
	gotHash := crypto.Hash256(data)
	if hex.EncodeToString(gotHash) != hex.EncodeToString(wantHash) {
		t.Fatalf("Hash256 mismatch")
	}
	wantDouble, _ := hex.DecodeString("4f8b42c22dd3729b519ba6f68d2da7cc5b2d606d05daed5ad5128cc03e6c6358")
	gotDouble := crypto.DoubleHash256(data)
	if hex.EncodeToString(gotDouble) != hex.EncodeToString(wantDouble) {
		t.Fatalf("DoubleHash256 mismatch")
	}

	// Merkle 根：稳定且对输入变化敏感
	tx1 := &core.Transaction{ID: []byte("tx1"), IsCoinbase: true}
	tx2 := &core.Transaction{ID: []byte("tx2"), IsCoinbase: false}
	root1 := core.ComputeMerkleRoot([]*core.Transaction{tx1, tx2})
	if len(root1) == 0 {
		t.Fatalf("merkle root should not be empty")
	}
	// 修改交易应导致根变化
	tx2b := &core.Transaction{ID: []byte("tx2-mod"), IsCoinbase: false}
	root2 := core.ComputeMerkleRoot([]*core.Transaction{tx1, tx2b})
	if hex.EncodeToString(root1) == hex.EncodeToString(root2) {
		t.Fatalf("merkle root should change when tx changes")
	}
	// 相同输入应产生相同根
	root1Repeat := core.ComputeMerkleRoot([]*core.Transaction{tx1, tx2})
	if hex.EncodeToString(root1) != hex.EncodeToString(root1Repeat) {
		t.Fatalf("merkle root not deterministic")
	}

	// 公私钥签名/验签
	w, err := crypto.GenerateWallet()
	if err != nil {
		t.Fatalf("generate wallet: %v", err)
	}
	msg := []byte("sign me")
	sig, err := w.Sign(msg)
	if err != nil {
		t.Fatalf("sign failed: %v", err)
	}
	if !crypto.Verify(w.PublicKey, msg, sig) {
		t.Fatalf("verify failed for valid signature")
	}
	// 篡改消息应验签失败
	if crypto.Verify(w.PublicKey, []byte("other"), sig) {
		t.Fatalf("verify should fail for modified message")
	}
}
