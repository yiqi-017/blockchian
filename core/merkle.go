package core

import "crypto/sha256"

// ComputeMerkleRoot 依据交易列表计算 Merkle 根；若为空返回 nil
func ComputeMerkleRoot(txs []*Transaction) []byte {
	if len(txs) == 0 {
		return nil
	}

	// 初始化叶子节点哈希
	hashes := make([][]byte, len(txs))
	for i, tx := range txs {
		hashes[i] = txDigest(tx)
	}

	// 自底向上两两哈希，单数时复制最后一个
	for len(hashes) > 1 {
		nextLevel := make([][]byte, 0, (len(hashes)+1)/2)
		for i := 0; i < len(hashes); i += 2 {
			left := hashes[i]
			var right []byte
			if i+1 < len(hashes) {
				right = hashes[i+1]
			} else {
				right = left // 复制尾节点
			}
			nextLevel = append(nextLevel, hashPair(left, right))
		}
		hashes = nextLevel
	}

	return hashes[0]
}

// txDigest 提供交易的基础哈希；若已有 ID 则直接使用
func txDigest(tx *Transaction) []byte {
	if tx == nil {
		return nil
	}
	if len(tx.ID) > 0 {
		return tx.ID
	}

	return ComputeTxID(tx)
}

// hashPair 将左右子哈希拼接后再做一次 SHA-256
func hashPair(left, right []byte) []byte {
	combined := append(left, right...)
	sum := sha256.Sum256(combined)
	return sum[:]
}
