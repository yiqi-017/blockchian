package network

import (
	"bytes"
	"encoding/json"
	"errors"
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
	// fetchBlockFn 注入便于测试；生产使用默认 HTTP 拉取
	fetchBlockFn func(height uint64) (*core.Block, error)
}

func NewSyncer(peer string) *Syncer {
	return &Syncer{
		Peer: peer,
		Client: &http.Client{
			Timeout: 5 * time.Second,
		},
		fetchBlockFn: nil,
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

	// 尝试增量同步；遇到冲突时进行重组
	for h := start; h <= status.Height; h++ {
		block, err := s.fetchBlockInternal(h)
		if err != nil {
			return fmt.Errorf("fetch block %d: %w", h, err)
		}
		if err := validateAndPersistBlock(store, block); err != nil {
			if errors.Is(err, errConflictBlock) {
				return s.reorgFromPeer(store, status.Height)
			}
			return fmt.Errorf("validate block %d: %w", h, err)
		}
	}
	return nil
}

// reorgFromPeer 拉取对端全链并替换（当对端更长时）
func (s *Syncer) reorgFromPeer(store *storage.FileStorage, peerTip uint64) error {
	// 仅当对端链更长时才重组
	localHeights, err := store.ListBlockHeights()
	if err != nil {
		return err
	}
	if uint64(len(localHeights)) >= peerTip+1 {
		return fmt.Errorf("peer conflict but not longer chain")
	}

	var genesisHash []byte
	if len(localHeights) > 0 {
		g, err := store.LoadBlock(0)
		if err != nil {
			return err
		}
		genesisHash = core.HashBlockHeader(&g.Header)
	}

	blocks := make([]*core.Block, 0, peerTip+1)
	for h := uint64(0); h <= peerTip; h++ {
		b, err := s.fetchBlockInternal(h)
		if err != nil {
			return fmt.Errorf("fetch block %d during reorg: %w", h, err)
		}
		blocks = append(blocks, b)
	}
	if err := validateChainWithGenesis(blocks, genesisHash); err != nil {
		return fmt.Errorf("peer chain invalid: %w", err)
	}
	if err := store.ClearBlocks(); err != nil {
		return fmt.Errorf("clear local blocks: %w", err)
	}
	for _, b := range blocks {
		if err := store.SaveBlock(b); err != nil {
			return fmt.Errorf("rewrite block %d: %w", b.Header.Height, err)
		}
	}
	return nil
}

// validateChainWithGenesis 校验链的连续性、Merkle 和 POW，并在提供时校验创世哈希
func validateChainWithGenesis(blocks []*core.Block, expectGenesis []byte) error {
	var prevHash []byte
	for i, b := range blocks {
		if b == nil {
			return fmt.Errorf("nil block at %d", i)
		}
		if b.Header.Height != uint64(i) {
			return fmt.Errorf("height mismatch at %d", i)
		}
		if i == 0 {
			if len(b.Header.PrevHash) != 0 {
				return fmt.Errorf("genesis prev hash not empty")
			}
			if len(expectGenesis) > 0 {
				if !bytes.Equal(core.HashBlockHeader(&b.Header), expectGenesis) {
					return fmt.Errorf("genesis hash mismatch")
				}
			}
		} else {
			if !bytes.Equal(prevHash, b.Header.PrevHash) {
				return fmt.Errorf("prev hash mismatch at %d", i)
			}
		}
		merkle := core.ComputeMerkleRoot(b.Transactions)
		if !bytes.Equal(merkle, b.Header.MerkleRoot) {
			return fmt.Errorf("merkle mismatch at %d", i)
		}
		if !core.ValidateBlockPOW(b) {
			return fmt.Errorf("pow invalid at %d", i)
		}
		prevHash = core.HashBlockHeader(&b.Header)
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
func (s *Syncer) fetchBlockInternal(height uint64) (*core.Block, error) {
	if s.fetchBlockFn != nil {
		return s.fetchBlockFn(height)
	}
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
