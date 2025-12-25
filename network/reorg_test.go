package network

import (
	"testing"

	"github.com/yiqi-017/blockchain/core"
)

// TestReorgFromPeer 本地短链，对端更长链，触发重组
func TestReorgFromPeer(t *testing.T) {
	base := t.TempDir()
	store := mustStore(t, base, "local")

	// 本地链：仅创世
	genesis := core.MineBlock(nil, []*core.Transaction{core.NewCoinbaseTx("minerA", 50)}, 4)
	if err := store.SaveBlock(genesis); err != nil {
		t.Fatalf("save local genesis: %v", err)
	}

	// 构造对端更长链：创世 + 区块1
	peerBlocks := []*core.Block{genesis}
	block1 := core.MineBlock(genesis, []*core.Transaction{core.NewCoinbaseTx("minerB", 50)}, 4)
	peerBlocks = append(peerBlocks, block1)

	// fake syncer，直接调用 reorgFromPeer
	s := NewSyncer("peer")
	fetches := 0
	s.fetchBlockFn = func(height uint64) (*core.Block, error) {
		fetches++
		return peerBlocks[height], nil
	}
	// 调用重组
	if err := s.reorgFromPeer(store, uint64(len(peerBlocks)-1)); err != nil {
		t.Fatalf("reorg failed: %v", err)
	}
	if fetches != len(peerBlocks) {
		t.Fatalf("expected fetch %d blocks, got %d", len(peerBlocks), fetches)
	}

	heights, err := store.ListBlockHeights()
	if err != nil {
		t.Fatalf("list heights: %v", err)
	}
	if len(heights) != 2 {
		t.Fatalf("expect 2 blocks after reorg, got %d", len(heights))
	}
	b1, _ := store.LoadBlock(1)
	if string(core.HashBlockHeader(&b1.Header)) != string(core.HashBlockHeader(&block1.Header)) {
		t.Fatalf("block1 hash mismatch after reorg")
	}
}

