package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"io"

	"github.com/yiqi-017/blockchain/network"
)

// Dashboard 代理服务：聚合多节点状态，并提供转发接口（tx/mine）
// 启动示例：
//
//	go run ./cmd/dashboard -addr :3000 -peers http://127.0.0.1:8080,http://127.0.0.1:8081
func main() {
	addr := flag.String("addr", ":3000", "dashboard 监听地址")
	peersStr := flag.String("peers", "http://127.0.0.1:8080", "逗号分隔的 peer 列表")
	flag.Parse()

	peers := parsePeers(*peersStr)
	if len(peers) == 0 {
		log.Fatal("至少需要一个 peer")
	}

	srv := &Server{
		peers:  peers,
		client: &http.Client{Timeout: 5 * time.Second},
		addr:   *addr,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/status", srv.handleStatus)
	mux.HandleFunc("/api/block/latest", srv.handleLatestBlock)
	mux.HandleFunc("/api/txpool", srv.handleTxPool)
	mux.HandleFunc("/api/tx", srv.handleTx)
	mux.HandleFunc("/api/mine", srv.handleMine)

	log.Printf("dashboard listening on %s, peers=%v", *addr, peers)
	log.Fatal(http.ListenAndServe(*addr, withCORS(mux)))
}

type Server struct {
	peers  []string
	client *http.Client
	addr   string
}

// GET /api/status -> 聚合各节点状态
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var out []map[string]any
	for _, p := range s.peers {
		status, err := s.fetchStatus(p)
		if err != nil {
			out = append(out, map[string]any{"peer": p, "error": err.Error()})
		} else {
			out = append(out, map[string]any{"peer": p, "status": status})
		}
	}
	writeJSON(w, out)
}

// GET /api/block/latest?peer=...
func (s *Server) handleLatestBlock(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	peer := r.URL.Query().Get("peer")
	if peer == "" {
		http.Error(w, "peer required", http.StatusBadRequest)
		return
	}
	status, err := s.fetchStatus(peer)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	block, err := s.fetchBlock(peer, status.Height)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, map[string]any{
		"peer":  peer,
		"block": block,
	})
}

// GET /api/txpool?peer=...
func (s *Server) handleTxPool(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	peer := r.URL.Query().Get("peer")
	if peer == "" {
		http.Error(w, "peer required", http.StatusBadRequest)
		return
	}
	resp, err := s.client.Get(peer + "/txpool")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		http.Error(w, fmt.Sprintf("peer status %d", resp.StatusCode), http.StatusBadGateway)
		return
	}
	var payload network.TxPoolResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, map[string]any{"peer": peer, "txpool": payload})
}

// POST /api/tx {peer,to,value}
func (s *Server) handleTx(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Peer  string `json:"peer"`
		To    string `json:"to"`
		Value int64  `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if req.Peer == "" || req.To == "" || req.Value <= 0 {
		http.Error(w, "peer/to/value required", http.StatusBadRequest)
		return
	}
	resp, err := s.forwardJSON(req.Peer+"/tx", req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	copyStatus(w, resp)
}

// POST /api/mine {peer,miner,difficulty?,reward?}
func (s *Server) handleMine(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Peer       string  `json:"peer"`
		Miner      string  `json:"miner"`
		Difficulty *uint32 `json:"difficulty,omitempty"`
		Reward     *int64  `json:"reward,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if req.Peer == "" || req.Miner == "" {
		http.Error(w, "peer/miner required", http.StatusBadRequest)
		return
	}
	resp, err := s.forwardJSON(req.Peer+"/mine", req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	copyStatus(w, resp)
}

// helpers

func (s *Server) fetchStatus(peer string) (*network.StatusResponse, error) {
	resp, err := s.client.Get(peer + "/status")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("peer status %d", resp.StatusCode)
	}
	var status network.StatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, err
	}
	return &status, nil
}

func (s *Server) fetchBlock(peer string, height uint64) (*network.BlockResponse, error) {
	resp, err := s.client.Get(fmt.Sprintf("%s/block?height=%d", peer, height))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("peer status %d", resp.StatusCode)
	}
	var block network.BlockResponse
	if err := json.NewDecoder(resp.Body).Decode(&block); err != nil {
		return nil, err
	}
	return &block, nil
}

func (s *Server) forwardJSON(url string, body any) (*http.Response, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func copyStatus(w http.ResponseWriter, resp *http.Response) {
	// 避免复制下游的 CORS 头，使用本服务的 CORS 设置
	if ct := resp.Header.Get("Content-Type"); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	w.WriteHeader(resp.StatusCode)
	if resp.Body != nil {
		defer resp.Body.Close()
		_, _ = io.Copy(w, resp.Body)
	}
}

func parsePeers(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
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
