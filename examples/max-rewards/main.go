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

	var (
		rewards []maxapi.Reward
		err     error
	)

	if len(os.Args) > 1 {
		pathType := os.Args[1]
		rewardType, err1 := maxapi.ParseRewardType(pathType)
		if err1 != nil {
			log.Fatal(err1)
		}

		rewards, err = api.RewardService.NewGetRewardsOfTypeRequest(rewardType).Limit(100).Do(ctx)
	} else {
		rewards, err = api.RewardService.NewGetRewardsRequest().Limit(100).Do(ctx)
	}

	if err != nil {
		log.Fatal(err)
	}

	for _, reward := range rewards {
		log.Printf("%+v\n", reward)
	}
}
