package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/yiqi-017/blockchain/crypto"
	"github.com/yiqi-017/blockchain/storage"
)

// 简单工具：打印钱包地址（公钥 hex），若钱包不存在则生成
func main() {
	walletPath := flag.String("wallet", "data/n1/wallet.json", "钱包文件路径")
	flag.Parse()

	w, err := storage.LoadOrCreateWallet(*walletPath)
	if err != nil {
		log.Fatalf("load wallet failed: %v", err)
	}
	fmt.Println(crypto.PublicKeyHex(w.PublicKey))
}
