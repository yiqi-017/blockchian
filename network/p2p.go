package network

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/yiqi-017/blockchain/core"
	"github.com/yiqi-017/blockchain/storage"
)

// NodeServer 提供最小 HTTP 接口用于同步区块和交易池
type NodeServer struct {
	NodeID string
	Store  *storage.FileStorage
	Addr   string // 监听地址，例 ":8080"
	Peers  []string
}

// Start 启动 HTTP 服务（阻塞）
func (s *NodeServer) Start() error {
	mux := http.NewServeMux()

	mux.HandleFunc("/status", s.handleStatus)
	mux.HandleFunc("/block", s.handleBlock)
	mux.HandleFunc("/txpool", s.handleTxPool)
	mux.HandleFunc("/tx", s.handleSubmitTx)
	mux.HandleFunc("/balance", s.handleBalance)

	log.Printf("P2P HTTP server listening on %s", s.Addr)
	return http.ListenAndServe(s.Addr, mux)
}

// handleStatus 返回节点高度
func (s *NodeServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	height, err := latestHeight(s.Store)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := StatusResponse{NodeID: s.NodeID, Height: height}
	writeJSON(w, resp)
}

// handleBlock 支持 GET/POST
// GET /block?height=H  返回指定高度的区块
// POST /block          写入区块
func (s *NodeServer) handleBlock(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		heightStr := r.URL.Query().Get("height")
		h, err := strconv.ParseUint(heightStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid height", http.StatusBadRequest)
			return
		}
		block, err := s.Store.LoadBlock(h)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, BlockResponse{Block: block})
	case http.MethodPost:
		var payload BlockResponse
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		if payload.Block == nil {
			http.Error(w, "block is nil", http.StatusBadRequest)
			return
		}
		if err := validateAndPersistBlock(s.Store, payload.Block); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleTxPool GET 返回交易池；POST 覆盖交易池
func (s *NodeServer) handleTxPool(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		pool, err := s.Store.LoadTxPool()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, TxPoolResponse{Entries: pool.Snapshot()})
	case http.MethodPost:
		var payload TxPoolResponse
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		pool := core.NewTxPool()
		if payload.Entries != nil {
			pool.LoadSnapshot(payload.Entries)
		}
		if err := s.Store.SaveTxPool(pool); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleBalance 返回某地址的余额（基于全链 UTXO 扫描）
func (s *NodeServer) handleBalance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	addr := r.URL.Query().Get("addr")
	if addr == "" {
		http.Error(w, "addr is required", http.StatusBadRequest)
		return
	}
	blocks, err := loadAllBlocks(s.Store)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	utxos := core.BuildUTXOSet(blocks)
	var balance int64
	for _, list := range utxos {
		for _, u := range list {
			if u.Output.ScriptPubKey == addr {
				balance += u.Output.Value
			}
		}
	}
	writeJSON(w, BalanceResponse{Address: addr, Balance: balance})
}

// handleSubmitTx 接收外部提交的简单交易并写入交易池
func (s *NodeServer) handleSubmitTx(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var tx core.Transaction
	if err := json.NewDecoder(r.Body).Decode(&tx); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}

	blocks, err := loadAllBlocks(s.Store)
	if err != nil {
		http.Error(w, fmt.Sprintf("load blocks: %v", err), http.StatusInternalServerError)
		return
	}
	utxos := core.BuildUTXOSet(blocks)
	if len(tx.ID) == 0 {
		tx.ID = core.ComputeTxID(&tx)
	}
	if err := core.ValidateTransaction(&tx, utxos); err != nil {
		http.Error(w, fmt.Sprintf("tx invalid: %v", err), http.StatusBadRequest)
		return
	}

	pool, err := s.Store.LoadTxPool()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	pool.Add(fmt.Sprintf("%x", tx.ID), &tx)
	if err := s.Store.SaveTxPool(pool); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)

	// 广播给 peers，防止风暴：若来自转发请求则不再转发
	if r.Header.Get("X-No-Relay") == "" {
		s.broadcastTx(&tx)
	}
}

// latestHeight 获取本地区块最高高度，若无区块返回 0
func latestHeight(store *storage.FileStorage) (uint64, error) {
	heights, err := store.ListBlockHeights()
	if err != nil {
		return 0, err
	}
	if len(heights) == 0 {
		return 0, nil
	}
	return heights[len(heights)-1], nil
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// validateAndPersistBlock 对从网络收到的区块进行基本校验并落盘
func validateAndPersistBlock(store *storage.FileStorage, block *core.Block) error {
	if block == nil {
		return fmt.Errorf("block is nil")
	}

	// 若已存在同高度的区块且哈希一致，视为重复接受
	existing, err := store.LoadBlock(block.Header.Height)
	if err == nil && existing != nil {
		if bytes.Equal(core.HashBlockHeader(&existing.Header), core.HashBlockHeader(&block.Header)) {
			return nil
		}
		return errConflictBlock
	}

	var prev *core.Block
	if block.Header.Height > 0 {
		prev, err = store.LoadBlock(block.Header.Height - 1)
		if err != nil {
			return fmt.Errorf("missing prev block %d: %w", block.Header.Height-1, err)
		}
		expectedPrevHash := core.HashBlockHeader(&prev.Header)
		if !bytes.Equal(expectedPrevHash, block.Header.PrevHash) {
			return fmt.Errorf("prev hash mismatch at height %d", block.Header.Height)
		}
		if block.Header.Timestamp <= prev.Header.Timestamp {
			return fmt.Errorf("block timestamp not increasing")
		}
	} else {
		// 创世块要求 prev 为空
		if len(block.Header.PrevHash) != 0 {
			return fmt.Errorf("genesis prev hash must be empty")
		}
	}

	now := time.Now().Unix()
	const maxFutureDrift = int64(120) // 2 分钟容忍
	if block.Header.Timestamp > now+maxFutureDrift {
		return fmt.Errorf("block timestamp too far in future")
	}

	merkle := core.ComputeMerkleRoot(block.Transactions)
	if !bytes.Equal(merkle, block.Header.MerkleRoot) {
		return fmt.Errorf("invalid merkle root")
	}
	if !core.ValidateBlockPOW(block) {
		return fmt.Errorf("pow invalid")
	}

	// 校验交易（签名、余额）
	existingBlocks, err := loadAllBlocks(store)
	if err != nil {
		return err
	}
	utxos := core.BuildUTXOSet(existingBlocks)
	for _, tx := range block.Transactions {
		if err := core.ValidateTransaction(tx, utxos); err != nil {
			return fmt.Errorf("tx invalid: %w", err)
		}
		// 应用花费到 utxo 集以避免同块内双花
		utxos = applyTxToUTXO(tx, utxos)
	}

	if err := store.SaveBlock(block); err != nil {
		return err
	}
	// 收到新区块后移除已上链交易
	pruneTxPool(store, block.Transactions)
	return nil
}

// loadAllBlocks 按高度顺序加载全链
func loadAllBlocks(store *storage.FileStorage) ([]*core.Block, error) {
	heights, err := store.ListBlockHeights()
	if err != nil {
		return nil, err
	}
	var blocks []*core.Block
	for _, h := range heights {
		b, err := store.LoadBlock(h)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, b)
	}
	return blocks, nil
}

// applyTxToUTXO 将交易对 utxo 集的消费和新增应用（用于同块内依赖）
func applyTxToUTXO(tx *core.Transaction, utxos map[string][]core.UTXO) map[string][]core.UTXO {
	if tx == nil {
		return utxos
	}
	if !tx.IsCoinbase {
		for _, in := range tx.Inputs {
			key := fmt.Sprintf("%x", in.TxID)
			list := utxos[key]
			for i, u := range list {
				if bytes.Equal(u.TxID, in.TxID) && u.Index == in.Vout {
					// remove
					list = append(list[:i], list[i+1:]...)
					break
				}
			}
			if len(list) == 0 {
				delete(utxos, key)
			} else {
				utxos[key] = list
			}
		}
	}
	txID := core.ComputeTxID(tx)
	key := fmt.Sprintf("%x", txID)
	for idx, out := range tx.Outputs {
		utxos[key] = append(utxos[key], core.UTXO{
			TxID:   txID,
			Index:  idx,
			Output: out,
		})
	}
	return utxos
}

// pruneTxPool 移除交易池中已被区块包含的交易
func pruneTxPool(store *storage.FileStorage, txs []*core.Transaction) {
	if len(txs) == 0 {
		return
	}
	pool, err := store.LoadTxPool()
	if err != nil {
		return
	}
	target := make(map[string]struct{})
	for _, tx := range txs {
		if tx == nil {
			continue
		}
		idHex := fmt.Sprintf("%x", core.ComputeTxID(tx))
		target[idHex] = struct{}{}
	}
	// 根据交易池当前条目匹配目标 ID，按键删除
	var toRemove []string
	for id, tx := range pool.Snapshot() {
		idHex := fmt.Sprintf("%x", core.ComputeTxID(tx))
		if _, ok := target[idHex]; ok {
			toRemove = append(toRemove, id)
			continue
		}
		if _, ok := target[id]; ok { // 键本身就是 ID
			toRemove = append(toRemove, id)
		}
	}
	pool.RemoveMany(toRemove)
	_ = store.SaveTxPool(pool)
}

// broadcastTx 将交易推送到 peers 的 /tx 接口
func (s *NodeServer) broadcastTx(tx *core.Transaction) {
	if tx == nil || len(s.Peers) == 0 {
		return
	}
	body, err := json.Marshal(tx)
	if err != nil {
		return
	}
	for _, peer := range s.Peers {
		req, err := http.NewRequest(http.MethodPost, peer+"/tx", bytes.NewReader(body))
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-No-Relay", "1")
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			resp.Body.Close()
		}
	}
}
