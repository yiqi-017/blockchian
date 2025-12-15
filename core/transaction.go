package core

// TxInput 表示交易输入，引用前一交易的输出
type TxInput struct {
	TxID      []byte // 被引用的交易 ID
	Vout      int    // 被引用的输出索引
	ScriptSig string // 简化的签名脚本
}

// TxOutput 表示交易输出
type TxOutput struct {
	Value        int64  // 转账金额（最小单位）
	ScriptPubKey string // 简化的锁定脚本
}

// Transaction 定义一笔交易，包含输入、输出和 coinbase 标记
type Transaction struct {
	ID        []byte      // 交易哈希（可延迟计算）
	Inputs    []TxInput   // 输入列表
	Outputs   []TxOutput  // 输出列表
	IsCoinbase bool       // 是否为 coinbase 交易
}

// NewCoinbaseTx 创建一笔简单的 coinbase 交易
func NewCoinbaseTx(to string, reward int64) *Transaction {
	output := TxOutput{
		Value:        reward,
		ScriptPubKey: to,
	}
	return &Transaction{
		ID:        nil, // 可后续计算哈希
		Inputs:    nil, // coinbase 无输入
		Outputs:   []TxOutput{output},
		IsCoinbase: true,
	}
}

