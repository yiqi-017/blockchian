package crypto

import (
	"crypto/sha256"
	"encoding/hex"
)

// Hash256 对数据做一次 SHA-256
func Hash256(data []byte) []byte {
	sum := sha256.Sum256(data)
	return sum[:]
}

// DoubleHash256 对数据做两次 SHA-256（常用于区块链哈希）
func DoubleHash256(data []byte) []byte {
	first := Hash256(data)
	sum := sha256.Sum256(first)
	return sum[:]
}

// HexEncode 将字节切片编码为十六进制字符串
func HexEncode(b []byte) string {
	return hex.EncodeToString(b)
}

// HexDecode 将十六进制字符串解码为字节切片
func HexDecode(s string) ([]byte, error) {
	return hex.DecodeString(s)
}
