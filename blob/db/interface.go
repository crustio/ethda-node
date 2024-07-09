package db

type BlobDB interface {
	Put(key []byte, value []byte) error
	Get(key []byte) ([]byte, error)
	Has(key []byte) (bool, error)

	GetLatestZkBlob() (from uint64, to uint64, err error)
	HasFrom(from uint64) (has bool, err error)
	AddZkBlob(from uint64, to uint64) error
	Close()
}
