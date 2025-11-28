package v3

type RewardType string

const (
	RewardAirdrop    = RewardType("airdrop_reward")
	RewardCommission = RewardType("commission")
	RewardHolding    = RewardType("holding_reward")
	RewardMining     = RewardType("mining_reward")
	RewardTrading    = RewardType("trading_reward")
	RewardStaking    = RewardType("staking_reward")
	RewardRedemption = RewardType("redemption_reward")
	RewardVipRebate  = RewardType("vip_rebate")
	RewardYield      = RewardType("yield")
)
