package main

import (
	"context"
	"database/sql"
	"math/big"
	"sync"
	"testing"
	"time"

	"blockchainTask/services"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	_ "github.com/lib/pq"
)

func TestProcessBlock(t *testing.T) {
	client, err := ethclient.Dial("https://eth-sepolia.g.alchemy.com/v2/6UlG8ZHeQPuCSXhB1fDgPfKB0M91iyIs")
	if err != nil {
		t.Fatal(err)
	}
	block, err := client.BlockByNumber(context.Background(), big.NewInt(4909916))
	if err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("postgres", "host=localhost port=5432 user=postgres password=tselmuun100 dbname=Blockchain sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	blockId, err := services.ProcessBlock(client, block, db, true)
	if err != nil || (err == nil && blockId == "") {
		t.Error(err)
	}
	if blockId != "4909916" {
		t.Fatalf("Block ids don't match. Got %s instead of 4909916\n", blockId)
	}
	t.Log(time.Since(start).String())
}

func TestProcessBlockGoroutine(t *testing.T) {
	client, err := ethclient.Dial("https://binance.llamarpc.com")
	if err != nil {
		t.Fatal(err)
	}
	blocks := [10]*types.Block{}
	for i := 0; i < 10; i++ {
		blocks[i], err = client.BlockByNumber(context.Background(), big.NewInt(34535894))
		if err != nil {
			t.Fatal(err)
		}
	}
	db, err := sql.Open("postgres", "host=localhost port=5432 user=postgres password=tselmuun100 dbname=Blockchain sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	var wg sync.WaitGroup
	ch := make(chan services.Response)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go services.ProcessBlockGoroutine(&wg, ch, uint64(i), client, blocks[i], db, true)
	}
	go func() {
		wg.Wait()
		close(ch)
	}()
	for val := range ch {
		if val.Err != nil {
			t.Fatal(val.Err)
		}
	}
	t.Log(time.Since(start).String())
}
