package fee

import (
	"context"

	"github.com/0xPolygonHermez/zkevm-node/state"
	"github.com/ethereum/go-ethereum/common"
	"github.com/jackc/pgx/v4"
)

// stateInterface gathers the methods required to interact with the state.
type stateInterface interface {
	GetBatchByNumber(ctx context.Context, batchNumber uint64, dbTx pgx.Tx) (*state.Batch, error)
}

// txPool contains the methods required to interact with the tx pool.
type txPool interface {
	IsBlob(ctx context.Context, hash common.Hash) (bool, error)
}
