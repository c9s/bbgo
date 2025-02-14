package coinbase

import "context"

func (client *RestAPIClient) GetBalances(ctx context.Context) (BalanceSnapshot, error) {
	req := GetBalancesRequest{client: client}
	return req.Do(ctx)
}
