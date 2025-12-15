package core

// TxPool 用于暂存待打包的交易
type TxPool struct {
	pool map[string]*Transaction
}

// NewTxPool 创建空交易池
func NewTxPool() *TxPool {
	return &TxPool{
		pool: make(map[string]*Transaction),
	}
}

// Add 将交易放入池，重复 ID 会覆盖旧交易
func (p *TxPool) Add(id string, tx *Transaction) {
	p.pool[id] = tx
}

// Remove 移除已确认或无效的交易
func (p *TxPool) Remove(id string) {
	delete(p.pool, id)
}

// Pending 返回当前所有待打包交易
func (p *TxPool) Pending() []*Transaction {
	txs := make([]*Transaction, 0, len(p.pool))
	for _, tx := range p.pool {
		txs = append(txs, tx)
	}
	return txs
}

// Size 返回交易池大小
func (p *TxPool) Size() int {
	return len(p.pool)
}

