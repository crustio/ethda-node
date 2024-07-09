package solidity

import (
	"math/big"

	smt "github.com/FantasyJony/openzeppelin-merkle-tree-go/standard_merkle_tree"
)

// AbiPack packs types and values into a single byte array
// example: AbiPack([]string{"uint256", "bytes32"}, big.NewInt(testBatch.BatchNum), common.BytesToHash(root))
// TODO but wrong: AbiPack([]string{"uint256[]", "bytes32[]"}, []*big.Int{big.NewInt(testBatch.BatchNum)}, [][32]byte{common.BytesToHash(root)}); now use ArrayPack instead
func AbiPack(types []string, values ...interface{}) ([]byte, error) {
	return smt.AbiPack(types, values...)
}

// ArrayPack packs int256 array and bytes32 array into a single byte array
func ArrayPack(int256Arr []*big.Int, bytes32Arr [][32]byte) []byte {
	var bytesPack []byte

	// pack int256 array
	for _, num := range int256Arr {
		numBytes := num.Bytes()
		// ensure each number is 32 bytes
		if len(numBytes) < 32 {
			leadingZeros := make([]byte, 32-len(numBytes))
			numBytes = append(leadingZeros, numBytes...)
		}
		bytesPack = append(bytesPack, numBytes...)
	}

	// pack bytes32 array
	for _, root := range bytes32Arr {
		// length of root is 32
		bytesPack = append(bytesPack, root[:]...)
	}

	return bytesPack
}
