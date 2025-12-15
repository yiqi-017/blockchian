package core

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"math/big"
	"time"
)

// HashBlockHeader 对区块头做双 SHA-256
func HashBlockHeader(header *BlockHeader) []byte {
	if header == nil {
		return nil
	}
	buf := serializeHeader(header)
	first := sha256.Sum256(buf)
	second := sha256.Sum256(first[:])
	return second[:]
}

// ValidateBlockPOW 校验区块头哈希是否满足难度目标
func ValidateBlockPOW(block *Block) bool {
	if block == nil {
		return false
	}
	target := targetFromDifficulty(block.Header.Difficulty)
	hashInt := new(big.Int).SetBytes(HashBlockHeader(&block.Header))
	return hashInt.Cmp(target) <= 0
}

// MineBlock 依据给定难度寻找满足目标的 Nonce；用于单节点模拟
// 若 prev 为 nil，视作创世块
func MineBlock(prev *Block, txs []*Transaction, difficulty uint32) *Block {
	var prevHash []byte
	var height uint64
	if prev != nil {
		prevHash = HashBlockHeader(&prev.Header)
		height = prev.Header.Height + 1
	} else {
		prevHash = nil
		height = 0
	}

	merkle := ComputeMerkleRoot(txs)
	block := NewBlock(prevHash, merkle, txs, height)
	block.Header.Difficulty = difficulty

	target := targetFromDifficulty(difficulty)

	for nonce := uint64(0); ; nonce++ {
		block.Header.Nonce = nonce
		block.Header.Timestamp = time.Now().Unix()
		hashInt := new(big.Int).SetBytes(HashBlockHeader(&block.Header))
		if hashInt.Cmp(target) <= 0 {
			return block
		}
	}
}

// targetFromDifficulty 将“前导零比特数”难度转换为目标值
// difficulty 表示要求的前导零位数（0-255）
func targetFromDifficulty(difficulty uint32) *big.Int {
	const maxBits = 256
	if difficulty >= maxBits {
		// 256 位全零仅理论极限，返回 0 确保比较结果一致
		return big.NewInt(0)
	}

	shift := maxBits - difficulty
	// 目标 = 1 << shift
	target := new(big.Int).Lsh(big.NewInt(1), uint(shift))
	return target
}

// serializeHeader 将区块头编码为字节流（小端整数）
func serializeHeader(h *BlockHeader) []byte {
	buf := new(bytes.Buffer)
	writeUint32(buf, h.Version)
	writeBytes(buf, h.PrevHash)
	writeBytes(buf, h.MerkleRoot)
	writeUint64(buf, uint64(h.Timestamp))
	writeUint32(buf, h.Difficulty)
	writeUint64(buf, h.Nonce)
	writeUint64(buf, h.Height)
	return buf.Bytes()
}

func writeUint32(buf *bytes.Buffer, v uint32) {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], v)
	buf.Write(b[:])
}

func writeUint64(buf *bytes.Buffer, v uint64) {
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], v)
	buf.Write(b[:])
}

func writeBytes(buf *bytes.Buffer, b []byte) {
	writeUint32(buf, uint32(len(b)))
	buf.Write(b)
}
