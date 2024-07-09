package solidity

import "golang.org/x/crypto/sha3"

func Keccak256(value []byte) ([]byte, error) {
	hash := sha3.NewLegacyKeccak256()
	_, err := hash.Write(value)
	if err != nil {
		return nil, err
	}
	return hash.Sum(nil), nil
}
