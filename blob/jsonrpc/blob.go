package blobjsonrpc

import (
	"context"

	"github.com/ethereum/go-ethereum/common"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
)

type GetBlobTxer interface {
	GetBlobTx(ctx context.Context, hash common.Hash) ([]byte, error)
	IsBlob(ctx context.Context, hash common.Hash) (bool, error)
}

func GetBlobTx(ctx context.Context, pool GetBlobTxer, hash common.Hash) (*ethTypes.Transaction, error) {
	existed, err := pool.IsBlob(ctx, hash)
	if err != nil {
		return nil, err
	}

	if existed {
		data, err := pool.GetBlobTx(ctx, hash)
		if err != nil {
			return nil, err
		}
		tx := new(ethTypes.Transaction)
		if err := tx.UnmarshalBinary(data); err != nil {
			return nil, err
		}

		return tx, nil
	}

	return nil, nil
}
