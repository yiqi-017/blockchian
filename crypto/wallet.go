package crypto

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"math/big"
)

var (
	curve      = elliptic.P256()
	pubKeySize = curve.Params().BitSize / 8 // 256-bit => 32 bytes
)

// Wallet 封装 ECDSA 密钥对
type Wallet struct {
	PrivateKey *ecdsa.PrivateKey
	PublicKey  []byte // 非压缩形式：X||Y
}

// GenerateWallet 生成新的 ECDSA 密钥对
func GenerateWallet() (*Wallet, error) {
	priv, err := ecdsa.GenerateKey(curve, rand.Reader)
	if err != nil {
		return nil, err
	}
	return &Wallet{
		PrivateKey: priv,
		PublicKey:  serializePubKey(&priv.PublicKey),
	}, nil
}

// Sign 对数据进行 SHA-256 后签名（ASN.1 编码）
func (w *Wallet) Sign(data []byte) ([]byte, error) {
	if w == nil || w.PrivateKey == nil {
		return nil, errors.New("wallet private key is nil")
	}
	digest := sha256.Sum256(data)
	return ecdsa.SignASN1(rand.Reader, w.PrivateKey, digest[:])
}

// Verify 使用公钥验证签名
func Verify(pubKey []byte, data []byte, sig []byte) bool {
	if len(pubKey) != 2*pubKeySize {
		return false
	}
	pub := deserializePubKey(pubKey)
	if pub == nil {
		return false
	}
	digest := sha256.Sum256(data)
	return ecdsa.VerifyASN1(pub, digest[:], sig)
}

// PublicKeyHex 将公钥编码为十六进制字符串
func PublicKeyHex(pubKey []byte) string {
	return hex.EncodeToString(pubKey)
}

// serializePubKey 将公钥序列化为非压缩 64 字节
func serializePubKey(pub *ecdsa.PublicKey) []byte {
	if pub == nil {
		return nil
	}
	xBytes := pub.X.FillBytes(make([]byte, pubKeySize))
	yBytes := pub.Y.FillBytes(make([]byte, pubKeySize))
	return append(xBytes, yBytes...)
}

// deserializePubKey 从非压缩 64 字节还原公钥
func deserializePubKey(b []byte) *ecdsa.PublicKey {
	if len(b) != 2*pubKeySize {
		return nil
	}
	x := new(big.Int).SetBytes(b[:pubKeySize])
	y := new(big.Int).SetBytes(b[pubKeySize:])
	if !curve.IsOnCurve(x, y) {
		return nil
	}
	return &ecdsa.PublicKey{Curve: curve, X: x, Y: y}
}
