package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	ethTypes "github.com/ethereum/go-ethereum/core/types"

	_ "modernc.org/sqlite"
)

// NewBlobDB creates a new sqlite3 BlobDB instance
func NewBlobDB(path string) (BlobDB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	// Create the table
	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS blocks (
		hash TEXT PRIMARY KEY, 
		data BLOB
	);
	CREATE TABLE IF NOT EXISTS blobs (
		from_batch_num INTEGER NOT NULL UNIQUE,
		to_batch_num INTEGER
	);
	CREATE TABLE IF NOT EXISTS blobgas (
		block_num INTEGER NOT NULL UNIQUE,
		blob_gas_used INTEGER NOT NULL,
		excess_blob_gas INTEGER NOT NULL
	);
	`)
	if err != nil {
		return nil, err
	}

	return &SqliteBlobDB{
		db: db,
	}, nil
}

type SqliteBlobDB struct {
	db *sql.DB
}

func (s *SqliteBlobDB) Put(key []byte, value []byte) error {
	_, err := s.db.Exec(`INSERT INTO blocks (hash, data) VALUES (?, ?)`, string(key), value)
	if err != nil {
		return err
	}

	return nil
}

func (s *SqliteBlobDB) Get(key []byte) ([]byte, error) {
	var retrievedHash string
	var retrievedData []byte
	err := s.db.QueryRow(`SELECT hash, data FROM blocks WHERE hash = ?`, string(key)).Scan(&retrievedHash, &retrievedData)
	if err != nil {
		return nil, err
	}

	return retrievedData, nil
}

func (s *SqliteBlobDB) Has(key []byte) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM blocks WHERE hash = ?)`
	var exists bool
	err := s.db.QueryRow(query, string(key)).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}

/**
* GetLatestZkBlob gets the latest zk blob from the database
* use for zkblob-sender
 */
func (s *SqliteBlobDB) GetLatestZkBlob() (from uint64, to uint64, err error) {
	from, to = 0, 0
	// Query
	row := s.db.QueryRow("SELECT from_batch_num, to_batch_num FROM blobs ORDER BY from_batch_num DESC LIMIT 1")

	// parse result
	err = row.Scan(&from, &to)
	if err != nil {
		if err == sql.ErrNoRows {
			// no raws, from = 0, to = 0
			// log.Warn("no rows in blobs table, set from = 0, to = 0")
			return 0, 0, nil
		} else {
			return 0, 0, err
		}
	}

	return from, to, nil
}

/**
* AddZkBlob saves the latest zk blob to the database
* use for zkblob-sender
 */
func (s *SqliteBlobDB) AddZkBlob(from uint64, to uint64) error {
	_, err := s.db.Exec("INSERT INTO blobs (from_batch_num, to_batch_num) VALUES (?, ?)", from, to)
	if err != nil {
		return err
	}

	return nil
}

func (s *SqliteBlobDB) HasFrom(from uint64) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM blobs WHERE from_batch_num = ?)`
	exists := false
	err := s.db.QueryRow(query, from).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}

// TODO Close db when pool is closed
func (s *SqliteBlobDB) Close() {
	s.db.Close()
}

func (s *SqliteBlobDB) GetBlobTx(ctx context.Context, hash common.Hash) (*ethTypes.Transaction, error) {
	existed, err := s.IsBlob(ctx, hash)
	if err != nil {
		return nil, err
	}

	if existed {
		data, err := s.Get([]byte(fmt.Sprintf("blob-%s", hash.Hex())))
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

func (s *SqliteBlobDB) IsBlob(ctx context.Context, hash common.Hash) (bool, error) {
	return s.Has([]byte(fmt.Sprintf("blob-%s", hash.Hex())))
}

func (s *SqliteBlobDB) UpdateBlobGasUsedAndExcessBlobGas(blockNum uint64, gasUsed uint64, excessGas uint64) error {
	sqlStmt := `
	INSERT INTO blobgas (block_num, blob_gas_used, excess_blob_gas)
	VALUES (?, ?, ?)
	ON CONFLICT(block_num) DO UPDATE SET blob_gas_used = excluded.blob_gas_used, excess_blob_gas = excluded.excess_blob_gas
	`

	stmt, err := s.db.Prepare(sqlStmt)
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(blockNum, gasUsed, excessGas)
	if err != nil {
		return err
	}

	return nil
}

var (
	ErrBlobGasNotFound = errors.New("blob gas not found")
)

func (s *SqliteBlobDB) GetBlobGasUsedAndExcessBlobGas(blockNum uint64) (uint64, uint64, error) {
	sqlStmt := `
	SELECT block_num, blob_gas_used, excess_blob_gas
	FROM blobgas
	WHERE block_num = ?
	`

	rows, err := s.db.Query(sqlStmt, blockNum)
	if err != nil {
		return 0, 0, err
	}
	defer rows.Close()

	var num, gasUsed, excessGas uint64
	for rows.Next() {
		err = rows.Scan(&num, &gasUsed, &excessGas)
		if err != nil {
			if err == sql.ErrNoRows {
				return 0, 0, ErrBlobGasNotFound
			}

			return 0, 0, err
		}
	}

	return gasUsed, excessGas, nil
}
