package network

import (
	"testing"
	"time"

	"github.com/yiqi-017/blockchain/core"
)

// TestRejectFutureTimestamp 检查过远未来时间戳的区块被拒
func TestRejectFutureTimestamp(t *testing.T) {
	base := t.TempDir()
	store := mustStore(t, base, "ts")

	block := core.MineBlock(nil, []*core.Transaction{core.NewCoinbaseTx("miner", 50)}, 0)
	block.Header.Timestamp = time.Now().Add(5 * time.Minute).Unix()
	if err := validateAndPersistBlock(store, block); err == nil {
		t.Fatalf("expected future timestamp block to be rejected")
	}
}

// TestGenesisMismatchReorg 对端 genesis 与本地不同，重组应失败
func TestGenesisMismatchReorg(t *testing.T) {
	base := t.TempDir()
	store := mustStore(t, base, "genesis")

	localGenesis := core.MineBlock(nil, []*core.Transaction{core.NewCoinbaseTx("minerA", 50)}, 0)
	if err := store.SaveBlock(localGenesis); err != nil {
		t.Fatalf("save local genesis: %v", err)
	}

	peerGenesis := core.MineBlock(nil, []*core.Transaction{core.NewCoinbaseTx("minerB", 50)}, 0)
	peerBlock1 := core.MineBlock(peerGenesis, []*core.Transaction{core.NewCoinbaseTx("minerB", 50)}, 0)
	peerBlocks := []*core.Block{peerGenesis, peerBlock1}

	s := NewSyncer("peer")
	s.fetchBlockFn = func(height uint64) (*core.Block, error) {
		return peerBlocks[height], nil
	}

	if err := s.reorgFromPeer(store, 1); err == nil {
		t.Fatalf("expected reorg to fail due to genesis mismatch")
	}
}

