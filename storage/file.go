package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/yiqi-017/blockchain/core"
)

// FileStorage 以节点隔离的目录结构存储链数据和交易池
type FileStorage struct {
	rootDir   string
	blocksDir string
	poolDir   string
}

// NewFileStorage 创建存储实例，目录结构：baseDir/nodeID/{blocks,txpool}
func NewFileStorage(baseDir, nodeID string) (*FileStorage, error) {
	if nodeID == "" {
		return nil, errors.New("nodeID is required")
	}

	root := filepath.Join(baseDir, nodeID)
	blocks := filepath.Join(root, "blocks")
	pool := filepath.Join(root, "txpool")

	for _, dir := range []string{blocks, pool} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}

	return &FileStorage{
		rootDir:   root,
		blocksDir: blocks,
		poolDir:   pool,
	}, nil
}

// SaveBlock 将区块序列化为 JSON 按高度存储
func (s *FileStorage) SaveBlock(block *core.Block) error {
	if block == nil {
		return errors.New("block is nil")
	}

	filename := fmt.Sprintf("%d.json", block.Header.Height)
	path := filepath.Join(s.blocksDir, filename)

	data, err := json.MarshalIndent(block, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// LoadBlock 按高度读取区块
func (s *FileStorage) LoadBlock(height uint64) (*core.Block, error) {
	path := filepath.Join(s.blocksDir, fmt.Sprintf("%d.json", height))
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var block core.Block
	if err := json.Unmarshal(data, &block); err != nil {
		return nil, err
	}
	return &block, nil
}

// ListBlockHeights 返回已存储的区块高度（升序）
func (s *FileStorage) ListBlockHeights() ([]uint64, error) {
	entries, err := os.ReadDir(s.blocksDir)
	if err != nil {
		return nil, err
	}

	var heights []uint64
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		h, err := strconv.ParseUint(strings.TrimSuffix(name, ".json"), 10, 64)
		if err == nil {
			heights = append(heights, h)
		}
	}

	sort.Slice(heights, func(i, j int) bool { return heights[i] < heights[j] })
	return heights, nil
}

// ClearBlocks 删除所有区块文件（用于重组覆盖）
func (s *FileStorage) ClearBlocks() error {
	entries, err := os.ReadDir(s.blocksDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		if err := os.Remove(filepath.Join(s.blocksDir, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

type txPoolPersist struct {
	Entries map[string]*core.Transaction `json:"entries"`
}

// SaveTxPool 存储交易池，便于多节点模拟时隔离
func (s *FileStorage) SaveTxPool(pool *core.TxPool) error {
	if pool == nil {
		return errors.New("tx pool is nil")
	}

	data, err := json.MarshalIndent(txPoolPersist{Entries: pool.Snapshot()}, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(s.poolDir, "pool.json")
	return os.WriteFile(path, data, 0o644)
}

// LoadTxPool 读取交易池，若不存在则返回空池
func (s *FileStorage) LoadTxPool() (*core.TxPool, error) {
	path := filepath.Join(s.poolDir, "pool.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return core.NewTxPool(), nil
		}
		return nil, err
	}

	var persist txPoolPersist
	if err := json.Unmarshal(data, &persist); err != nil {
		return nil, err
	}

	pool := core.NewTxPool()
	if persist.Entries != nil {
		pool.LoadSnapshot(persist.Entries)
	}
	return pool, nil
}
