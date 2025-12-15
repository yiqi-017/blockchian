package core

import "time"

// BlockHeader 定义区块头的核心字段，便于共识和校验
type BlockHeader struct {
	Version    uint32 // 协议版本
	PrevHash   []byte // 前一个区块哈希
	MerkleRoot []byte // 交易列表的 Merkle 根
	Timestamp  int64  // 时间戳（秒）
	Difficulty uint32 // 难度目标（简化）
	Nonce      uint64 // POW 随机数
	Height     uint64 // 区块高度（创世块为 0）
}

// Block 表示完整区块，包含区块头和交易列表
type Block struct {
	Header       BlockHeader     // 区块头
	Transactions []*Transaction  // 交易列表，包含一笔 coinbase
}

// NewBlock 简单构造器，生成带时间戳的区块结构
func NewBlock(prevHash []byte, merkleRoot []byte, txs []*Transaction, height uint64) *Block {
	header := BlockHeader{
		Version:    1,
		PrevHash:   prevHash,
		MerkleRoot: merkleRoot,
		Timestamp:  time.Now().Unix(),
		Difficulty: 0,
		Nonce:      0,
		Height:     height,
	}

	return &Block{
		Header:       header,
		Transactions: txs,
	}
}

