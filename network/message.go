package network

import "github.com/yiqi-017/blockchain/core"

// StatusResponse 返回节点基础状态
type StatusResponse struct {
	NodeID string `json:"node_id"`
	Height uint64 `json:"height"`
}

// BlockResponse 用于传输单个区块
type BlockResponse struct {
	Block *core.Block `json:"block"`
}

// TxPoolResponse 用于传输交易池
type TxPoolResponse struct {
	Entries map[string]*core.Transaction `json:"entries"`
}

