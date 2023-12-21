package services

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

const (
	contract = "0x55d398326f99059fF775485246999027B3197955"
	from     = "4982085c9e2f89f2ecb8131eca71afad896e89cb"
)

func parseHash(hash *string) {
	for {
		if len(*hash) < 2 || ((*hash)[0] != 'x' && (*hash)[0] != '0') {
			return
		}
		*hash = (*hash)[1:]
	}
}

type Response struct {
	BlockId string
	Err     error
	Index   uint64
}

// Send technical error. If it is logical error, just quietly quit.
func ProcessTransaction(wg *sync.WaitGroup, ch chan error, tx *types.Transaction, client *ethclient.Client, db *sql.DB, isVerified bool) {
	defer wg.Done()
	receipt, err := client.TransactionReceipt(context.Background(), tx.Hash())
	if err != nil {
		ch <- err
		return
	}
	if tx.To() == nil || tx.To().Hex() != contract {
		return
	}
	for _, receiptLog := range receipt.Logs {

		if len(receiptLog.Topics) < 3 {
			continue
		}
		s := receiptLog.Topics[1].Hex()
		parseHash(&s)
		if s != from {
			continue
		}
		fmt.Printf("Transaction hash: %s\n", tx.Hash())
		to := receiptLog.Topics[2].Hex()
		parseHash(&to)
		amount := new(big.Int).Quo(tx.Value(), big.NewInt(int64(math.Pow10(18)))).Uint64()

		// Start transaction
		_, err = db.Exec("BEGIN;")
		if err != nil {
			ch <- err
			return
		}

		// Insert new row
		var depositId int
		err = db.QueryRow(`
			INSERT INTO "deposit"("from_address", "to_address", "amount", "amount_wei", "is_verified")
			VALUES($1, $2, $3, $4, $5)
			RETURNING id;
		`, from, to, amount, tx.Value().String(), isVerified).Scan(&depositId)
		if err != nil {
			ch <- err
			return
		}

		// Insert new row into "deposit_tracker"
		_, err = db.Exec(`
			INSERT INTO "deposit_tracker"("block_number", "deposit_id")
			VALUES($1, $2)
		`, receipt.BlockNumber.Uint64(), depositId)
		if err != nil {
			ch <- err
			return
		}

		// Commit to finish transaction
		_, err = db.Exec("COMMIT;")
		if err != nil {
			ch <- err
			return
		}
	}
}

func ProcessBlockGoroutine(wg *sync.WaitGroup, ch chan Response, index uint64, client *ethclient.Client, block *types.Block, db *sql.DB, isVerified bool) {
	defer wg.Done()
	blockId, err := ProcessBlock(client, block, db, isVerified)
	ch <- Response{
		BlockId: blockId,
		Err:     err,
		Index:   index,
	}
}

func ProcessBlock(client *ethclient.Client, block *types.Block, db *sql.DB, isVerified bool) (string, error) {
	var wg sync.WaitGroup
	ch := make(chan error)
	fmt.Printf("Total amount of transactions is %d\n", len(block.Transactions()))
	for _, tx := range block.Transactions() {
		wg.Add(1)
		go ProcessTransaction(&wg, ch, tx, client, db, isVerified)
	}
	go func() {
		wg.Wait()
		close(ch)
	}()
	for val := range ch {
		if val != nil {
			return "", val
		}
	}
	blockId := block.Number()
	return blockId.String(), nil
}
