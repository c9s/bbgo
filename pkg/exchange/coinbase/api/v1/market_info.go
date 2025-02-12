package coinbase

import "context"

func (client *RestAPIClient) GetMarketInfo(ctx context.Context) (MarketInfoResponse, error) {
	req := client.NewGetMarketInfoRequest()
	return req.Do(ctx)
}
