package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/yiqi-017/blockchain/core"
	"github.com/yiqi-017/blockchain/crypto"
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
	if err := Run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

// Run 解析参数并执行节点指令，便于测试复用
func Run(args []string) error {
	fs := flag.NewFlagSet("node", flag.ContinueOnError)

	mode := fs.String("mode", "init", "init | tx | mine | serve")
	nodeID := fs.String("node", "node1", "节点标识，用于隔离数据目录")
	dataDir := fs.String("data", "./data", "数据目录")
	miner := fs.String("miner", "miner", "挖矿奖励接收者（coinbase 输出脚本）")
	to := fs.String("to", "", "交易接收者脚本（用于 mode=tx）")
	walletPath := fs.String("wallet", "", "钱包文件路径（mode=tx 使用，默认 data/<node>/wallet.json）")
	value := fs.Int64("value", 10, "交易金额（用于 mode=tx）")
	difficulty := fs.Uint("difficulty", 12, "POW 难度（前导零位数）")
	addr := fs.String("addr", ":8080", "HTTP 监听地址（mode=serve）")
	peersStr := fs.String("peers", "", "逗号分隔的 peer 列表（mode=serve）")
	syncInterval := fs.Duration("sync-interval", 5*time.Second, "与 peers 同步间隔（mode=serve）")

	if err := fs.Parse(args); err != nil {
		return err
	}

	rand.Seed(time.Now().UnixNano())

	store, err := storage.NewFileStorage(*dataDir, *nodeID)
	if err != nil {
		return fmt.Errorf("init storage failed: %w", err)
	}

	switch *mode {
	case "init":
		if err := initChain(store, *miner, uint32(*difficulty)); err != nil {
			return fmt.Errorf("init chain failed: %w", err)
		}
	case "tx":
		if *to == "" {
			return fmt.Errorf("mode=tx 需要指定 -to")
		}
		if *walletPath == "" {
			*walletPath = defaultWalletPath(*dataDir, *nodeID)
		}
		if err := submitTx(store, *walletPath, *to, *value); err != nil {
			return fmt.Errorf("submit tx failed: %w", err)
		}
	case "mine":
		if err := mineOnce(store, *miner, uint32(*difficulty)); err != nil {
			return fmt.Errorf("mine failed: %w", err)
		}
	case "serve":
		peers := parsePeers(*peersStr)
		if err := serveNode(*nodeID, store, *addr, peers, *syncInterval); err != nil {
			return fmt.Errorf("serve failed: %w", err)
		}
	default:
		return fmt.Errorf("unknown mode: %s", *mode)
	}
	return nil
}

// initChain 创建创世块并保存，若已存在区块则跳过
func initChain(store *storage.FileStorage, miner string, difficulty uint32) error {
	heights, err := store.ListBlockHeights()
	if err != nil {
		return err
	}
	// miner/difficulty 对固定创世块无效，保留参数为兼容 CLI
	_ = miner
	_ = difficulty
	if len(heights) > 0 {
		log.Printf("链已存在，最高高度=%d，跳过创世", heights[len(heights)-1])
		return nil
	}

	// 使用硬编码的创世块，避免不同时间 init 产生不同 genesis
	block := core.GenesisBlock()

	if err := store.SaveBlock(block); err != nil {
		return err
	}
	if err := store.SaveTxPool(core.NewTxPool()); err != nil {
		return err
	}

	log.Printf("创世块创建成功，高度=%d，哈希=%x", block.Header.Height, core.HashBlockHeader(&block.Header))
	return nil
}

// submitTx 创建一笔签名交易并写入交易池
func submitTx(store *storage.FileStorage, walletPath string, to string, value int64) error {
	tip, err := loadTip(store)
	if err != nil {
		return err
	}
	if tip == nil {
		return fmt.Errorf("链不存在，请先执行 -mode init")
	}

	wallet, err := storage.LoadOrCreateWallet(walletPath)
	if err != nil {
		return fmt.Errorf("load wallet failed: %w", err)
	}

	tx, err := buildSignedTx(store, wallet, to, value)
	if err != nil {
		return err
	}

	txID := core.ComputeTxID(tx)
	tx.ID = txID

	pool, err := store.LoadTxPool()
	if err != nil {
		return err
	}
	pool.Add(fmt.Sprintf("%x", txID), tx)

	if err := store.SaveTxPool(pool); err != nil {
		return err
	}

	log.Printf("交易已加入池：id=%x, to=%s, value=%d, 池大小=%d", txID, to, value, pool.Size())
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
		Peers:  peers,
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

func defaultWalletPath(baseDir, nodeID string) string {
	return fmt.Sprintf("%s/%s/wallet.json", strings.TrimRight(baseDir, "/"), nodeID)
}

// buildSignedTx 简单 UTXO 选择（全链扫描），签名并返回交易
func buildSignedTx(store *storage.FileStorage, wallet *crypto.Wallet, to string, value int64) (*core.Transaction, error) {
	if value <= 0 {
		return nil, fmt.Errorf("value must be positive")
	}
	blocks, err := loadAllBlocks(store)
	if err != nil {
		return nil, err
	}
	utxoSet := core.BuildUTXOSet(blocks)
	fromAddr := crypto.PublicKeyHex(wallet.PublicKey)

	// 收集属于 from 的 UTXO
	var selected []core.UTXO
	var total int64
	for _, list := range utxoSet {
		for _, u := range list {
			if u.Output.ScriptPubKey == fromAddr {
				selected = append(selected, u)
				total += u.Output.Value
				if total >= value {
					break
				}
			}
		}
		if total >= value {
			break
		}
	}
	if total < value {
		return nil, fmt.Errorf("余额不足，需 %d 实有 %d", value, total)
	}

	var inputs []core.TxInput
	for _, u := range selected {
		inputs = append(inputs, core.TxInput{
			TxID:   u.TxID,
			Vout:   u.Index,
			PubKey: wallet.PublicKey,
		})
	}
	outputs := []core.TxOutput{
		{Value: value, ScriptPubKey: to},
	}
	change := total - value
	if change > 0 {
		outputs = append(outputs, core.TxOutput{Value: change, ScriptPubKey: fromAddr})
	}

	tx := &core.Transaction{
		Inputs:     inputs,
		Outputs:    outputs,
		IsCoinbase: false,
	}
	signHash := core.TxSigningHash(tx)
	for i := range tx.Inputs {
		sig, err := wallet.Sign(signHash)
		if err != nil {
			return nil, err
		}
		tx.Inputs[i].Signature = sig
	}
	tx.ID = core.ComputeTxID(tx)
	return tx, nil
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
