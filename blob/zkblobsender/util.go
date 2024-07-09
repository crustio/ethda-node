package main

import (
	"fmt"
	"math/big"

	"github.com/0xPolygonHermez/zkevm-node/blob"
	"github.com/ethereum/go-ethereum/common"
)

func buildTreeAndGetRoots(hashes [][]common.Hash) (roots [][]byte, err error) {
	for i := 0; i < len(hashes); i++ {
		currents := hashes[i]
		tree, err := blob.NewMerkleTree(blob.ModeProofGen, currents...)
		if err != nil {
			return nil, fmt.Errorf("failed to create merkle tree: %s", err.Error())
		}
		roots = append(roots, tree.Root)
	}

	return
}

func parseBatchNumbers(batchNumbers []int64) []*big.Int {
	var bigBatchNumbers []*big.Int
	for _, batchNumber := range batchNumbers {
		bigBatchNumbers = append(bigBatchNumbers, big.NewInt(batchNumber))
	}

	return bigBatchNumbers
}

func parseRoots(roots [][]byte) [][32]byte {
	var bytes32Roots [][32]byte
	for _, root := range roots {
		bytes32Roots = append(bytes32Roots, common.BytesToHash(root))
	}

	return bytes32Roots
}
