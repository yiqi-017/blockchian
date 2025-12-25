package storage

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/yiqi-017/blockchain/crypto"
)

type walletPersist struct {
	PrivateHex string `json:"private_hex"`
}

// LoadOrCreateWallet 从文件加载钱包，不存在则生成新钱包并保存
func LoadOrCreateWallet(path string) (*crypto.Wallet, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		var wp walletPersist
		if err := json.Unmarshal(data, &wp); err != nil {
			return nil, err
		}
		if wp.PrivateHex == "" {
			return nil, errors.New("wallet file missing private key")
		}
		return crypto.FromPrivateHex(wp.PrivateHex)
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	// create new
	w, err := crypto.GenerateWallet()
	if err != nil {
		return nil, err
	}
	if err := SaveWallet(path, w); err != nil {
		return nil, err
	}
	return w, nil
}

// SaveWallet 覆盖保存钱包到文件
func SaveWallet(path string, w *crypto.Wallet) error {
	privHex, err := crypto.PrivateKeyHex(w)
	if err != nil {
		return err
	}
	payload := walletPersist{PrivateHex: privHex}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

