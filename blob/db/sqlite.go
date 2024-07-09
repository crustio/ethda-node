package db

import (
	"database/sql"

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
