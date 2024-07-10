package blob

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func BlobTxToLegacyTx(tx types.Transaction) *types.Transaction {
	v, r, s := tx.RawSignatureValues()
	newV := 2*tx.ChainId().Uint64() + 35 + v.Uint64()

	return types.NewTx(&types.LegacyTx{
		Nonce:    tx.Nonce(),
		To:       tx.To(),
		Value:    tx.Value(),
		Gas:      tx.Gas(),
		GasPrice: tx.GasPrice(),
		Data:     tx.Data(),
		V:        big.NewInt(int64(newV)),
		R:        r,
		S:        s,
	})
}

func FilterLegacyTx(tx types.Transaction) *types.Transaction {
	if tx.Type() == types.LegacyTxType {
		return &tx
	}

	v, r, s := tx.RawSignatureValues()
	newV := 2*tx.ChainId().Uint64() + 35 + v.Uint64()

	return types.NewTx(&types.LegacyTx{
		Nonce:    tx.Nonce(),
		To:       tx.To(),
		Value:    tx.Value(),
		Gas:      tx.Gas(),
		GasPrice: tx.GasPrice(),
		Data:     tx.Data(),
		V:        big.NewInt(int64(newV)),
		R:        r,
		S:        s,
	})
}

func GetTxHash(tx types.Transaction) common.Hash {
	if tx.Type() == types.LegacyTxType {
		return tx.Hash()
	}

	ltx := BlobTxToLegacyTx(tx)

	return ltx.Hash()
}

// GetSender gets the sender from the transaction's signature
func GetSender(tx types.Transaction) (common.Address, error) {
	var signer types.Signer
	if tx.Type() == types.BlobTxType {
		signer = types.NewCancunSigner(tx.ChainId())
		legacyTx := BlobTxToLegacyTx(tx)
		return types.Sender(signer, legacyTx)
	} else {
		signer = types.NewEIP155Signer(tx.ChainId())
		sender, err := signer.Sender(&tx)
		if err != nil {
			return common.Address{}, err
		}
		return sender, nil
	}
}
