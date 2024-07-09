package solidity

import (
	"crypto/ecdsa"

	"github.com/ethereum/go-ethereum/crypto"
)

func Sign(bytesToSign []byte, privateKey *ecdsa.PrivateKey) ([]byte, error) {
	sig, err := crypto.Sign(bytesToSign, privateKey)
	if err != nil {
		return nil, err
	}
	sig[64] += 27

	return sig, nil
}
