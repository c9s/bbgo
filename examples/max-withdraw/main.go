package main

import (
	"context"
	"log"
	"os"
	"time"

	maxapi "github.com/c9s/bbgo/pkg/exchange/max/maxapi"
	flag "github.com/spf13/pflag"
)

func waitWithdrawalsComplete(ctx context.Context, client *maxapi.RestClient, currency string, limit int) error {
	var lastState string
	for {
		withdrawals, err := client.WithdrawalService.NewGetWithdrawalHistoryRequest().
			Currency(currency).
			Limit(limit).
			Do(ctx)
		if err != nil {
			return err
		}

		pending := false
		for _, withdrawal := range withdrawals {
			if lastState == "" {
				log.Printf("-> %s", withdrawal.State)
			} else if withdrawal.State != lastState {
				log.Printf("%s -> %s", lastState, withdrawal.State)
				log.Printf("withdrawal %s %s: %s", withdrawal.Amount, withdrawal.Currency, withdrawal.State)
				log.Printf("\t%+v", withdrawal)
			}
			lastState = withdrawal.State

			switch withdrawal.State {
			case "submitting", "submitted", "pending", "processing", "approved":
				pending = true

				log.Printf("there is a pending withdrawal request, waiting...")
				break

			case "sent", "confirmed":
				continue

			case "rejected":

			}

		}

		if !pending {
			break
		}

		time.Sleep(10 * time.Second)
	}

	return nil
}

func main() {
	var do bool
	var currency string
	var targetAddress string
	var amount float64
	flag.StringVar(&currency, "currency", "", "currency")
	flag.StringVar(&targetAddress, "targetAddress", "", "withdraw target address")
	flag.Float64Var(&amount, "amount", 0, "transfer amount")
	flag.BoolVar(&do, "do", false, "do")
	flag.Parse()

	key := os.Getenv("MAX_API_KEY")
	secret := os.Getenv("MAX_API_SECRET")

	if currency == "" {
		log.Fatal("--targetAddress is required")
	}

	if targetAddress == "" {
		log.Fatal("--targetAddress is required")
	}

	if amount == 0.0 {
		log.Fatal("--amount can not be zero")
	}

	ctx := context.Background()

	maxRest := maxapi.NewRestClient(maxapi.ProductionAPIURL)
	maxRest.Auth(key, secret)

	if err := waitWithdrawalsComplete(ctx, maxRest, currency, 1); err != nil {
		log.Fatal(err)
	}
	log.Printf("all withdrawals are sent, sending new withdrawal request...")

	addresses, err := maxRest.WithdrawalService.NewGetWithdrawalAddressesRequest().
		Currency(currency).Do(ctx)
	if err != nil {
		log.Fatal(err)
	}

	for _, address := range addresses {
		if address.Address == targetAddress {
			log.Printf("found address: %+v", address)
			if do {
				response, err := maxRest.WithdrawalService.NewWithdrawalRequest().
					Currency(currency).
					Amount(amount).
					AddressUUID(address.UUID).
					Do(ctx)
				if err != nil {
					log.Fatal(err)
				}
				log.Printf("withdrawal request response: %+v", response)
				break
			}
		}
	}

	if err := waitWithdrawalsComplete(ctx, maxRest, currency, 1); err != nil {
		log.Fatal(err)
	}
	log.Printf("all withdrawals are sent")
}
