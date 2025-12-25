package network

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/yiqi-017/blockchain/core"
	"github.com/yiqi-017/blockchain/storage"
)

// Syncer 用于从单个 peer 拉取区块和交易池
type Syncer struct {
	Peer   string // 例如 http://127.0.0.1:8081
	Client *http.Client
}

func NewSyncer(peer string) *Syncer {
	return &Syncer{
		Peer: peer,
		Client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// SyncBlocks 拉取缺失区块并落盘
func (s *Syncer) SyncBlocks(store *storage.FileStorage) error {
	status, err := s.fetchStatus()
	if err != nil {
		return err
	}

	localHeights, err := store.ListBlockHeights()
	if err != nil {
		return err
	}
	// 若本地为空，则从 0 开始拉取（含创世）
	var start uint64
	if len(localHeights) == 0 {
		start = 0
	} else {
		start = localHeights[len(localHeights)-1] + 1
	}

	if status.Height+1 <= start {
		return nil // 无需同步
	}

	for h := start; h <= status.Height; h++ {
		block, err := s.fetchBlock(h)
		if err != nil {
			return fmt.Errorf("fetch block %d: %w", h, err)
		}
		if err := validateAndPersistBlock(store, block); err != nil {
			return fmt.Errorf("validate block %d: %w", h, err)
		}
	}
	return nil
}

// SyncTxPool 直接覆盖本地交易池
func (s *Syncer) SyncTxPool(store *storage.FileStorage) error {
	resp, err := s.get("/txpool")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var payload TxPoolResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return err
	}

	pool := core.NewTxPool()
	if payload.Entries != nil {
		pool.LoadSnapshot(payload.Entries)
	}
	return store.SaveTxPool(pool)
}

// fetchStatus 获取对端高度
func (s *Syncer) fetchStatus() (*StatusResponse, error) {
	resp, err := s.get("/status")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var status StatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, err
	}
	return &status, nil
}

// fetchBlock 获取指定高度区块
func (s *Syncer) fetchBlock(height uint64) (*core.Block, error) {
	resp, err := s.get(fmt.Sprintf("/block?height=%d", height))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var payload BlockResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	if payload.Block == nil {
		return nil, fmt.Errorf("empty block payload")
	}
	return payload.Block, nil
}

func (s *Syncer) get(path string) (*http.Response, error) {
	url := s.Peer + path
	resp, err := s.Client.Get(url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return nil, fmt.Errorf("GET %s status %d", url, resp.StatusCode)
	}
	return resp, nil
}
