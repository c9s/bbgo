package autoborrow

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/slack-go/slack"

	"github.com/c9s/bbgo/pkg/accounting/margininterest"
	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/livenote"
	"github.com/c9s/bbgo/pkg/slack/slackalert"
	"github.com/c9s/bbgo/pkg/types"
)

// InterestBudgetAlertConfig enables the alert on the interest budgets of the
// session (interestEstimate.budget). It needs a budget.
//
// The alert is one live message per session, updated in place. It is raised
// when a budget is exceeded or projected to be exceeded, and it lists every
// budget of the session with its status. Below its title it says when its
// figures were last updated. It only tells an operator: the alert repays
// nothing and does not change borrowing. Whether the budgets limit borrowing
// is set with interestEstimate.budget.limitBorrow, and the message says so.
//
//	interestBudgetAlert:
//	  slack:
//	    channel: "channel_id"
//	    mentions:
//	    - '<@USER_ID>'
type InterestBudgetAlertConfig struct {
	Slack *slackalert.SlackAlert `json:"slack,omitempty"`
}

// interestBudgetLine is one budget of the alert: the session budget or an asset budget
type interestBudgetLine struct {
	// Subject names the budget: "Session budget" or "<asset> budget"
	Subject string
	Budget  margininterest.BudgetEvaluation
}

// InterestBudgetAlert is the live note of the interest budgets of one session
// in one UTC calendar month
type InterestBudgetAlert struct {
	AlertID      string
	AccountLabel string

	SessionName string
	Exchange    types.ExchangeName

	// Month is the start of the UTC calendar month the budgets cover
	Month time.Time

	// UpdatedAt is when the tick the alert was built on started: the debts are
	// read then, while realized interest is as of the last sync. The message is
	// edited in place and keeps the time it was first posted, so without it a
	// reader cannot tell a message that is still updated from one that is not.
	UpdatedAt time.Time

	// Budgets lists the session budget first, then the asset budgets by asset
	Budgets []interestBudgetLine

	// LimitsBorrow says the budgets limit what autoborrow borrows
	// (budget.limitBorrow), so a reader knows why a borrow was not made
	LimitsBorrow bool
}

// interestBudgetAlertTimeLayout formats the update time of the alert, in UTC like its month
const interestBudgetAlertTimeLayout = "2006-01-02 15:04:05 UTC"

func (a *InterestBudgetAlert) ObjectID() string {
	return a.AlertID
}

// isOverBudget reports whether status raises the alert. unknown does not: an
// incomplete estimate is reported in the log and the metrics, and one failed
// sync should not call anyone.
func isOverBudget(status margininterest.BudgetStatus) bool {
	return status == margininterest.BudgetStatusExceeded || status == margininterest.BudgetStatusProjectedExceeded
}

// over returns the budgets that are exceeded or projected to be exceeded
func (a *InterestBudgetAlert) over() []interestBudgetLine {
	var out []interestBudgetLine
	for _, line := range a.Budgets {
		if isOverBudget(line.Budget.Status()) {
			out = append(out, line)
		}
	}
	return out
}

func (a *InterestBudgetAlert) SlackAttachment() slack.Attachment {
	var fields []slack.AttachmentField

	if len(a.AccountLabel) > 0 {
		fields = append(fields, slack.AttachmentField{Title: "Account", Value: a.AccountLabel})
	}

	if len(a.Exchange) > 0 {
		fields = append(fields, slack.AttachmentField{Title: "Exchange", Value: string(a.Exchange), Short: true})
	}

	fields = append(fields, slack.AttachmentField{Title: "Session", Value: a.SessionName, Short: true})

	month := a.Month.Format("2006-01")
	over := a.over()

	color := "good"
	text := fmt.Sprintf("✅ Every interest budget is within its budget (%s)", month)
	if len(over) > 0 {
		color = "warning"
		text = fmt.Sprintf("💸 %d of %d interest budgets are over (%s)", len(over), len(a.Budgets), month)
	}

	for _, line := range a.Budgets {
		b := line.Budget
		status := b.Status()
		if status == margininterest.BudgetStatusExceeded {
			color = "danger"
		}

		fields = append(fields, slack.AttachmentField{
			Title: line.Subject,
			Value: fmt.Sprintf("%s: %s / %s USDT, realized %s",
				status, b.MonthUSDT.FormatString(2), b.BudgetUSDT.FormatString(2), b.RealizedUSDT.FormatString(2)),
		})
	}

	// the update time is right below the title: every line of the message is from that tick
	if !a.UpdatedAt.IsZero() {
		text += "\nUpdated at " + a.UpdatedAt.UTC().Format(interestBudgetAlertTimeLayout)
	}

	if a.LimitsBorrow {
		text += "\nAutoborrow borrows only what these budgets can still pay for. Nothing is repaid."
	}

	return slack.Attachment{
		Color:      color,
		Title:      fmt.Sprintf("Interest Budget Alert: %s session", a.SessionName),
		Text:       text,
		Fields:     fields,
		Footer:     fmt.Sprintf("%s - month total (realized + projected) / budget", a.Exchange),
		FooterIcon: types.ExchangeFooterIcon(a.Exchange),
		MarkdownIn: []string{"text"},
	}
}

// interestBudgetNotifier posts the alert. The worker talks to it, not to
// Slack, so the alert logic is tested without a notification system.
type interestBudgetNotifier interface {
	// Update posts the live note of alert, or updates the one already posted for its ObjectID
	Update(alert *InterestBudgetAlert)

	// Comment adds a message to the thread of the live note of alert
	Comment(alert *InterestBudgetAlert, message string)
}

// liveNoteBudgetNotifier posts the alert as a live note, like the other autoborrow alerts
type liveNoteBudgetNotifier struct {
	slack *slackalert.SlackAlert
}

func (n *liveNoteBudgetNotifier) Update(alert *InterestBudgetAlert) {
	bbgo.PostLiveNote(alert, n.updateOptions()...)
}

// updateOptions returns the options the alert is posted and updated with.
// Unlike the other autoborrow alerts there is no livenote.CompareObject: it
// comments every changed field in the thread, and UpdatedAt changes on every
// tick. The alerter comments the status changes itself.
func (n *liveNoteBudgetNotifier) updateOptions() []livenote.Option {
	var opts []livenote.Option
	if n.slack != nil {
		opts = append(opts, livenote.Channel(n.slack.Channel), livenote.OneTimeMention(n.slack.Mentions...))
		if n.slack.Pin {
			opts = append(opts, livenote.Pin(true))
		}
	}
	return opts
}

func (n *liveNoteBudgetNotifier) Comment(alert *InterestBudgetAlert, message string) {
	opts := []livenote.Option{livenote.Comment(message)}
	if n.slack != nil {
		opts = append(opts, livenote.Channel(n.slack.Channel))
	}

	bbgo.PostLiveNote(alert, opts...)
}

// interestBudgetAlerter raises and resolves the interest budget alert of one
// session. Its state is in memory only: after a restart a session that is
// still over its budget is alerted again, in a new message.
type interestBudgetAlerter struct {
	session  *bbgo.ExchangeSession
	notifier interestBudgetNotifier
	newID    func() string

	alertID string

	// month is the UTC calendar month of the current alert
	month time.Time

	// open is true from the tick a budget goes over until every budget is settled again
	open bool

	// statuses holds the status of each budget on the last tick the alert was
	// open, by subject, to comment on a change once
	statuses map[string]margininterest.BudgetStatus

	// wasOver holds the budgets that went over while the alert is open and are
	// not ok again yet
	wasOver map[string]struct{}

	// limitsBorrow says the budgets limit what autoborrow borrows
	limitsBorrow bool
}

func newInterestBudgetAlerter(session *bbgo.ExchangeSession, config *InterestBudgetAlertConfig) *interestBudgetAlerter {
	a := &interestBudgetAlerter{
		session:  session,
		notifier: &liveNoteBudgetNotifier{slack: config.Slack},
		newID:    func() string { return uuid.New().String() },
	}
	a.reset()
	return a
}

// reset starts a new alert: the next one is a new message, not an update of the last
func (a *interestBudgetAlerter) reset() {
	a.alertID = a.newID()
	a.open = false
	a.statuses = make(map[string]margininterest.BudgetStatus)
	a.wasOver = make(map[string]struct{})
}

func (a *interestBudgetAlerter) build(est *interestEstimate) *InterestBudgetAlert {
	alert := &InterestBudgetAlert{
		AlertID:      a.alertID,
		AccountLabel: a.session.GetAccountLabel(),
		SessionName:  a.session.Name,
		Exchange:     a.session.ExchangeName,
		Month:        est.Month,
		UpdatedAt:    est.Time,
		LimitsBorrow: a.limitsBorrow,
	}

	if est.Budget != nil {
		alert.Budgets = append(alert.Budgets, interestBudgetLine{Subject: "Session budget", Budget: *est.Budget})
	}

	for _, asset := range sortedKeys(est.AssetBudgets) {
		alert.Budgets = append(alert.Budgets, interestBudgetLine{Subject: asset + " budget", Budget: est.AssetBudgets[asset]})
	}

	return alert
}

// update raises, updates or resolves the alert for the budgets of est.
//
// The alert opens when a budget is exceeded or projected to be exceeded. While
// it is open the live note follows every tick, and a status change of a budget
// is commented once. It is resolved when no budget is over any more.
//
// A budget that went over and then reads unknown keeps the alert open: when a
// query or a sync fails the realized interest is missing, and that is not the
// budget being fine again.
//
// A new month is a new alert. The message of the past month keeps its last state.
func (a *interestBudgetAlerter) update(est *interestEstimate) {
	if est.Budget == nil && len(est.AssetBudgets) == 0 {
		return
	}

	if !est.Month.Equal(a.month) {
		// the first month needs no new alert: nothing was posted yet
		if !a.month.IsZero() {
			a.reset()
		}
		a.month = est.Month
	}

	alert := a.build(est)
	over := alert.over()

	stillOpen := len(over) > 0
	for _, line := range alert.Budgets {
		status := line.Budget.Status()
		switch {
		case isOverBudget(status):
			a.wasOver[line.Subject] = struct{}{}
		case status == margininterest.BudgetStatusOK:
			delete(a.wasOver, line.Subject)
		default:
			if _, was := a.wasOver[line.Subject]; was {
				stillOpen = true
			}
		}
	}

	if !a.open && len(over) == 0 {
		return
	}

	a.notifier.Update(alert)

	if !a.open {
		a.notifier.Comment(alert, fmt.Sprintf(
			"⚠️ Session %s is over its margin interest budget: %s. Please repay the debt or review the strategy settings.",
			a.session.Name, describeBudgets(over)))
	} else {
		for _, line := range alert.Budgets {
			status := line.Budget.Status()
			if previous, seen := a.statuses[line.Subject]; seen && previous != status {
				a.notifier.Comment(alert, fmt.Sprintf("%s is %s (was %s)", line.Subject, status, previous))
			}
		}
	}

	for _, line := range alert.Budgets {
		a.statuses[line.Subject] = line.Budget.Status()
	}

	a.open = stillOpen
	if !a.open {
		a.notifier.Comment(alert, fmt.Sprintf("✅ The margin interest of session %s is within its budgets again", a.session.Name))
		a.reset()
	}
}

// describeBudgets lists lines as "ETH budget is exceeded, Session budget is projected_exceeded"
func describeBudgets(lines []interestBudgetLine) string {
	parts := make([]string, 0, len(lines))
	for _, line := range lines {
		parts = append(parts, fmt.Sprintf("%s is %s", line.Subject, line.Budget.Status()))
	}
	return strings.Join(parts, ", ")
}
