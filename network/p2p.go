package network

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

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

