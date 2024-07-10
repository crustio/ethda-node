package types

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/0xPolygonHermez/zkevm-node/blob"
	"github.com/0xPolygonHermez/zkevm-node/blob/db"
	blobtypes "github.com/0xPolygonHermez/zkevm-node/blob/types"
	"github.com/0xPolygonHermez/zkevm-node/state"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto/kzg4844"
)

type BlobTxSidecar struct {
	Blobs       []kzg4844.Blob       `json:"blobs"`       // Blobs needed by the blob pool
	Commitments []kzg4844.Commitment `json:"commitments"` // Commitments needed by the blob pool
	Proofs      []kzg4844.Proof      `json:"proofs"`      // Proofs needed by the blob pool
}

func NewBlobTransaction(tx types.Transaction,
	receipt *types.Receipt,
	includeReceipt bool,
	withBlob bool,
	l2Hash *common.Hash,
) (*Transaction, error) {
	ltx := blob.BlobTxToLegacyTx(tx)
	v, r, s := ltx.RawSignatureValues()
	from, _ := state.GetSender(*ltx)

	ff, err := json.Marshal(tx)
	if err != nil {
		return nil, err
	}

	fmt.Printf("%+v\n", string(ff))
	res := &Transaction{
		Nonce:            ArgUint64(tx.Nonce()),
		GasPrice:         ArgBig(*tx.GasPrice()),
		Gas:              ArgUint64(tx.Gas()),
		To:               tx.To(),
		Value:            ArgBig(*tx.Value()),
		Input:            tx.Data(),
		V:                ArgBig(*v),
		R:                ArgBig(*r),
		S:                ArgBig(*s),
		Hash:             ltx.Hash(),
		From:             from,
		ChainID:          ArgBig(*tx.ChainId()),
		Type:             ArgUint64(tx.Type()),
		BlobHashes:       tx.BlobHashes(),
		MaxFeePerBlobGas: ArgBig(*tx.BlobGasFeeCap()),
	}

	if withBlob {
		res.Sidecar = &blobtypes.BlobTxSidecar{
			Blobs:       tx.BlobTxSidecar().Blobs,
			Commitments: tx.BlobTxSidecar().Commitments,
			Proofs:      tx.BlobTxSidecar().Proofs,
		}
	}

	if receipt != nil {
		bn := ArgUint64(receipt.BlockNumber.Uint64())
		res.BlockNumber = &bn
		res.BlockHash = &receipt.BlockHash
		ti := ArgUint64(receipt.TransactionIndex)
		res.TxIndex = &ti
		rpcReceipt, err := NewReceipt(*blob.BlobTxToLegacyTx(tx), receipt, l2Hash)
		if err != nil {
			return nil, err
		}
		if includeReceipt {
			res.Receipt = &rpcReceipt
		}
	}

	return res, nil
}

func GetBlobTransaction(hash common.Hash, receipt *types.Receipt, includeReceipt bool, l2Hash *common.Hash) (*Transaction, error) {
	sqliteDB, err := db.NewBlobDB("/blob/sqlite.db")
	if err != nil {
		panic(err)
	}

	existed, err := sqliteDB.IsBlob(context.Background(), hash)
	if err != nil {
		return nil, err
	}

	if existed {
		btx, err := sqliteDB.GetBlobTx(context.Background(), hash)
		if err != nil {
			return nil, err
		}
		return NewBlobTransaction(*btx, receipt, includeReceipt, true, l2Hash)
	}

	return nil, nil
}
