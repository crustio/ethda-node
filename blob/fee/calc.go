package fee

import (
	"context"
	"math/big"

	"github.com/0xPolygonHermez/zkevm-node/blob/db"
	"github.com/0xPolygonHermez/zkevm-node/blob/eip4844"
	"github.com/0xPolygonHermez/zkevm-node/state"
	"github.com/ethereum/go-ethereum/params"
)

const (
	MinBlobBaseFee = 1000000000
)

func CalcBlobGasUsed(ctx context.Context, s stateInterface, blobDB db.BlobDB, batchNumber uint64) uint64 {
	batch, err := s.GetBatchByNumber(ctx, batchNumber, nil)
	if err != nil {
		return 0 // TODO handle error
	}
	brb, er := state.DecodeBatchV2(batch.BatchL2Data)
	if er != nil {
		return 0 // TODO handle error
	}

	var count uint64 = 0
	for _, btx := range brb.Blocks {
		for _, tx := range btx.Transactions {
			if ok, _ := blobDB.IsBlob(ctx, tx.Tx.Hash()); ok {
				count++
			}
		}
	}

	return uint64(count * params.BlobTxBlobGasPerBlob)
}

func GetL2BlobFeeCap(excessBlobGas *uint64) *big.Int {
	if excessBlobGas == nil {
		return big.NewInt(MinBlobBaseFee)
	}

	blobFee := eip4844.CalcBlobFee(*excessBlobGas)
	if blobFee.Cmp(big.NewInt(MinBlobBaseFee)) == -1 {
		blobFee = big.NewInt(MinBlobBaseFee)
	}

	return blobFee
}
