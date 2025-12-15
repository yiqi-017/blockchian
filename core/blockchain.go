package core

// Blockchain 维护区块的链式结构（简化为内存切片）
type Blockchain struct {
	blocks []*Block
}

// NewBlockchain 创建仅包含创世区块的链
func NewBlockchain(genesis *Block) *Blockchain {
	return &Blockchain{
		blocks: []*Block{genesis},
	}
}

// Tip 返回最新区块
func (bc *Blockchain) Tip() *Block {
	if len(bc.blocks) == 0 {
		return nil
	}
	return bc.blocks[len(bc.blocks)-1]
}

// AppendBlock 将新区块附加到链尾
func (bc *Blockchain) AppendBlock(block *Block) {
	bc.blocks = append(bc.blocks, block)
}

// Blocks 返回当前的区块列表副本
func (bc *Blockchain) Blocks() []*Block {
	out := make([]*Block, len(bc.blocks))
	copy(out, bc.blocks)
	return out
}

