package autoborrow

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/accounting/margininterest"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/livenote"
	"github.com/c9s/bbgo/pkg/slack/slackalert"
	. "github.com/c9s/bbgo/pkg/testing/testhelper"
	"github.com/c9s/bbgo/pkg/types"
)

type budgetComment struct {
	alertID string
	message string
}

// fakeBudgetNotifier records what the alerter posts
type fakeBudgetNotifier struct {
	updates  []*InterestBudgetAlert
	comments []budgetComment
}

func (f *fakeBudgetNotifier) Update(alert *InterestBudgetAlert) {
	f.updates = append(f.updates, alert)
}

func (f *fakeBudgetNotifier) Comment(alert *InterestBudgetAlert, message string) {
	f.comments = append(f.comments, budgetComment{alert.AlertID, message})
}

func (f *fakeBudgetNotifier) messages() []string {
	out := make([]string, 0, len(f.comments))
	for _, c := range f.comments {
		out = append(out, c.message)
	}
	return out
}

// withBudgetAlerter gives the worker an alerter that posts to the returned fake.
// Its alert ids are alert-1, alert-2, ...
func withBudgetAlerter(w *interestEstimateWorker) *fakeBudgetNotifier {
	notifier := &fakeBudgetNotifier{}
	seq := 0
	w.budgetAlerter = &interestBudgetAlerter{
		session:  w.session,
		notifier: notifier,
		newID: func() string {
			seq++
			return fmt.Sprintf("alert-%d", seq)
		},
	}
	w.budgetAlerter.reset()
	return notifier
}

func ethRow(day int, interest string) types.MarginInterestValuation {
	return valuedRow("ETH", interest, "3000", time.Date(2026, 11, day, 0, 0, 0, 0, time.UTC))
}

func TestInterestBudgetAlert_NothingIsPostedWithinTheBudget(t *testing.T) {
	w := newAssetBudgetWorker(t, &fakeHistoryStore{}, map[string]string{"USDT": "600", "ETH": "20"})
	notifier := withBudgetAlerter(w)

	w.tick(context.Background())
	w.tick(context.Background())

	assert.Empty(t, notifier.updates)
	assert.Empty(t, notifier.comments)
}

// The alert opens when a budget goes over, follows every tick, and comments a status change once
func TestInterestBudgetAlert_OpensAndFollowsTheStatus(t *testing.T) {
	// ETH: 10 borrowed projects 11.19 USDT to the month end
	history := &fakeHistoryStore{rows: []types.MarginInterestValuation{ethRow(2, "0.001")}}
	w := newAssetBudgetWorker(t, history, map[string]string{"USDT": "600", "ETH": "20"})
	notifier := withBudgetAlerter(w)

	// 3 realized + 11.19 = 14.19 of 20: within
	w.tick(context.Background())
	require.Empty(t, notifier.updates)

	// 12 realized + 11.19 = 23.19: projected to go over
	history.rows = append(history.rows, ethRow(3, "0.003"))
	w.tick(context.Background())

	require.Len(t, notifier.updates, 1)
	alert := notifier.updates[0]
	assert.Equal(t, "alert-1", alert.AlertID)
	assert.Equal(t, w.session.Name, alert.SessionName)
	assert.Equal(t, time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), alert.Month)
	require.Len(t, notifier.comments, 1)
	assert.Contains(t, notifier.comments[0].message, "⚠️")
	assert.Contains(t, notifier.comments[0].message, w.session.Name)
	assert.Contains(t, notifier.comments[0].message, "ETH budget is projected_exceeded")
	// USDT is within its budget: it is not named as over
	assert.NotContains(t, notifier.comments[0].message, "USDT budget is")

	// the same status again: the live note follows, nothing new is said
	w.tick(context.Background())
	assert.Len(t, notifier.updates, 2)
	assert.Len(t, notifier.comments, 1)

	// 21 realized: over for good
	history.rows = append(history.rows, ethRow(4, "0.003"))
	w.tick(context.Background())
	assert.Len(t, notifier.updates, 3)
	require.Len(t, notifier.comments, 2)
	assert.Equal(t, "ETH budget is exceeded (was projected_exceeded)", notifier.comments[1].message)

	// every post is on the same live note
	for _, u := range notifier.updates {
		assert.Equal(t, "alert-1", u.AlertID)
	}
	for _, c := range notifier.comments {
		assert.Equal(t, "alert-1", c.alertID)
	}
}

// The alert lists every budget of the session: the session first, then the assets
func TestInterestBudgetAlert_ListsEveryBudget(t *testing.T) {
	history := &fakeHistoryStore{rows: []types.MarginInterestValuation{ethRow(2, "0.004")}}
	// ETH: 12 + 11.19 = 23.19 of 20; session: 23.19 + 0.373 = 23.563 of 100
	w := newAssetBudgetWorker(t, history, map[string]string{"USDT": "600", "ETH": "20"})
	w.config.Budget.MonthlyUSDT = Number(100)
	notifier := withBudgetAlerter(w)

	w.tick(context.Background())

	require.Len(t, notifier.updates, 1)
	alert := notifier.updates[0]
	require.Len(t, alert.Budgets, 3)
	assert.Equal(t, "Session budget", alert.Budgets[0].Subject)
	assert.Equal(t, "ETH budget", alert.Budgets[1].Subject)
	assert.Equal(t, "USDT budget", alert.Budgets[2].Subject)
	assert.Equal(t, margininterest.BudgetStatusOK, alert.Budgets[0].Budget.Status())
	assert.Equal(t, margininterest.BudgetStatusProjectedExceeded, alert.Budgets[1].Budget.Status())

	attachment := alert.SlackAttachment()
	assert.Equal(t, "Interest Budget Alert: "+w.session.Name+" session", attachment.Title)
	assert.Contains(t, attachment.Text, "1 of 3 interest budgets are over (2026-11)")
	assert.Equal(t, "warning", attachment.Color)

	values := map[string]string{}
	for _, f := range attachment.Fields {
		values[f.Title] = f.Value
	}
	assert.Equal(t, w.session.Name, values["Session"])
	assert.Equal(t, "ok: 23.56 / 100.00 USDT, realized 12.00", values["Session budget"])
	assert.Equal(t, "projected_exceeded: 23.19 / 20.00 USDT, realized 12.00", values["ETH budget"])
	assert.Equal(t, "ok: 0.37 / 600.00 USDT, realized 0.00", values["USDT budget"])
	assert.Equal(t, "💸 1 of 3 interest budgets are over (2026-11)\nUpdated at 2026-11-15 10:20:00 UTC", attachment.Text)
}

// The message is edited in place and keeps the time of its first post, so it
// says when its figures were last updated: the time of each tick
func TestInterestBudgetAlert_SaysWhenItWasUpdated(t *testing.T) {
	// 0.373 USDT is projected
	w := newBudgetWorker(t, "0.2", &fakeHistoryStore{})
	notifier := withBudgetAlerter(w)
	now := testNow
	w.now = func() time.Time { return now }

	w.tick(context.Background())
	now = testNow.Add(5 * time.Minute)
	w.tick(context.Background())

	require.Len(t, notifier.updates, 2)
	assert.Equal(t, testNow, notifier.updates[0].UpdatedAt)
	assert.Equal(t, now, notifier.updates[1].UpdatedAt)
	assert.Contains(t, notifier.updates[0].SlackAttachment().Text, "\nUpdated at 2026-11-15 10:20:00 UTC")
	assert.Contains(t, notifier.updates[1].SlackAttachment().Text, "\nUpdated at 2026-11-15 10:25:00 UTC")

	// the update is not a status change: nothing more is said in the thread
	require.Len(t, notifier.comments, 1)
}

// The notifier comments every changed field of a compared object in the
// thread, and the update time changes on every tick: the alert is not compared
func TestInterestBudgetAlert_UpdateDoesNotCompareTheObject(t *testing.T) {
	n := &liveNoteBudgetNotifier{slack: &slackalert.SlackAlert{Channel: "C1", Mentions: []string{"<@U1>"}, Pin: true}}

	opts := n.updateOptions()

	require.Len(t, opts, 3)
	for _, opt := range opts {
		_, compares := opt.(*livenote.OptionCompare)
		assert.False(t, compares)
	}
	assert.Empty(t, (&liveNoteBudgetNotifier{}).updateOptions())
}

// An alert without an update time does not claim one
func TestInterestBudgetAlert_TextWithoutUpdateTime(t *testing.T) {
	alert := &InterestBudgetAlert{SessionName: "s", Month: testNow}
	assert.NotContains(t, alert.SlackAttachment().Text, "Updated at")
}

func TestInterestBudgetAlert_AttachmentColor(t *testing.T) {
	line := func(subject, realized, month string) interestBudgetLine {
		return interestBudgetLine{
			Subject: subject,
			Budget:  margininterest.EvaluateBudget(Number(100), Number(realized), Number(month), true),
		}
	}

	within := &InterestBudgetAlert{SessionName: "s", Month: testNow, Budgets: []interestBudgetLine{line("ETH", "10", "50")}}
	assert.Equal(t, "good", within.SlackAttachment().Color)
	assert.Contains(t, within.SlackAttachment().Text, "✅")

	projected := &InterestBudgetAlert{SessionName: "s", Month: testNow, Budgets: []interestBudgetLine{
		line("ETH", "10", "50"), line("USDT", "10", "150"),
	}}
	assert.Equal(t, "warning", projected.SlackAttachment().Color)

	exceeded := &InterestBudgetAlert{SessionName: "s", Month: testNow, Budgets: []interestBudgetLine{
		line("ETH", "10", "150"), line("USDT", "110", "150"),
	}}
	assert.Equal(t, "danger", exceeded.SlackAttachment().Color)
	assert.Contains(t, exceeded.SlackAttachment().Text, "2 of 2")
}

// When nothing is over any more the alert is resolved, and the next one is a new message
func TestInterestBudgetAlert_ResolvesAndStartsANewAlert(t *testing.T) {
	account := newTestAccount(Number(10), debtBalance("USDT", fixedpoint.Zero, Number(1000)))
	rates := &fakeRateService{rates: map[string]string{"USDT": "0.000001"}}
	w := newTestInterestWorker(t, account, rates, &fakeHistoryStore{}, configured("USDT")...)
	// 0.373 USDT is projected
	w.config.Budget = &InterestBudgetConfig{MonthlyUSDT: Number("0.2")}
	notifier := withBudgetAlerter(w)

	w.tick(context.Background())
	require.Len(t, notifier.updates, 1)
	require.Len(t, notifier.comments, 1)
	assert.Contains(t, notifier.comments[0].message, "Session budget is projected_exceeded")

	// the rate drops: 0.0373 is projected, within the budget again
	rates.rates["USDT"] = "0.0000001"
	w.tick(context.Background())

	// the live note is updated to its final state, then resolved
	require.Len(t, notifier.updates, 2)
	assert.Equal(t, margininterest.BudgetStatusOK, notifier.updates[1].Budgets[0].Budget.Status())
	assert.Equal(t, "good", notifier.updates[1].SlackAttachment().Color)
	require.Len(t, notifier.comments, 3)
	assert.Equal(t, "Session budget is ok (was projected_exceeded)", notifier.comments[1].message)
	assert.Contains(t, notifier.comments[2].message, "✅")
	assert.Equal(t, "alert-1", notifier.comments[2].alertID)

	// within the budget: nothing more is posted
	w.tick(context.Background())
	assert.Len(t, notifier.updates, 2)
	assert.Len(t, notifier.comments, 3)

	// over again: a new alert, not an update of the resolved one
	rates.rates["USDT"] = "0.000001"
	w.tick(context.Background())
	require.Len(t, notifier.updates, 3)
	assert.Equal(t, "alert-2", notifier.updates[2].AlertID)
	require.Len(t, notifier.comments, 4)
	assert.Contains(t, notifier.comments[3].message, "⚠️")
	assert.Equal(t, "alert-2", notifier.comments[3].alertID)
}

// A failed query makes the realized interest disappear and the status unknown.
// That is not the budget being fine again, so the alert stays open.
func TestInterestBudgetAlert_UnknownDoesNotResolveAnOpenAlert(t *testing.T) {
	history := &fakeHistoryStore{rows: []types.MarginInterestValuation{
		usdtRow(time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC), "5"),
	}}
	w := newBudgetWorker(t, "1", history)
	notifier := withBudgetAlerter(w)

	w.tick(context.Background())
	require.Len(t, notifier.comments, 1)
	assert.Contains(t, notifier.comments[0].message, "Session budget is exceeded")

	history.queryErr = errors.New("db down")
	est := w.tick(context.Background())
	require.Equal(t, margininterest.BudgetStatusUnknown, est.Budget.Status())
	require.Len(t, notifier.comments, 2)
	assert.Equal(t, "Session budget is unknown (was exceeded)", notifier.comments[1].message)

	history.queryErr = nil
	w.tick(context.Background())
	require.Len(t, notifier.comments, 3)
	assert.Equal(t, "Session budget is exceeded (was unknown)", notifier.comments[2].message)

	// never resolved, and still the same alert
	for _, m := range notifier.messages() {
		assert.NotContains(t, m, "✅")
	}
	assert.Equal(t, "alert-1", notifier.updates[len(notifier.updates)-1].AlertID)
}

// An incomplete estimate alone does not raise the alert
func TestInterestBudgetAlert_UnknownDoesNotOpenAnAlert(t *testing.T) {
	history := &fakeHistoryStore{queryErr: errors.New("db down")}
	w := newBudgetWorker(t, "10", history)
	notifier := withBudgetAlerter(w)

	est := w.tick(context.Background())

	assert.Equal(t, margininterest.BudgetStatusUnknown, est.Budget.Status())
	assert.Empty(t, notifier.updates)
	assert.Empty(t, notifier.comments)
}

// A new month is a new alert: the message of the past month keeps its last state
func TestInterestBudgetAlert_ANewMonthIsANewAlert(t *testing.T) {
	history := &fakeHistoryStore{rows: []types.MarginInterestValuation{
		usdtRow(time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC), "5"),
	}}
	// December projects 1000 x 743 x 0.000001 = 0.743
	w := newBudgetWorker(t, "0.5", history)
	notifier := withBudgetAlerter(w)

	now := time.Date(2026, 11, 30, 23, 59, 0, 0, time.UTC)
	w.now = func() time.Time { return now }

	w.tick(context.Background())
	require.Len(t, notifier.updates, 1)
	assert.Equal(t, "alert-1", notifier.updates[0].AlertID)
	assert.Equal(t, margininterest.BudgetStatusExceeded, notifier.updates[0].Budgets[0].Budget.Status())

	now = time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	w.tick(context.Background())

	// November is not resolved; December opens its own alert
	for _, m := range notifier.messages() {
		assert.NotContains(t, m, "✅")
	}
	require.Len(t, notifier.updates, 2)
	december := notifier.updates[1]
	assert.Equal(t, "alert-2", december.AlertID)
	assert.Equal(t, time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC), december.Month)
	assert.Equal(t, margininterest.BudgetStatusProjectedExceeded, december.Budgets[0].Budget.Status())
	require.Len(t, notifier.comments, 2)
	assert.Contains(t, notifier.comments[1].message, "⚠️")
	assert.Equal(t, "alert-2", notifier.comments[1].alertID)
}

// A month that starts within the budget posts nothing after an exceeded month
func TestInterestBudgetAlert_ANewMonthWithinTheBudgetIsQuiet(t *testing.T) {
	history := &fakeHistoryStore{rows: []types.MarginInterestValuation{
		usdtRow(time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC), "5"),
	}}
	w := newBudgetWorker(t, "1", history)
	notifier := withBudgetAlerter(w)

	now := time.Date(2026, 11, 30, 23, 59, 0, 0, time.UTC)
	w.now = func() time.Time { return now }
	w.tick(context.Background())
	require.Len(t, notifier.updates, 1)

	now = time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	w.tick(context.Background())

	assert.Len(t, notifier.updates, 1)
	assert.Len(t, notifier.comments, 1)
}

func TestInterestBudgetAlertConfig_Validate(t *testing.T) {
	interval := types.Duration(5 * time.Minute)
	alert := &InterestBudgetAlertConfig{Slack: &slackalert.SlackAlert{Channel: "c"}}

	// an alert without a budget is an error, not a silent no-op
	for name, s := range map[string]*Strategy{
		"no estimate":   {InterestBudgetAlert: alert},
		"no budget":     {InterestBudgetAlert: alert, InterestEstimate: &InterestEstimateConfig{Interval: interval}},
		"a zero budget": {InterestBudgetAlert: alert, InterestEstimate: &InterestEstimateConfig{Interval: interval, Budget: &InterestBudgetConfig{}}},
	} {
		err := s.Validate()
		require.Error(t, err, name)
		assert.True(t, strings.Contains(err.Error(), "interestBudgetAlert"), name)
	}

	withSession := &Strategy{
		InterestBudgetAlert: alert,
		InterestEstimate:    &InterestEstimateConfig{Interval: interval, Budget: &InterestBudgetConfig{MonthlyUSDT: Number(1000)}},
	}
	assert.NoError(t, withSession.Validate())

	withAsset := &Strategy{
		Assets:              configured("ETH"),
		InterestBudgetAlert: alert,
		InterestEstimate: &InterestEstimateConfig{Interval: interval, Budget: &InterestBudgetConfig{
			Assets: map[string]fixedpoint.Value{"ETH": Number(300)},
		}},
	}
	assert.NoError(t, withAsset.Validate())

	// a budget without an alert is fine: it is still reported
	assert.NoError(t, (&Strategy{InterestEstimate: withSession.InterestEstimate}).Validate())
}

// The worker has an alerter only when the alert is configured
func TestInterestBudgetAlert_WorkerHasAnAlerterOnlyWhenConfigured(t *testing.T) {
	build := func(alert *InterestBudgetAlertConfig) *interestEstimateWorker {
		account := newTestAccount(Number(10))
		s := newTestStrategy(t, account, newFakeMarginBorrowRepayService(nil), configured("USDT")...)
		s.InterestBudgetAlert = alert
		s.ExchangeSession.Name = fmt.Sprintf("test-%d", sessionSeq.Add(1))
		return newInterestEstimateWorker(s, s.ExchangeSession, &InterestEstimateConfig{
			Interval: types.Duration(5 * time.Minute),
			Budget:   &InterestBudgetConfig{MonthlyUSDT: Number(1000)},
		})
	}

	assert.Nil(t, build(nil).budgetAlerter)

	w := build(&InterestBudgetAlertConfig{Slack: &slackalert.SlackAlert{Channel: "c", Mentions: []string{"<@U1>"}}})
	require.NotNil(t, w.budgetAlerter)
	assert.NotEmpty(t, w.budgetAlerter.alertID)
	notifier, ok := w.budgetAlerter.notifier.(*liveNoteBudgetNotifier)
	require.True(t, ok)
	assert.Equal(t, "c", notifier.slack.Channel)
}
