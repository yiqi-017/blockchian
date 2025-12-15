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

// Snapshot 返回交易池 map 的浅拷贝，便于序列化存储
func (p *TxPool) Snapshot() map[string]*Transaction {
	copyMap := make(map[string]*Transaction, len(p.pool))
	for id, tx := range p.pool {
		copyMap[id] = tx
	}
	return copyMap
}

// LoadSnapshot 用外部提供的 map 覆盖当前池
func (p *TxPool) LoadSnapshot(entries map[string]*Transaction) {
	p.pool = make(map[string]*Transaction, len(entries))
	for id, tx := range entries {
		p.pool[id] = tx
	}
}

// Size 返回交易池大小
func (p *TxPool) Size() int {
	return len(p.pool)
}
