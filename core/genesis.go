package core

import (
	"bytes"
	"encoding/base64"
	"log"
)

// 固定创世块参数，确保所有节点使用同一创世哈希。
const (
	genesisMiner      = "miner"
	genesisTimestamp  = int64(1766922950)
	genesisDifficulty = uint32(12)
	genesisNonce      = uint64(1966)
	genesisMerkleB64  = "lnzZx6skOjSgtVsnI1Pz9PxxUTOUNt7owZ2vb/jAOsQ="
)

// GenesisBlock 返回硬编码的创世块（哈希稳定，不再依赖 time.Now）
func GenesisBlock() *Block {
	tx := NewCoinbaseTx(genesisMiner, 50)
	txs := []*Transaction{tx}

	merkle := ComputeMerkleRoot(txs)
	expectedMerkle := mustDecodeB64(genesisMerkleB64)
	if !bytes.Equal(merkle, expectedMerkle) {
		// 若计算逻辑被改动，提前暴露问题，避免生成错误创世
		log.Panic("genesis merkle root mismatch")
	}

	return &Block{
		Header: BlockHeader{
			Version:    1,
			PrevHash:   nil,
			MerkleRoot: expectedMerkle,
			Timestamp:  genesisTimestamp,
			Difficulty: genesisDifficulty,
			Nonce:      genesisNonce,
			Height:     0,
		},
		Transactions: txs,
	}
}

func mustDecodeB64(s string) []byte {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		log.Panicf("decode base64: %v", err)
	}
	return b
}
