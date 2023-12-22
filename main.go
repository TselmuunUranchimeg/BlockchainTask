package main

import (
	"context"
	"fmt"
	"math/big"
	"os"
	"sync"
	"time"

	_ "github.com/lib/pq"

	"blockchainTask/services"
)

func main() {
	// Necessary variables
	var (
		host          = os.Getenv("POSTGRESQL_HOST")
		port          = os.Getenv("POSTGRESQL_PORT")
		user          = os.Getenv("POSTGRESQL_USER")
		password      = os.Getenv("POSTGRESQL_PASSWORD")
		checkDbName   = os.Getenv("POSTGRESQL_CHECKUP")
		dbName        = os.Getenv("POSTGRESQL_DBNAME")
		rpcLink       = "https://binance.llamarpc.com" // "https://eth-sepolia.g.alchemy.com/v2/6UlG8ZHeQPuCSXhB1fDgPfKB0M91iyIs"
		dbString      = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", host, port, user, password, dbName)
		checkDbString = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", host, port, user, password, checkDbName)
		count         = 1
	)

	// Scheduler
	for {
		fmt.Printf("Starting run: %d\n", count)
		// Connections
		client, db, checkDb, err := services.Connections(rpcLink, dbString, checkDbString)
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		// Create table if not created
		err = services.CreateTable(db, checkDb)
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		// Get last checked and update those more than 15 blocks away
		lastChecked, err := services.GetLastCheckedBlockNumber(checkDb)
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		latestBlock, err := client.BlockByNumber(context.Background(), nil)
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		err = services.UpdatePreviousTransactions(db, latestBlock.Number().String())
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		if lastChecked == "" {
			// First run
			blockId, err := services.ProcessBlock(client, latestBlock, db, false)
			if err != nil {
				fmt.Println(err.Error())
				return
			}
			_, err = checkDb.Exec(`INSERT INTO "check"("block_number") VALUES($1);`, blockId)
			if err != nil {
				fmt.Println(err.Error())
				return
			}
		} else {
			// After first run

			// Find the difference
			lastCheckedNumber, ok := new(big.Int).SetString(lastChecked, 10)
			if !ok {
				fmt.Println("couldn't convert to number")
			}
			latestBlockNumber := latestBlock.Number()
			difference := new(big.Int).Sub(latestBlockNumber, lastCheckedNumber).Uint64()
			fmt.Printf("Difference is %d\n", difference)
			var i uint64
			var wg sync.WaitGroup
			ch := make(chan services.Response)

			// Iterate over blocks in between
			for i = 0; i < difference; i++ {
				block, err := client.BlockByNumber(context.Background(), new(big.Int).Add(lastCheckedNumber, big.NewInt(int64(i+1))))
				if err != nil {
					fmt.Println(err.Error())
					return
				}
				wg.Add(1)
				go services.ProcessBlockGoroutine(&wg, ch, i, client, block, db, difference-i > 15)
			}
			go func() {
				wg.Wait()
				close(ch)
			}()

			// Error handling and finding the latest block
			var target string
			for val := range ch {
				if val.Err != nil {
					fmt.Println(val.Err.Error())
					return
				}
				if val.Index == difference-1 {
					target = val.BlockId
					break
				}
			}

			// Save recently checked block number
			_, err = checkDb.Exec(`INSERT INTO "check"("block_number") VALUES($1);`, target)
			if err != nil {
				fmt.Println(err.Error())
				return
			}
		}

		if err = db.Close(); err != nil {
			fmt.Println(err.Error())
			return
		}
		if err = checkDb.Close(); err != nil {
			fmt.Println(err.Error())
			return
		}
		client.Close()
		fmt.Printf("Run: %d\n", count)
		count += 1
		time.Sleep(time.Second * 30)
	}
}
