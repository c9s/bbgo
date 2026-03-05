package main

import (
	"flag"
	"os"

	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/service"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
)

func main() {
	var tradesFile string
	var driver string
	var dsn string
	var envFile string

	flag.StringVar(&tradesFile, "file", "", "path to trades JSON file")
	flag.StringVar(&driver, "driver", "", "database driver (mysql or sqlite3)")
	flag.StringVar(&dsn, "dsn", "", "database DSN")
	flag.StringVar(&envFile, "env", ".env.local", "path to environment file")
	flag.Parse()

	if tradesFile == "" {
		log.Fatal("please specify trades file with -file flag")
	}

	if _, err := os.Stat(tradesFile); os.IsNotExist(err) {
		log.Fatalf("trades file %s does not exist", tradesFile)
	}

	if envFile != "" {
		err := godotenv.Load(envFile)
		if err != nil {
			log.Warnf("failed to load environment file %s: %v", envFile, err)
		}
	}

	// Read from environment variables if not provided via flags
	if driver == "" {
		driver = os.Getenv("DB_DRIVER")
	}
	if dsn == "" {
		dsn = os.Getenv("DB_DSN")
	}

	// Validate that driver and dsn are set
	if driver == "" {
		log.Fatal("database driver not specified: use -driver flag or DB_DRIVER environment variable")
	}
	if dsn == "" {
		log.Fatal("database DSN not specified: use -dsn flag or DB_DSN environment variable")
	}

	// read a given JSON file and parse it as `[]types.Trade`
	var trades []types.Trade
	if err := util.ReadJsonFile(tradesFile, &trades); err != nil {
		log.Fatalf("failed to read trades from file %s: %v", tradesFile, err)
	}

	log.Printf("loaded %d trades from %s", len(trades), tradesFile)

	// connect to the database
	db := service.NewDatabaseService(driver, dsn)
	if err := db.Connect(); err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.DB.Close()

	// insert the trades into the database
	tradeService := &service.TradeService{DB: db.DB}
	inserted := 0
	for i, trade := range trades {
		if err := tradeService.Insert(trade); err != nil {
			log.Printf("warning: failed to insert trade #%d (ID: %d): %v", i, trade.ID, err)
		} else {
			inserted++
		}
	}

	log.Printf("successfully inserted %d out of %d trades", inserted, len(trades))
}
