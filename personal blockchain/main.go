package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

// 定义区块结构
type Block struct {
	Index        int
	Timestamp    string
	PrevHash     string
	Hash         string
	Transactions []Transaction
	Coinbase     Transaction // 添加 coinbase 交易
	Nonce        int
}

// 全局区块链和交易池
var Blockchain []Block

// 定义 PoW 难度
const Difficulty = 5

// 节点配置
type NodeConfig struct {
	Address string
	Peers   []string
}

// 当前节点配置
var CurrentNode NodeConfig

// 计算区块的哈希
func calculateHash(block Block) string {
	data, _ := json.Marshal(block)
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

// 创建创世区块
func createGenesisBlock() Block {
	genesisBlock := Block{
		Index:        0,
		Timestamp:    time.Now().String(),
		PrevHash:     "0",
		Transactions: []Transaction{},
		Nonce:        0,
	}
	genesisBlock.Hash = calculateHash(genesisBlock)
	return genesisBlock
}

// 创建新的区块
func generateBlock(prevBlock Block, minerAddress string) Block {
	transactions := clearTransactionPool()

	// 创建 coinbase 交易
	coinbaseTx := Transaction{
		Sender:    "coinbase",
		Recipient: minerAddress,
		Amount:    50, // 假设每个区块的奖励是 50
	}

	newBlock := Block{
		Index:        prevBlock.Index + 1,
		Timestamp:    time.Now().String(),
		PrevHash:     prevBlock.Hash,
		Transactions: transactions,
		Coinbase:     coinbaseTx,
		Nonce:        0,
	}

	newBlock.Nonce, newBlock.Hash = proofOfWork(newBlock)
	return newBlock
}

// 工作量证明函数
func proofOfWork(block Block) (int, string) {
	var nonce int
	var hash string
	for {
		block.Nonce = nonce
		hash = calculateHash(block)
		if isValidHash(hash) {
			break
		}
		nonce++
	}
	return nonce, hash
}

// 验证哈希是否符合 PoW 的难度要求
func isValidHash(hash string) bool {
	prefix := strings.Repeat("0", Difficulty)
	return hash[:Difficulty] == prefix
}

// 处理网络连接
func handleConnection(conn net.Conn) {
	defer conn.Close()

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		message := scanner.Text()
		var data map[string]interface{}
		if err := json.Unmarshal([]byte(message), &data); err != nil {
			fmt.Println("Error decoding message:", err)
			continue
		}

		switch data["type"] {
		case "block":
			handleBlockSync(data["data"])
		case "transaction":
			handleTransactionSync(data["data"])
		}
	}
}

// 验证区块的有效性
func isValidBlock(newBlock, prevBlock Block) bool {
	// 验证索引
	if newBlock.Index != prevBlock.Index+1 {
		fmt.Println("Invalid index")
		return false
	}

	// 验证前一个区块的哈希
	if newBlock.PrevHash != prevBlock.Hash {
		fmt.Println("Invalid previous hash")
		return false
	}

	// 验证区块哈希
	expectedHash := calculateHash(newBlock)
	if newBlock.Hash != expectedHash {
		fmt.Println("Invalid hash")
		return false
	}

	// 验证 PoW 难度
	if !isValidHash(newBlock.Hash) {
		fmt.Println("Invalid proof of work")
		return false
	}

	return true
}

// 处理区块同步
func handleBlockSync(data interface{}) {
	blocks, err := parseBlocks(data)
	if err != nil {
		fmt.Println("Error parsing blocks:", err)
		return
	}
	if len(blocks) > len(Blockchain) {
		Blockchain = blocks
		fmt.Println("Blockchain updated!")
	}
}

// 处理交易同步
func handleTransactionSync(data interface{}) {
	transactions, err := parseTransactions(data)
	if err != nil {
		fmt.Println("Error parsing transactions:", err)
		return
	}
	TransactionPool = append(TransactionPool, transactions...)
	fmt.Println("Transaction pool updated!")
}

// 启动服务器
func startServer(address string) {
	http.HandleFunc("/blocks", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			var blocks []Block
			err := json.NewDecoder(r.Body).Decode(&blocks)
			if err != nil {
				fmt.Println("Error parsing blocks:", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			// 验证区块链有效性并更新区块链
			if len(blocks) > len(Blockchain) {
				Blockchain = blocks
				fmt.Println("Blockchain updated!")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("Blockchain updated"))
			} else {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("Invalid blockchain"))
			}
		}
	})

	http.HandleFunc("/transactions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			var transactions []Transaction
			err := json.NewDecoder(r.Body).Decode(&transactions)
			if err != nil {
				fmt.Println("Error parsing transactions:", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			// 将接收到的交易添加到交易池
			TransactionPool = append(TransactionPool, transactions...)
			fmt.Println("Transactions received:", transactions)

			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Transactions added to pool"))
		}
	})

	fmt.Println("Starting server at", address)
	if err := http.ListenAndServe(address, nil); err != nil {
		fmt.Println("Error starting server:", err)
	}
}

// 广播消息到网络
func broadcastMessage(message string) {
	for _, peer := range CurrentNode.Peers {
		conn, err := net.Dial("tcp", peer)
		if err != nil {
			fmt.Println("Error connecting to peer:", peer, err)
			continue
		}
		defer conn.Close()

		fmt.Fprintln(conn, message)
	}
}

// 同步区块
func syncBlock(block Block) {
	data, err := json.Marshal(block)
	if err != nil {
		fmt.Println("Error marshalling block:", err)
		return
	}

	for _, peer := range CurrentNode.Peers {
		resp, err := http.Post("http://"+peer+"/blocks", "application/json", bytes.NewBuffer(data))
		if err != nil {
			fmt.Println("Error sending block to", peer, ":", err)
			continue
		}
		resp.Body.Close()
		fmt.Println("Block sent to", peer)
	}
}

func syncBlockchain() {
	data, err := json.Marshal(Blockchain)
	if err != nil {
		fmt.Println("Error marshalling blockchain:", err)
		return
	}

	for _, peer := range CurrentNode.Peers {
		resp, err := http.Post("http://"+peer+"/blocks", "application/json", bytes.NewBuffer(data))
		if err != nil {
			fmt.Println("Error sending blockchain to", peer, ":", err)
			continue
		}
		resp.Body.Close()
		fmt.Println("Blockchain sent to", peer)
	}

	// 保存区块链到文件
	err = saveBlockchainToFile(strings.ReplaceAll(CurrentNode.Address, ":", "_") + "_blockchain.json")
	if err != nil {
		fmt.Println("Error saving blockchain to file:", err)
	}
}

// 同步交易
// 广播交易到其他节点
func syncTransaction(transaction Transaction) {
	transactionList := []Transaction{transaction} // 将单个交易包装成数组
	data, err := json.Marshal(transactionList)
	if err != nil {
		fmt.Println("Error marshalling transactions:", err)
		return
	}

	for _, peer := range CurrentNode.Peers {
		resp, err := http.Post("http://"+peer+"/transactions", "application/json", bytes.NewBuffer(data))
		if err != nil {
			fmt.Println("Error sending transaction to", peer, ":", err)
			continue
		}
		resp.Body.Close()
		fmt.Println("Transaction sent to", peer)
	}

	// 保存区块链到文件
	err = saveBlockchainToFile(strings.ReplaceAll(CurrentNode.Address, ":", "_") + "_blockchain.json")
	if err != nil {
		fmt.Println("Error saving blockchain to file:", err)
	}
}

// 打印区块链
func printBlockchain() {
	for i, block := range Blockchain {
		fmt.Printf("\n--- Block %d ---\n", i)
		fmt.Printf("Index: %d\n", block.Index)
		fmt.Printf("Timestamp: %s\n", block.Timestamp)
		fmt.Printf("PrevHash: %s\n", block.PrevHash)
		fmt.Printf("Hash: %s\n", block.Hash)
		fmt.Printf("Nonce: %d\n", block.Nonce)
		fmt.Println("Coinbase Transaction:")
		fmt.Printf("  - %s -> %s: %d\n", block.Coinbase.Sender, block.Coinbase.Recipient, block.Coinbase.Amount)
		fmt.Println("Transactions:")
		for _, tx := range block.Transactions {
			fmt.Printf("  - %s -> %s: %d\n", tx.Sender, tx.Recipient, tx.Amount)
		}
	}
}

func saveBlockchainToFile(filename string) error {
	data, err := json.Marshal(Blockchain)
	if err != nil {
		return err
	}
	return os.WriteFile(filename, data, 0644)
}

// 从文件加载区块链
func loadBlockchainFromFile(filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			// 文件不存在，初始化创世区块
			Blockchain = append(Blockchain, createGenesisBlock())
			return saveBlockchainToFile(filename)
		}
		return err
	}
	return json.Unmarshal(data, &Blockchain)
}

// 程序主入口
func main() {
	// 通过命令行参数指定节点地址和邻居
	address := flag.String("address", "localhost:8080", "Current node address")
	peers := flag.String("peers", "", "Comma-separated list of peer addresses")
	flag.Parse()

	// 处理 peers 参数，确保没有空字符串
	peerList := strings.Split(*peers, ",")
	var validPeers []string
	for _, peer := range peerList {
		if peer != "" {
			validPeers = append(validPeers, peer)
		}
	}

	CurrentNode = NodeConfig{
		Address: *address,
		Peers:   validPeers,
	}

	// 从文件加载区块链
	err := loadBlockchainFromFile(strings.ReplaceAll(*address, ":", "_") + "_blockchain.json")
	if err != nil {
		fmt.Println("Error loading blockchain from file:", err)
		return
	}

	// 启动服务器
	go startServer(CurrentNode.Address)

	// 启动用户交互
	fmt.Println("Node running at", *address)
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Println("\nChoose an option:")
		fmt.Println("1. Add Transaction")
		fmt.Println("2. Generate Block")
		fmt.Println("3. Print Blockchain")
		fmt.Println("4. Mine Block") // 添加模拟挖矿选项
		fmt.Print("> ")

		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		switch input {
		case "1":
			fmt.Print("Enter recipient name: ")
			recipient, _ := reader.ReadString('\n')
			recipient = strings.TrimSpace(recipient)

			fmt.Print("Enter amount: ")
			var amount int
			fmt.Scanln(&amount)

			alice := createUser() // 假设当前用户是 Alice
			addTransaction(alice, recipient, amount)
			syncTransaction(TransactionPool[len(TransactionPool)-1]) // 同步交易
			fmt.Println("Transaction added!")
		case "2":
			if len(TransactionPool) == 0 {
				fmt.Println("No transactions to include in the block!")
			} else {
				newBlock := generateBlock(Blockchain[len(Blockchain)-1], CurrentNode.Address)
				Blockchain = append(Blockchain, newBlock)
				syncBlockchain() // 同步整个区块链
				fmt.Println("New block generated and blockchain synchronized!")
			}
		case "3":
			printBlockchain()
		case "4":
			// 模拟挖矿
			newBlock := generateBlock(Blockchain[len(Blockchain)-1], CurrentNode.Address)
			Blockchain = append(Blockchain, newBlock)
			syncBlockchain() // 同步整个区块链
			fmt.Println("Block mined and blockchain synchronized!")
		default:
			fmt.Println("Invalid option. Please try again.")
		}
	}
}
