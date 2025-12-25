package network

import (
	"encoding/hex"
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/yiqi-017/blockchain/core"
	"github.com/yiqi-017/blockchain/storage"
)

// NodeServer 提供最小 HTTP 接口用于同步区块和交易池
type NodeServer struct {
	NodeID string
	Store  *storage.FileStorage
	Addr   string // 监听地址，例 ":8080"
}

// Start 启动 HTTP 服务（阻塞）
func (s *NodeServer) Start() error {
	mux := http.NewServeMux()

	mux.HandleFunc("/status", s.handleStatus)
	mux.HandleFunc("/block", s.handleBlock)
	mux.HandleFunc("/txpool", s.handleTxPool)
	mux.HandleFunc("/tx", s.handleTx)
	mux.HandleFunc("/mine", s.handleMine)

	log.Printf("P2P HTTP server listening on %s", s.Addr)
	return http.ListenAndServe(s.Addr, withCORS(mux))
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
		if err := s.Store.SaveBlock(payload.Block); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
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

// handleTx 追加一笔简易交易到交易池
func (s *NodeServer) handleTx(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var payload struct {
		To    string `json:"to"`
		Value int64  `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if payload.To == "" || payload.Value <= 0 {
		http.Error(w, "invalid to/value", http.StatusBadRequest)
		return
	}

	pool, err := s.Store.LoadTxPool()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	tx := &core.Transaction{
		Outputs:    []core.TxOutput{{Value: payload.Value, ScriptPubKey: payload.To}},
		IsCoinbase: false,
	}
	txID := makeTxID()
	pool.Add(txID, tx)

	if err := s.Store.SaveTxPool(pool); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{
		"id":      txID,
		"poolLen": pool.Size(),
	})
}

// handleMine 挖出一块（包含当前交易池 + coinbase），并清空交易池
func (s *NodeServer) handleMine(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var payload struct {
		Miner      string  `json:"miner"`
		Difficulty *uint32 `json:"difficulty,omitempty"`
		Reward     *int64  `json:"reward,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if payload.Miner == "" {
		http.Error(w, "miner required", http.StatusBadRequest)
		return
	}

	tip, err := loadTip(s.Store)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	pool, err := s.Store.LoadTxPool()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	pending := pool.Pending()
	reward := int64(50)
	if payload.Reward != nil {
		reward = *payload.Reward
	}
	coinbase := core.NewCoinbaseTx(payload.Miner, reward)

	var baseTxs []*core.Transaction
	baseTxs = append(baseTxs, coinbase)
	baseTxs = append(baseTxs, pending...)

	diff := uint32(8)
	if payload.Difficulty != nil {
		diff = *payload.Difficulty
	} else if tip != nil {
		diff = tip.Header.Difficulty
	}

	block := core.MineBlock(tip, baseTxs, diff)

	if err := s.Store.SaveBlock(block); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := s.Store.SaveTxPool(core.NewTxPool()); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]any{
		"height": block.Header.Height,
		"hash":   hex.EncodeToString(core.HashBlockHeader(&block.Header)),
		"txs":    len(baseTxs),
		"diff":   diff,
	})
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

func makeTxID() string {
	return strings.Join([]string{
		strconv.FormatInt(time.Now().UnixNano(), 10),
		strconv.Itoa(rand.Int()),
	}, "-")
}

// loadTip 从存储中读取最高区块
func loadTip(store *storage.FileStorage) (*core.Block, error) {
	heights, err := store.ListBlockHeights()
	if err != nil {
		return nil, err
	}
	if len(heights) == 0 {
		return nil, nil
	}
	last := heights[len(heights)-1]
	return store.LoadBlock(last)
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
