package services

import (
	"database/sql"
	"errors"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
)

// Returns client, main database, secondary database and error exactly in this order
func Connections(rpcLink, dbString, checkDbString string) (*ethclient.Client, *sql.DB, *sql.DB, error) {
	client, err := ethclient.Dial(rpcLink)
	if err != nil {
		return nil, nil, nil, err
	}
	checkDb, err := sql.Open("postgres", checkDbString)
	if err != nil {
		return nil, nil, nil, err
	}
	db, err := sql.Open("postgres", dbString)
	if err != nil {
		return nil, nil, nil, err
	}
	return client, db, checkDb, nil
}

func CreateTable(db *sql.DB, checkDb *sql.DB) error {
	_, err := checkDb.Exec(`
		CREATE TABLE IF NOT EXISTS "check" (
			id SERIAL PRIMARY KEY,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			block_number TEXT NOT NULL
		);
	`)
	if err != nil {
		return err
	}
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS "deposit" (
			id SERIAL PRIMARY KEY,
			from_address TEXT NOT NULL,
			to_address TEXT NOT NULL,
			amount REAL NOT NULL,
			amount_wei TEXT NOT NULL,
			is_verified BOOLEAN NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			verified_at TIMESTAMP
		);
	`)
	if err != nil {
		return err
	}
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS "deposit_tracker" (
			id SERIAL PRIMARY KEY,
			block_number INTEGER NOT NULL,
			deposit_id INTEGER REFERENCES "deposit"(id)
		);
	`)
	if err != nil {
		return err
	}
	return nil
}

func GetLastCheckedBlockNumber(checkDb *sql.DB) (string, error) {
	result, err := checkDb.Query(`SELECT * FROM "check" ORDER BY "created_at" DESC LIMIT 1;`)
	if err != nil {
		return "", err
	}
	var (
		id          int
		createdAt   time.Time
		blockNumber string
	)
	for result.Next() {
		err = result.Scan(&id, &createdAt, &blockNumber)
		if err != nil {
			if err == sql.ErrNoRows {
				return "", nil
			}
			return "", err
		}
	}
	return blockNumber, nil
}

func UpdatePreviousTransactions(db *sql.DB, latestBlockId string) error {
	num, ok := new(big.Int).SetString(latestBlockId, 10)
	if !ok {
		return errors.New("can't convert to big.Int")
	}
	limit := num.Uint64()
	_, err := db.Exec(`
		UPDATE "deposit"
		SET "is_verified" = true
		WHERE "id" IN (
			SELECT "deposit_id" FROM "deposit_tracker" WHERE "block_number" <= $1
		);
	`, limit-15)
	if err != nil {
		return err
	}
	return nil
}
