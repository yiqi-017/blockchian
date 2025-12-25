package core

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
)

// TxInput 表示交易输入，引用前一交易的输出
type TxInput struct {
	TxID      []byte // 被引用的交易 ID
	Vout      int    // 被引用的输出索引
	Signature []byte // 交易签名
	PubKey    []byte // 发送者公钥（用于验签和地址匹配）
}

// TxOutput 表示交易输出
type TxOutput struct {
	Value        int64  // 转账金额（最小单位）
	ScriptPubKey string // 简化的锁定脚本
}

// Transaction 定义一笔交易，包含输入、输出和 coinbase 标记
type Transaction struct {
	ID         []byte     // 交易哈希（可延迟计算）
	Inputs     []TxInput  // 输入列表
	Outputs    []TxOutput // 输出列表
	IsCoinbase bool       // 是否为 coinbase 交易
}

// NewCoinbaseTx 创建一笔简单的 coinbase 交易
func NewCoinbaseTx(to string, reward int64) *Transaction {
	output := TxOutput{
		Value:        reward,
		ScriptPubKey: to,
	}
	return &Transaction{
		ID:         nil, // 可后续计算哈希
		Inputs:     nil, // coinbase 无输入
		Outputs:    []TxOutput{output},
		IsCoinbase: true,
	}
}

// TxSigningHash 返回用于签名/验签的交易摘要（不包含 Signature 字段）
func TxSigningHash(tx *Transaction) []byte {
	return txDigestWithSig(tx, false)
}

// ComputeTxID 返回包含签名在内的交易唯一哈希，用于引用输出
func ComputeTxID(tx *Transaction) []byte {
	return txDigestWithSig(tx, true)
}

// txDigestWithSig 根据 needSig 控制是否纳入签名字段，生成双 SHA-256 哈希
func txDigestWithSig(tx *Transaction, includeSig bool) []byte {
	if tx == nil {
		return nil
	}

	var buf bytes.Buffer
	if tx.IsCoinbase {
		buf.WriteByte(1)
	} else {
		buf.WriteByte(0)
	}

	for _, in := range tx.Inputs {
		writeVarBytes(&buf, in.TxID)
		writeInt64(&buf, int64(in.Vout))
		if includeSig {
			writeVarBytes(&buf, in.Signature)
		}
		writeVarBytes(&buf, in.PubKey)
	}
	for _, out := range tx.Outputs {
		writeInt64(&buf, out.Value)
		buf.WriteString(out.ScriptPubKey)
	}
	sum := sha256.Sum256(buf.Bytes())
	sum2 := sha256.Sum256(sum[:])
	return sum2[:]
}

func writeInt64(buf *bytes.Buffer, v int64) {
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], uint64(v))
	buf.Write(b[:])
}

func writeVarBytes(buf *bytes.Buffer, b []byte) {
	writeInt64(buf, int64(len(b)))
	buf.Write(b)
}
