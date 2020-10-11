package main

import (
	"log"
	"os"

	maxapi "github.com/c9s/bbgo/pkg/exchange/max/maxapi"
)

func main() {
	key := os.Getenv("MAX_API_KEY")
	secret := os.Getenv("MAX_API_SECRET")

	maxRest := maxapi.NewRestClient(maxapi.ProductionAPIURL)
	maxRest.Auth(key, secret)

	orders, err := maxRest.OrderService.All("maxusdt", 100, 1, maxapi.OrderStateDone)
	if err != nil {
		log.Fatal(err)
	}

	for _, order := range orders {
		log.Printf("%+v", order)
	}
}
