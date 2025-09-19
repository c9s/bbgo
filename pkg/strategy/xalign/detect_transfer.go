package xalign

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/slack-go/slack"
)

// canAlign checks if the strategy can align the balances by checking for active transfers.
// It returns a map of currencies as keys and their corresponding active transfers.
// If the asset is not involved in any active transfer, it will not be included in the map.
func (s *Strategy) canAlign(ctx context.Context, sessions map[string]*bbgo.ExchangeSession) (map[string]*AssetTransfer, error) {
	if s.SkipTransferCheck {
		return nil, nil
	}

	activeTransfers := make(map[string]*AssetTransfer)
	pendingWithdraws, err := s.detectActiveWithdraw(ctx, sessions)
	if err != nil {
		return nil, fmt.Errorf("unable to check active transfers (withdraw): %w", err)
	}

	for _, pendingWithdraw := range pendingWithdraws {
		log.Warnf("found active transfer (%f %s withdraw)",
			pendingWithdraw.Amount.Float64(),
			pendingWithdraw.Asset)
		s.resetFaultBalanceRecords(pendingWithdraw.Asset)
		if at, ok := activeTransfers[pendingWithdraw.Asset]; ok {
			at.Withdraws = append(at.Withdraws, pendingWithdraw)
		} else {
			activeTransfers[pendingWithdraw.Asset] = &AssetTransfer{
				Withdraws: []types.Withdraw{pendingWithdraw},
			}
		}
	}

	pendingDeposits, err := s.detectActiveDeposit(ctx, sessions)
	if err != nil {
		return nil, fmt.Errorf("unable to check active transfers (deposit): %w", err)
	}

	for _, pendingDeposit := range pendingDeposits {
		log.Warnf("found active transfer (%f %s deposit)",
			pendingDeposit.Amount.Float64(),
			pendingDeposit.Asset)

		s.resetFaultBalanceRecords(pendingDeposit.Asset)
		if at, ok := activeTransfers[pendingDeposit.Asset]; ok {
			at.Deposits = append(at.Deposits, pendingDeposit)
		} else {
			activeTransfers[pendingDeposit.Asset] = &AssetTransfer{
				Deposits: []types.Deposit{pendingDeposit},
			}
		}
	}

	return activeTransfers, nil
}

func (s *Strategy) detectActiveWithdraw(
	ctx context.Context,
	sessions map[string]*bbgo.ExchangeSession,
) ([]types.Withdraw, error) {
	var err2 error
	var activeWithdraws []types.Withdraw
	until := time.Now()
	since := until.Add(-time.Hour * 24)
	for _, session := range sessions {
		transferService, ok := session.Exchange.(types.WithdrawHistoryService)
		if !ok {
			continue
		}

		withdraws, err := transferService.QueryWithdrawHistory(ctx, "", since, until)
		if err != nil {
			log.WithError(err).Error("unable to query withdraw history")
			err2 = err
			continue
		}
		for _, withdraw := range withdraws {
			log.Infof("checking withdraw status: %s", withdraw.String())
			switch withdraw.Status {
			case types.WithdrawStatusSent, types.WithdrawStatusProcessing, types.WithdrawStatusAwaitingApproval:
				activeWithdraws = append(activeWithdraws, withdraw)
			}
		}
	}

	return activeWithdraws, err2
}

func (s *Strategy) detectActiveDeposit(
	ctx context.Context,
	sessions map[string]*bbgo.ExchangeSession,
) ([]types.Deposit, error) {
	var err2 error
	var activeDeposits []types.Deposit
	until := time.Now()
	since := until.Add(-time.Hour * 24)
	for _, session := range sessions {
		transferService, ok := session.Exchange.(types.DepositHistoryService)
		if !ok {
			continue
		}

		deposits, err := transferService.QueryDepositHistory(ctx, "", since, until)
		if err != nil {
			log.WithError(err).Error("unable to query deposit history")
			err2 = err
			continue
		}

		for _, deposit := range deposits {
			log.Infof("checking deposit status: %s", deposit.String())
			switch deposit.Status {
			case types.DepositPending:
				activeDeposits = append(activeDeposits, deposit)
			}
		}
	}

	return activeDeposits, err2
}

type AssetTransfer struct {
	Withdraws []types.Withdraw
	Deposits  []types.Deposit
}

func (at *AssetTransfer) SlackAttachment() slack.Attachment {
	var fields []slack.AttachmentField
	var depositStrs, withdrawStrs []string

	for _, withdraw := range at.Withdraws {
		withdrawStrs = append(withdrawStrs, withdraw.String())
	}
	if len(withdrawStrs) > 0 {
		fields = append(fields, slack.AttachmentField{
			Title: "Withdrawals",
			Value: strings.Join(withdrawStrs, "\n"),
		})
	}

	for _, deposit := range at.Deposits {
		depositStrs = append(depositStrs, deposit.String())
	}

	if len(depositStrs) > 0 {
		fields = append(fields, slack.AttachmentField{
			Title: "Deposits",
			Value: strings.Join(depositStrs, "\n"),
		})
	}

	return slack.Attachment{
		Title:  "Active Asset Transfer",
		Text:   fmt.Sprintf("Found %d active transfers", len(at.Withdraws)+len(at.Deposits)),
		Fields: fields,
	}
}
