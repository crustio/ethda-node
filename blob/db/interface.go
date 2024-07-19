package db

import (
	"context"

	"github.com/ethereum/go-ethereum/common"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
)

type BlobDB interface {

	// blob
	Put(key []byte, value []byte) error
	Get(key []byte) ([]byte, error)
	Has(key []byte) (bool, error)
	GetBlobTx(ctx context.Context, hash common.Hash) (*ethTypes.Transaction, error)
	IsBlob(ctx context.Context, hash common.Hash) (bool, error)

	// zkblob
	GetLatestZkBlob() (from uint64, to uint64, err error)
	HasFrom(from uint64) (has bool, err error)
	AddZkBlob(from uint64, to uint64) error

	// blob gas fee
	UpdateBlobGasUsedAndExcessBlobGas(blockNum uint64, gasUsed uint64, excessGas uint64) error
	GetBlobGasUsedAndExcessBlobGas(blockNum uint64) (uint64, uint64, error)

	// close
	Close()
}
