package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
)

// 定义交易结构
type Transaction struct {
	Sender    string
	Recipient string
	Amount    int
	Signature string
}

// 全局交易池
var TransactionPool []Transaction

// 用户结构，包含公私钥
type User struct {
	PrivateKey *ecdsa.PrivateKey
	PublicKey  *ecdsa.PublicKey
}

// 创建用户（生成公私钥对）
func createUser() *User {
	privateKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	return &User{
		PrivateKey: privateKey,
		PublicKey:  &privateKey.PublicKey,
	}
}

// 对交易进行签名
func signTransaction(transaction Transaction, privateKey *ecdsa.PrivateKey) string {
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s%s%d", transaction.Sender, transaction.Recipient, transaction.Amount)))
	r, s, _ := ecdsa.Sign(rand.Reader, privateKey, hash[:])
	return hex.EncodeToString(r.Bytes()) + hex.EncodeToString(s.Bytes())
}

// 验证交易签名
func verifyTransaction(transaction Transaction, publicKey *ecdsa.PublicKey) bool {
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s%s%d", transaction.Sender, transaction.Recipient, transaction.Amount)))
	signature, err := hex.DecodeString(transaction.Signature)
	if err != nil || len(signature) != 64 {
		return false
	}
	r := big.NewInt(0).SetBytes(signature[:32])
	s := big.NewInt(0).SetBytes(signature[32:])
	return ecdsa.Verify(publicKey, hash[:], r, s)
}

// 添加交易到交易池
// func addTransaction(senderUser *User, recipient string, amount int) {
// 	transaction := Transaction{
// 		Sender:    hex.EncodeToString(senderUser.PublicKey.X.Bytes()), // 使用公钥作为发送者标识
// 		Recipient: recipient,
// 		Amount:    amount,
// 	}
// 	transaction.Signature = signTransaction(transaction, senderUser.PrivateKey)
// 	TransactionPool = append(TransactionPool, transaction)
// 	fmt.Println("Transaction added:", transaction)
// }

func addTransaction(senderUser *User, recipient string, amount int) {
	transaction := Transaction{
		Sender:    hex.EncodeToString(senderUser.PublicKey.X.Bytes()), // 使用公钥作为发送者标识
		Recipient: recipient,
		Amount:    amount,
	}
	transaction.Signature = signTransaction(transaction, senderUser.PrivateKey)

	// 计算交易属于哪个分片
	shardIndex := getShardIndex(transaction.Sender)
	if shardIndex == getShardIndex(CurrentNode.Address) {
		TransactionPool = append(TransactionPool, transaction)
		fmt.Println("Transaction added to pool:", transaction)
	} else {
		fmt.Println("Transaction does not belong to this shard")
	}
}

// 清空交易池并返回交易列表
func clearTransactionPool() []Transaction {
	transactions := TransactionPool
	TransactionPool = []Transaction{} // 清空交易池
	return transactions
}

func parseBlocks(data interface{}) ([]Block, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	var blocks []Block
	err = json.Unmarshal(jsonData, &blocks)
	return blocks, err
}

func parseTransactions(data interface{}) ([]Transaction, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	var transactions []Transaction
	err = json.Unmarshal(jsonData, &transactions)
	return transactions, err
}
