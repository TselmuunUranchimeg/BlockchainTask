package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

const (
	contract  = "0x55d398326f99059fF775485246999027B3197955"
	from      = "0x4982085C9e2F89F2eCb8131Eca71aFAD896e89CB"
	abiString = `[{"inputs":[],"payable":false,"stateMutability":"nonpayable","type":"constructor"},{"anonymous":false,"inputs":[{"indexed":true,"internalType":"address","name":"owner","type":"address"},{"indexed":true,"internalType":"address","name":"spender","type":"address"},{"indexed":false,"internalType":"uint256","name":"value","type":"uint256"}],"name":"Approval","type":"event"},{"anonymous":false,"inputs":[{"indexed":true,"internalType":"address","name":"previousOwner","type":"address"},{"indexed":true,"internalType":"address","name":"newOwner","type":"address"}],"name":"OwnershipTransferred","type":"event"},{"anonymous":false,"inputs":[{"indexed":true,"internalType":"address","name":"from","type":"address"},{"indexed":true,"internalType":"address","name":"to","type":"address"},{"indexed":false,"internalType":"uint256","name":"value","type":"uint256"}],"name":"Transfer","type":"event"},{"constant":true,"inputs":[],"name":"_decimals","outputs":[{"internalType":"uint8","name":"","type":"uint8"}],"payable":false,"stateMutability":"view","type":"function"},{"constant":true,"inputs":[],"name":"_name","outputs":[{"internalType":"string","name":"","type":"string"}],"payable":false,"stateMutability":"view","type":"function"},{"constant":true,"inputs":[],"name":"_symbol","outputs":[{"internalType":"string","name":"","type":"string"}],"payable":false,"stateMutability":"view","type":"function"},{"constant":true,"inputs":[{"internalType":"address","name":"owner","type":"address"},{"internalType":"address","name":"spender","type":"address"}],"name":"allowance","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"payable":false,"stateMutability":"view","type":"function"},{"constant":false,"inputs":[{"internalType":"address","name":"spender","type":"address"},{"internalType":"uint256","name":"amount","type":"uint256"}],"name":"approve","outputs":[{"internalType":"bool","name":"","type":"bool"}],"payable":false,"stateMutability":"nonpayable","type":"function"},{"constant":true,"inputs":[{"internalType":"address","name":"account","type":"address"}],"name":"balanceOf","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"payable":false,"stateMutability":"view","type":"function"},{"constant":false,"inputs":[{"internalType":"uint256","name":"amount","type":"uint256"}],"name":"burn","outputs":[{"internalType":"bool","name":"","type":"bool"}],"payable":false,"stateMutability":"nonpayable","type":"function"},{"constant":true,"inputs":[],"name":"decimals","outputs":[{"internalType":"uint8","name":"","type":"uint8"}],"payable":false,"stateMutability":"view","type":"function"},{"constant":false,"inputs":[{"internalType":"address","name":"spender","type":"address"},{"internalType":"uint256","name":"subtractedValue","type":"uint256"}],"name":"decreaseAllowance","outputs":[{"internalType":"bool","name":"","type":"bool"}],"payable":false,"stateMutability":"nonpayable","type":"function"},{"constant":true,"inputs":[],"name":"getOwner","outputs":[{"internalType":"address","name":"","type":"address"}],"payable":false,"stateMutability":"view","type":"function"},{"constant":false,"inputs":[{"internalType":"address","name":"spender","type":"address"},{"internalType":"uint256","name":"addedValue","type":"uint256"}],"name":"increaseAllowance","outputs":[{"internalType":"bool","name":"","type":"bool"}],"payable":false,"stateMutability":"nonpayable","type":"function"},{"constant":false,"inputs":[{"internalType":"uint256","name":"amount","type":"uint256"}],"name":"mint","outputs":[{"internalType":"bool","name":"","type":"bool"}],"payable":false,"stateMutability":"nonpayable","type":"function"},{"constant":true,"inputs":[],"name":"name","outputs":[{"internalType":"string","name":"","type":"string"}],"payable":false,"stateMutability":"view","type":"function"},{"constant":true,"inputs":[],"name":"owner","outputs":[{"internalType":"address","name":"","type":"address"}],"payable":false,"stateMutability":"view","type":"function"},{"constant":false,"inputs":[],"name":"renounceOwnership","outputs":[],"payable":false,"stateMutability":"nonpayable","type":"function"},{"constant":true,"inputs":[],"name":"symbol","outputs":[{"internalType":"string","name":"","type":"string"}],"payable":false,"stateMutability":"view","type":"function"},{"constant":true,"inputs":[],"name":"totalSupply","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"payable":false,"stateMutability":"view","type":"function"},{"constant":false,"inputs":[{"internalType":"address","name":"recipient","type":"address"},{"internalType":"uint256","name":"amount","type":"uint256"}],"name":"transfer","outputs":[{"internalType":"bool","name":"","type":"bool"}],"payable":false,"stateMutability":"nonpayable","type":"function"},{"constant":false,"inputs":[{"internalType":"address","name":"sender","type":"address"},{"internalType":"address","name":"recipient","type":"address"},{"internalType":"uint256","name":"amount","type":"uint256"}],"name":"transferFrom","outputs":[{"internalType":"bool","name":"","type":"bool"}],"payable":false,"stateMutability":"nonpayable","type":"function"},{"constant":false,"inputs":[{"internalType":"address","name":"newOwner","type":"address"}],"name":"transferOwnership","outputs":[],"payable":false,"stateMutability":"nonpayable","type":"function"}]`
)

type LogTransfer struct {
	From, To common.Address
	Tokens   *big.Int
}
type Response struct {
	BlockId string
	Err     error
	Index   uint64
}

// Send technical error. If it is logical error, just quietly quit.
func ProcessTransaction(wg *sync.WaitGroup, ch chan error, tx *types.Transaction, client *ethclient.Client, db *sql.DB, isVerified bool) {
	defer wg.Done()
	if tx.To() == nil || tx.To().Hex() != contract {
		return
	}
	receipt, err := client.TransactionReceipt(context.Background(), tx.Hash())
	if err != nil {
		ch <- err
		return
	}
	transferHash := crypto.Keccak256Hash([]byte("Transfer(address,address,uint256)")).Hex()
	contractAbi, err := abi.JSON(strings.NewReader(abiString))
	if err != nil {
		ch <- err
		return
	}
	for _, receiptLog := range receipt.Logs {
		if len(receiptLog.Topics) < 3 {
			continue
		}
		if receiptLog.Topics[0].Hex() != transferHash || common.HexToAddress(receiptLog.Topics[1].Hex()).String() != from {
			continue
		}
		obj, err := contractAbi.Unpack("Transfer", receiptLog.Data)
		if err != nil {
			ch <- err
			return
		}
		amountWei := obj[0].(*big.Int)
		amountBigFloat, ok := new(big.Float).SetString(amountWei.String())
		if !ok {
			ch <- errors.New("can't convert string to *big.Float")
			continue
		}
		amount, _ := new(big.Float).Quo(amountBigFloat, big.NewFloat(math.Pow10(18))).Float64()
		fmt.Printf("Transaction hash: %s\n", tx.Hash())
		to := common.HexToAddress(receiptLog.Topics[2].Hex()).String()

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
		`, from, to, amount, amountWei.String(), isVerified).Scan(&depositId)
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
