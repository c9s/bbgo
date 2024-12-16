package slacknotifier

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"golang.org/x/time/rate"

	"github.com/c9s/bbgo/pkg/dynamic"
	"github.com/c9s/bbgo/pkg/livenote"
	"github.com/c9s/bbgo/pkg/types"

	log "github.com/sirupsen/logrus"
	"github.com/slack-go/slack"
)

var limiter = rate.NewLimiter(rate.Every(600*time.Millisecond), 3)

// userIdRegExp matches strings like <@U012AB3CD>
var userIdRegExp = regexp.MustCompile(`^<@(.+?)>$`)

// groupIdRegExp matches strings like <!subteam^ID>
var groupIdRegExp = regexp.MustCompile(`^<!subteam\^(.+?)>$`)

var emailRegExp = regexp.MustCompile("`^(?P<name>[a-zA-Z0-9.!#$%&'*+/=?^_ \\x60{|}~-]+)@(?P<domain>[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*)$`gm")

var typeNamePrefixRE = regexp.MustCompile(`^\*?([a-zA-Z0-9_]+\.)?`)

type SlackAttachmentCreator interface {
	SlackAttachment() slack.Attachment
}

type SlackBlocksCreator interface {
	SlackBlocks() slack.Blocks
}

type notifyTask struct {
	channel string

	isUpdate bool

	// messageTs is the message timestamp, this is required when isUpdate is true
	messageTs string

	// threadTs is the thread timestamp
	threadTs string

	opts []slack.MsgOption
}

func (t *notifyTask) addMsgOption(opts ...slack.MsgOption) {
	t.opts = append(t.opts, opts...)
}

// Notifier is a slack notifier
//
// To use this notifier, you need to setup the slack app permissions:
// - channels:read
// - chat:write
//
// When using "pins", you will need permission: "pins:write"
type Notifier struct {
	ctx    context.Context
	cancel context.CancelFunc

	client  *slack.Client
	channel string

	taskC chan notifyTask

	liveNotePool *livenote.Pool

	userIdCache  map[string]*slack.User
	groupIdCache map[string]slack.UserGroup
}

type NotifyOption func(notifier *Notifier)

const defaultQueueSize = 500

func OptionContext(baseCtx context.Context) NotifyOption {
	return func(notifier *Notifier) {
		ctx, cancel := context.WithCancel(baseCtx)
		notifier.ctx = ctx
		notifier.cancel = cancel
	}
}

func OptionQueueSize(size int) NotifyOption {
	return NotifyOption(func(notifier *Notifier) {
		notifier.taskC = make(chan notifyTask, size)
	})
}

func New(client *slack.Client, channel string, options ...NotifyOption) *Notifier {
	notifier := &Notifier{
		ctx:          context.Background(),
		cancel:       func() {},
		channel:      channel,
		client:       client,
		taskC:        make(chan notifyTask, defaultQueueSize),
		liveNotePool: livenote.NewPool(100),
		userIdCache:  make(map[string]*slack.User, 30),
		groupIdCache: make(map[string]slack.UserGroup, 50),
	}

	for _, o := range options {
		o(notifier)
	}

	userGroups, err := client.GetUserGroupsContext(notifier.ctx)
	if err != nil {
		log.WithError(err).Error("failed to get the slack user groups")
	} else {
		for _, group := range userGroups {
			notifier.groupIdCache[group.Name] = group
		}

		// user groups: map[
		//   Development Team:{
		//      ID:S08004CQYQK
		//      TeamID:T036FASR3
		//      IsUserGroup:true
		//      Name:Development Team
		//      Description:dev
		//      Handle:dev
		//      IsExternal:false
		//      DateCreate:"Fri Nov  8"
		//      DateUpdate:"Fri Nov  8" DateDelete:"Thu Jan  1"
		//      AutoType: CreatedBy:U036FASR5 UpdatedBy:U12345678 DeletedBy:
		//      Prefs:{Channels:[] Groups:[]} UserCount:1 Users:[]}]
		log.Debugf("slack user groups: %+v", notifier.groupIdCache)
	}

	go notifier.worker(notifier.ctx)

	return notifier
}

func (n *Notifier) worker(ctx context.Context) {
	defer n.cancel()

	for {
		select {
		case <-ctx.Done():
			return

		case task := <-n.taskC:
			if err := n.executeTask(ctx, task); err != nil {
				log.WithError(err).
					WithField("channel", task.channel).
					Errorf("slack api error: %s", err.Error())
			}
		}
	}
}

func (n *Notifier) executeTask(ctx context.Context, task notifyTask) error {
	// ignore the wait error
	if err := limiter.Wait(ctx); err != nil {
		log.WithError(err).Warnf("slack rate limiter error")
	}

	if task.threadTs != "" {
		task.addMsgOption(slack.MsgOptionTS(task.threadTs))
	}

	if task.isUpdate && task.messageTs != "" {
		task.addMsgOption(slack.MsgOptionUpdate(task.messageTs))
	}

	_, _, err := n.client.PostMessageContext(ctx, task.channel, task.opts...)
	if err != nil {
		return err
	}

	return nil
}

func (n *Notifier) translateHandles(ctx context.Context, handles []string) ([]string, error) {
	var tags []string
	for _, handle := range handles {
		tag, err := n.translateHandle(ctx, handle)
		if err != nil {
			return nil, err
		}

		tags = append(tags, tag)
	}

	return tags, nil
}

func (n *Notifier) translateHandle(ctx context.Context, handle string) (string, error) {
	if handle == "" {
		return "", errors.New("handle is empty")
	}

	// trim the prefix '@' if we get a string like '@username'
	if handle[0] == '@' {
		handle = handle[1:]
	}

	// if the given handle is already in slack user id format, we don't need to look up
	if userIdRegExp.MatchString(handle) || groupIdRegExp.MatchString(handle) {
		return handle, nil
	}

	if user, exists := n.userIdCache[handle]; exists {
		return toUserHandle(user.ID), nil
	}

	if group, exists := n.groupIdCache[handle]; exists {
		return toSubteamHandle(group.ID), nil
	}

	var slackUser *slack.User
	var err error
	if emailRegExp.MatchString(handle) {
		slackUser, err = n.client.GetUserByEmailContext(ctx, handle)
		if err != nil {
			return "", fmt.Errorf("user %s not found: %v", handle, err)
		}
	} else {
		slackUser, err = n.client.GetUserInfoContext(ctx, handle)
		if err != nil {
			return "", fmt.Errorf("user handle %s not found: %v", handle, err)
		}
	}

	if slackUser != nil {
		n.userIdCache[handle] = slackUser
		return toUserHandle(slackUser.ID), nil
	}

	return "", fmt.Errorf("handle %s not found", handle)
}

func (n *Notifier) PostLiveNote(obj livenote.Object, opts ...livenote.Option) error {
	var slackOpts []slack.MsgOption

	var firstTimeHandles []string
	var commentHandles []string
	var comments []string
	var shouldCompare bool
	var shouldPin bool
	var ttl time.Duration = 0

	// load the default channel
	channel := n.channel

	for _, opt := range opts {
		switch val := opt.(type) {
		case *livenote.OptionOneTimeMention:
			firstTimeHandles = append(firstTimeHandles, val.Users...)
		case *livenote.OptionComment:
			comments = append(comments, val.Text)
			commentHandles = append(commentHandles, val.Users...)
		case *livenote.OptionCompare:
			shouldCompare = val.Value
		case *livenote.OptionPin:
			shouldPin = val.Value
		case *livenote.OptionTimeToLive:
			ttl = val.Duration
		case *livenote.OptionChannel:
			if val.Channel != "" {
				channel = val.Channel
			}
		}
	}

	var ctx = n.ctx
	var curObj, prevObj any
	if shouldCompare {
		if prevNote := n.liveNotePool.Get(obj); prevNote != nil {
			prevObj = prevNote.Object
		}
	}

	note := n.liveNotePool.Update(obj)
	curObj = note.Object

	if ttl > 0 {
		note.SetTimeToLive(ttl)
	}

	if shouldCompare && prevObj != nil {
		diffs, err := dynamic.Compare(curObj, prevObj)
		if err != nil {
			log.WithError(err).Warnf("unable to compare objects: %T and %T", curObj, prevObj)
		} else {
			if comment := diffsToComment(curObj, diffs); len(comment) > 0 {
				comments = append(comments, comment)
			}
		}
	}

	if note.ChannelID != "" {
		channel = note.ChannelID
	}

	var attachment slack.Attachment
	if creator, ok := note.Object.(SlackAttachmentCreator); ok {
		attachment = creator.SlackAttachment()
	} else {
		return fmt.Errorf("livenote object does not support types.SlackAttachmentCreator interface")
	}

	slackOpts = append(slackOpts, slack.MsgOptionAttachments(attachment))

	firstTimeTags, err := n.translateHandles(n.ctx, firstTimeHandles)
	if err != nil {
		return err
	}

	commentTags, err := n.translateHandles(n.ctx, commentHandles)
	if err != nil {
		return err
	}

	if note.MessageID != "" {
		// If compare is enabled, we need to attach the comments

		// UpdateMessageContext returns channel, timestamp, text, err
		respCh, respTs, _, err := n.client.UpdateMessageContext(ctx, note.ChannelID, note.MessageID, slackOpts...)
		if err != nil {
			return err
		}

		if len(comments) > 0 {
			var text string
			if len(commentTags) > 0 {
				text = joinTags(commentTags) + " "
			}

			text += joinComments(comments)
			n.queueTask(context.Background(), notifyTask{
				channel:  respCh,
				threadTs: respTs,
				opts: []slack.MsgOption{
					slack.MsgOptionText(text, false),
					slack.MsgOptionTS(respTs),
				},
			}, 100*time.Millisecond)
		}

	} else {
		respCh, respTs, err := n.client.PostMessageContext(ctx, channel, slackOpts...)
		if err != nil {
			log.WithError(err).
				WithField("channel", n.channel).
				Errorf("slack api error: %s", err.Error())
			return err
		}

		note.SetChannelID(respCh)
		note.SetMessageID(respTs)
		note.SetPostedTime(time.Now())

		if shouldPin {
			note.SetPin(true)

			if err := n.client.AddPinContext(ctx, respCh, slack.ItemRef{
				Channel:   respCh,
				Timestamp: respTs,
			}); err != nil {
				log.WithError(err).Warnf("unable to pin the slack message: %s", respTs)
			}
		}

		if len(firstTimeTags) > 0 {
			n.queueTask(n.ctx, notifyTask{
				channel:  respCh,
				threadTs: respTs,
				opts: []slack.MsgOption{
					slack.MsgOptionText(joinTags(firstTimeTags), false),
					slack.MsgOptionTS(respTs),
				},
			}, 100*time.Millisecond)
		}

		if len(comments) > 0 {
			var text string
			if len(commentTags) > 0 {
				text = joinTags(commentTags) + " "
			}

			text += joinComments(comments)
			n.queueTask(n.ctx, notifyTask{
				channel:  respCh,
				threadTs: respTs,
				opts: []slack.MsgOption{
					slack.MsgOptionText(text, false),
					slack.MsgOptionTS(respTs),
				},
			}, 100*time.Millisecond)
		}
	}

	return nil
}

func (n *Notifier) Notify(obj interface{}, args ...interface{}) {
	// TODO: filter args for the channel option
	n.NotifyTo(n.channel, obj, args...)
}

func filterSlackAttachments(args []interface{}) (slackAttachments []slack.Attachment, pureArgs []interface{}) {
	var firstAttachmentOffset = -1
	for idx, arg := range args {
		switch a := arg.(type) {

		// concrete type assert first
		case slack.Attachment:
			if firstAttachmentOffset == -1 {
				firstAttachmentOffset = idx
			}

			slackAttachments = append(slackAttachments, a)

		case *slack.Attachment:
			if firstAttachmentOffset == -1 {
				firstAttachmentOffset = idx
			}

			slackAttachments = append(slackAttachments, *a)

		case SlackAttachmentCreator:
			if firstAttachmentOffset == -1 {
				firstAttachmentOffset = idx
			}

			slackAttachments = append(slackAttachments, a.SlackAttachment())

		case types.PlainText:
			if firstAttachmentOffset == -1 {
				firstAttachmentOffset = idx
			}

			// fallback to PlainText if it's not supported
			// convert plain text to slack attachment
			text := a.PlainText()
			slackAttachments = append(slackAttachments, slack.Attachment{
				Title: text,
			})
		}
	}

	pureArgs = args
	if firstAttachmentOffset > -1 {
		pureArgs = args[:firstAttachmentOffset]
	}

	return slackAttachments, pureArgs
}

func (n *Notifier) NotifyTo(channel string, obj interface{}, args ...interface{}) {
	if len(channel) == 0 {
		channel = n.channel
	}

	slackAttachments, pureArgs := filterSlackAttachments(args)

	var opts []slack.MsgOption

	switch a := obj.(type) {
	case string:
		opts = append(opts, slack.MsgOptionText(fmt.Sprintf(a, pureArgs...), true),
			slack.MsgOptionAttachments(slackAttachments...))

	case slack.Attachment:
		opts = append(opts, slack.MsgOptionAttachments(append([]slack.Attachment{a}, slackAttachments...)...))

	case *slack.Attachment:
		opts = append(opts, slack.MsgOptionAttachments(append([]slack.Attachment{*a}, slackAttachments...)...))

	case SlackAttachmentCreator:
		// convert object to slack attachment (if supported)
		opts = append(opts, slack.MsgOptionAttachments(append([]slack.Attachment{a.SlackAttachment()}, slackAttachments...)...))

	default:
		log.Errorf("slack message conversion error, unsupported object: %T %+v", a, a)

	}

	n.queueTask(context.Background(), notifyTask{
		channel: channel,
		opts:    opts,
	}, 100*time.Millisecond)
}

func (n *Notifier) queueTask(ctx context.Context, task notifyTask, timeout time.Duration) {
	select {
	case <-ctx.Done():
		return
	case <-time.After(timeout):
		log.Warnf("slack notify task is dropped due to timeout %s", timeout)
		return
	case n.taskC <- task:
	}
}

func (n *Notifier) SendPhoto(buffer *bytes.Buffer) {
	n.SendPhotoTo(n.channel, buffer)
}

func (n *Notifier) SendPhotoTo(channel string, buffer *bytes.Buffer) {
	// TODO
}

func toUserHandle(id string) string {
	return fmt.Sprintf("<@%s>", id)
}

func toSubteamHandle(id string) string {
	return fmt.Sprintf("<!subteam^%s>", id)
}

func joinTags(tags []string) string {
	return strings.Join(tags, " ")
}

func joinComments(comments []string) string {
	return strings.Join(comments, "\n")
}

func diffsToComment(obj any, diffs []dynamic.Diff) (text string) {
	if len(diffs) == 0 {
		return text
	}

	text += fmt.Sprintf("_%s_ updated\n", objectName(obj))

	for _, diff := range diffs {
		text += fmt.Sprintf("- %s: `%s` transited to `%s`\n", diff.Field, diff.Before, diff.After)
	}

	return text
}

func objectName(obj any) string {
	type labelInf interface {
		Label() string
	}

	if ll, ok := obj.(labelInf); ok {
		return ll.Label()
	}

	typeName := fmt.Sprintf("%T", obj)
	typeName = typeNamePrefixRE.ReplaceAllString(typeName, "")
	return typeName
}
