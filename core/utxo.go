package core

import (
	"bytes"
	"errors"

	"github.com/yiqi-017/blockchain/crypto"
)

// UTXO 表示未花费输出
type UTXO struct {
	TxID   []byte
	Index  int
	Output TxOutput
}

// BuildUTXOSet 从区块列表构建 UTXO 集合（全链扫描）
// 返回 map[txidHex][]UTXO
func BuildUTXOSet(blocks []*Block) map[string][]UTXO {
	utxos := make(map[string][]UTXO)
	spent := make(map[string]map[int]bool) // txid -> vout -> spent

	for _, block := range blocks {
		for txIdx, tx := range block.Transactions {
			txID := ComputeTxID(tx)
			txIDHex := crypto.HexEncode(txID)

			// 处理支出
			if !tx.IsCoinbase {
				for _, in := range tx.Inputs {
					inIDHex := crypto.HexEncode(in.TxID)
					if spent[inIDHex] == nil {
						spent[inIDHex] = make(map[int]bool)
					}
					spent[inIDHex][in.Vout] = true
				}
			}

			// 收集未花费输出
			for outIdx, out := range tx.Outputs {
				if spent[txIDHex] != nil && spent[txIDHex][outIdx] {
					continue
				}
				utxos[txIDHex] = append(utxos[txIDHex], UTXO{
					TxID:   txID,
					Index:  outIdx,
					Output: out,
				})
			}

			// 避免后续交易（同区块）花费本交易输出时遗漏，标记当前 tx 的花费表存在
			if spent[txIDHex] == nil {
				spent[txIDHex] = make(map[int]bool)
			}

			// 确保交易哈希写回，便于后续引用
			block.Transactions[txIdx].ID = txID
		}
	}

	return utxos
}

// ValidateTransaction 校验单笔交易：存在性、余额守恒、验签、未花费
func ValidateTransaction(tx *Transaction, utxos map[string][]UTXO) error {
	if tx == nil {
		return errors.New("tx is nil")
	}
	if tx.IsCoinbase {
		return nil
	}

	signHash := TxSigningHash(tx)
	var inputSum int64
	for _, in := range tx.Inputs {
		utxo, ok := findUTXO(in.TxID, in.Vout, utxos)
		if !ok {
			return errors.New("referenced output not found or spent")
		}
		// 校验公钥匹配锁定脚本
		if utxo.Output.ScriptPubKey != crypto.PublicKeyHex(in.PubKey) {
			return errors.New("pubkey does not match script")
		}
		// 验签
		if !crypto.Verify(in.PubKey, signHash, in.Signature) {
			return errors.New("signature invalid")
		}
		inputSum += utxo.Output.Value
	}

	var outputSum int64
	for _, out := range tx.Outputs {
		if out.Value < 0 {
			return errors.New("negative output")
		}
		outputSum += out.Value
	}

	if inputSum < outputSum {
		return errors.New("inputs not enough")
	}
	return nil
}

func findUTXO(txid []byte, index int, utxos map[string][]UTXO) (UTXO, bool) {
	list, ok := utxos[crypto.HexEncode(txid)]
	if !ok {
		return UTXO{}, false
	}
	for _, u := range list {
		if bytes.Equal(u.TxID, txid) && u.Index == index {
			return u, true
		}
	}
	return UTXO{}, false
}
