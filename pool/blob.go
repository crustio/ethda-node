package pool

import (
	"context"
	"fmt"
	"math/big"

	"github.com/0xPolygonHermez/zkevm-node/blob"
	"github.com/0xPolygonHermez/zkevm-node/blob/db"
	"github.com/0xPolygonHermez/zkevm-node/blob/fee"
	"github.com/0xPolygonHermez/zkevm-node/log"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func GetBlobTxSender(tx types.Transaction) (common.Address, error) {
	var signer types.Signer
	signer = types.NewCancunSigner(tx.ChainId())

	legacyTx := blob.BlobTxToLegacyTx(tx)

	return types.Sender(signer, legacyTx)
}

func (p *Pool) validateBlobTx(ctx context.Context, tx types.Transaction) error {
	// gets tx sender for validations
	from, err := GetBlobTxSender(tx)
	if err != nil {
		return ErrInvalidSender
	}

	log.Infof("from: ", from.Hex())

	// Check das address
	toAddress := common.HexToAddress(p.cfg.DasAddress)
	if tx.To() == nil || *tx.To() != toAddress {
		return fmt.Errorf("blob transaction to address expect %v but got %v", toAddress, tx.To())
	}

	lastL2Block, err := p.state.GetLastL2Block(ctx, nil)
	if err != nil {
		return err
	}

	// Transactor should have enough funds to cover the costs
	// cost == V + GP * GL
	balance, err := p.state.GetBalance(ctx, from, lastL2Block.Root())
	if err != nil {
		return err
	}

	if balance.Cmp(tx.Cost()) < 0 {
		return ErrInsufficientFunds
	}

	if tx.Value().Cmp(new(big.Int).Sub(tx.Cost(), tx.Value())) < 0 {
		// TODO value - (cost - value - gasUsed) < 0
		return fmt.Errorf("value is less than blob cost, %s < %s", tx.Value().String(), tx.Cost())
	}

	_, excess, err := p.blobDB.GetBlobGasUsedAndExcessBlobGas(lastL2Block.NumberU64())
	if err != nil && err != db.ErrBlobGasNotFound {
		log.Errorf("BlobBaseFee: Failed to get blob gas used and excess blob gas: %v", err)
		return fmt.Errorf("failed to get blob gas used and excess blob gas: %v", err)
	}

	l2BlobFeeCap := fee.GetL2BlobFeeCap(excess)
	if tx.BlobGasFeeCapIntCmp(l2BlobFeeCap) == -1 {
		return fmt.Errorf("blob gas fee is less than base gas fee, %s < %s", tx.BlobGasFeeCap().String(), l2BlobFeeCap.String())
	}

	return nil
}
