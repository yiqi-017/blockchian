package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	"github.com/yiqi-017/blockchain/core"
	"github.com/yiqi-017/blockchain/network"
	"github.com/yiqi-017/blockchain/storage"
)

// 简易 CLI：支持初始化链、提交交易、单次挖矿、启动 HTTP 服务并同步
// 示例：
//
//	go run ./cmd/node -mode init -node node1
//	go run ./cmd/node -mode tx   -node node1 -to alice -value 12
//	go run ./cmd/node -mode mine -node node1 -miner bob -difficulty 12
//	go run ./cmd/node -mode serve -node node1 -addr :8080 -peers http://127.0.0.1:8081,http://127.0.0.1:8082
func main() {
	mode := flag.String("mode", "init", "init | tx | mine | serve")
	nodeID := flag.String("node", "node1", "节点标识，用于隔离数据目录")
	dataDir := flag.String("data", "./data", "数据目录")
	miner := flag.String("miner", "miner", "挖矿奖励接收者（coinbase 输出脚本）")
	to := flag.String("to", "", "交易接收者脚本（用于 mode=tx）")
	value := flag.Int64("value", 10, "交易金额（用于 mode=tx）")
	difficulty := flag.Uint("difficulty", 12, "POW 难度（前导零位数）")
	addr := flag.String("addr", ":8080", "HTTP 监听地址（mode=serve）")
	peersStr := flag.String("peers", "", "逗号分隔的 peer 列表（mode=serve）")
	syncInterval := flag.Duration("sync-interval", 5*time.Second, "与 peers 同步间隔（mode=serve）")
	flag.Parse()

	rand.Seed(time.Now().UnixNano())

	store, err := storage.NewFileStorage(*dataDir, *nodeID)
	if err != nil {
		log.Fatalf("init storage failed: %v", err)
	}

	switch *mode {
	case "init":
		if err := initChain(store, *miner, uint32(*difficulty)); err != nil {
			log.Fatalf("init chain failed: %v", err)
		}
	case "tx":
		if *to == "" {
			log.Fatal("mode=tx 需要指定 -to")
		}
		if err := submitTx(store, *to, *value); err != nil {
			log.Fatalf("submit tx failed: %v", err)
		}
	case "mine":
		if err := mineOnce(store, *miner, uint32(*difficulty)); err != nil {
			log.Fatalf("mine failed: %v", err)
		}
	case "serve":
		peers := parsePeers(*peersStr)
		if err := serveNode(*nodeID, store, *addr, peers, *syncInterval); err != nil {
			log.Fatalf("serve failed: %v", err)
		}
	default:
		log.Fatalf("unknown mode: %s", *mode)
	}
}

// initChain 创建创世块并保存，若已存在区块则跳过
func initChain(store *storage.FileStorage, miner string, difficulty uint32) error {
	heights, err := store.ListBlockHeights()
	if err != nil {
		return err
	}
	if len(heights) > 0 {
		log.Printf("链已存在，最高高度=%d，跳过创世", heights[len(heights)-1])
		return nil
	}

	genesisTx := core.NewCoinbaseTx(miner, 50)
	block := core.MineBlock(nil, []*core.Transaction{genesisTx}, difficulty)

	if err := store.SaveBlock(block); err != nil {
		return err
	}
	if err := store.SaveTxPool(core.NewTxPool()); err != nil {
		return err
	}

	log.Printf("创世块创建成功，高度=%d，哈希=%x", block.Header.Height, core.HashBlockHeader(&block.Header))
	return nil
}

// submitTx 创建一笔简单交易并写入交易池
func submitTx(store *storage.FileStorage, to string, value int64) error {
	tip, err := loadTip(store)
	if err != nil {
		return err
	}
	if tip == nil {
		return fmt.Errorf("链不存在，请先执行 -mode init")
	}

	pool, err := store.LoadTxPool()
	if err != nil {
		return err
	}

	tx := &core.Transaction{
		ID:         nil,
		Inputs:     nil,
		Outputs:    []core.TxOutput{{Value: value, ScriptPubKey: to}},
		IsCoinbase: false,
	}
	txID := fmt.Sprintf("%d-%d", time.Now().UnixNano(), rand.Int())
	pool.Add(txID, tx)

	if err := store.SaveTxPool(pool); err != nil {
		return err
	}

	log.Printf("交易已加入池：id=%s, to=%s, value=%d, 池大小=%d", txID, to, value, pool.Size())
	return nil
}

// mineOnce 取出交易池交易 + coinbase，挖一个区块并持久化
func mineOnce(store *storage.FileStorage, miner string, difficulty uint32) error {
	tip, err := loadTip(store)
	if err != nil {
		return err
	}

	pool, err := store.LoadTxPool()
	if err != nil {
		return err
	}

	pending := pool.Pending()
	coinbase := core.NewCoinbaseTx(miner, 50)

	var baseTxs []*core.Transaction
	baseTxs = append(baseTxs, coinbase)
	baseTxs = append(baseTxs, pending...)

	block := core.MineBlock(tip, baseTxs, difficulty)

	if err := store.SaveBlock(block); err != nil {
		return err
	}
	// 挖出后清空交易池
	if err := store.SaveTxPool(core.NewTxPool()); err != nil {
		return err
	}

	log.Printf("出块成功：高度=%d，哈希=%x，包含交易=%d（含 coinbase）", block.Header.Height, core.HashBlockHeader(&block.Header), len(baseTxs))
	return nil
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

// serveNode 启动 HTTP 服务并定期从 peers 同步区块和交易池
func serveNode(nodeID string, store *storage.FileStorage, addr string, peers []string, interval time.Duration) error {
	server := &network.NodeServer{
		NodeID: nodeID,
		Store:  store,
		Addr:   addr,
	}

	// 后台同步循环
	go func() {
		for {
			for _, peer := range peers {
				syncer := network.NewSyncer(peer)
				if err := syncer.SyncBlocks(store); err != nil {
					log.Printf("[sync][%s] sync blocks err: %v", peer, err)
				}
				if err := syncer.SyncTxPool(store); err != nil {
					log.Printf("[sync][%s] sync txpool err: %v", peer, err)
				}
			}
			time.Sleep(interval)
		}
	}()

	return server.Start()
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
