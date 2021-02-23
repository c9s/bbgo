package main

import (
	"context"
	"log"
	"os"

	maxapi "github.com/c9s/bbgo/pkg/exchange/max/maxapi"
)

func main() {

	key := os.Getenv("MAX_API_KEY")
	secret := os.Getenv("MAX_API_SECRET")

	api := maxapi.NewRestClient(maxapi.ProductionAPIURL)
	api.Auth(key, secret)

	ctx := context.Background()

	var req *maxapi.RewardsRequest

	if len(os.Args) > 1 {
		pathType := os.Args[1]
		rewardType, err := maxapi.ParseRewardType(pathType)
		if err != nil {
			log.Fatal(err)
		}

		req = api.RewardService.NewRewardsByTypeRequest(rewardType)
	} else {
		req = api.RewardService.NewRewardsRequest()
	}

	// req.From(1613931192)
	// req.From(1613240048)
	// req.From(maxapi.TimestampSince)
	// req.To(maxapi.TimestampSince + 3600 * 24)
	req.Limit(100)

	rewards, err := req.Do(ctx)
	if err != nil {
		log.Fatal(err)
	}

	for _, reward := range rewards {
		log.Printf("%+v\n", reward)
	}
}
