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

	"github.com/c9s/bbgo/pkg/livenote"
	"github.com/c9s/bbgo/pkg/types"

	log "github.com/sirupsen/logrus"
	"github.com/slack-go/slack"
)

var limiter = rate.NewLimiter(rate.Every(1*time.Second), 3)

type notifyTask struct {
	Channel string
	Opts    []slack.MsgOption
}

type Notifier struct {
	client  *slack.Client
	channel string

	taskC chan notifyTask

	liveNotePool *livenote.Pool

	userIdCache  map[string]*slack.User
	groupIdCache map[string]slack.UserGroup
}

type NotifyOption func(notifier *Notifier)

func New(client *slack.Client, channel string, options ...NotifyOption) *Notifier {
	notifier := &Notifier{
		channel:      channel,
		client:       client,
		taskC:        make(chan notifyTask, 100),
		liveNotePool: livenote.NewPool(100),
		userIdCache:  make(map[string]*slack.User, 30),
		groupIdCache: make(map[string]slack.UserGroup, 50),
	}

	for _, o := range options {
		o(notifier)
	}

	userGroups, err := client.GetUserGroupsContext(context.Background())
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

	go notifier.worker()

	return notifier
}

func (n *Notifier) worker() {
	ctx := context.Background()
	for {
		select {
		case <-ctx.Done():
			return

		case task := <-n.taskC:
			// ignore the wait error
			_ = limiter.Wait(ctx)

			_, _, err := n.client.PostMessageContext(ctx, task.Channel, task.Opts...)
			if err != nil {
				log.WithError(err).
					WithField("channel", task.Channel).
					Errorf("slack api error: %s", err.Error())
			}
		}
	}
}

// userIdRegExp matches strings like <@U012AB3CD>
var userIdRegExp = regexp.MustCompile(`^<@(.+?)>$`)

// groupIdRegExp matches strings like <!subteam^ID>
var groupIdRegExp = regexp.MustCompile(`^<!subteam\^(.+?)>$`)

var emailRegExp = regexp.MustCompile("`^(?P<name>[a-zA-Z0-9.!#$%&'*+/=?^_ \\x60{|}~-]+)@(?P<domain>[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*)$`gm")

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
	note := n.liveNotePool.Update(obj)
	ctx := context.Background()

	channel := note.ChannelID
	if channel == "" {
		channel = n.channel
	}

	var attachment slack.Attachment
	if creator, ok := note.Object.(types.SlackAttachmentCreator); ok {
		attachment = creator.SlackAttachment()
	} else {
		return fmt.Errorf("livenote object does not support types.SlackAttachmentCreator interface")
	}

	var slackOpts []slack.MsgOption
	slackOpts = append(slackOpts, slack.MsgOptionAttachments(attachment))

	var firstTimeHandles []string
	var commentHandles []string
	var mentions []*livenote.OptionOneTimeMention
	var comments []*livenote.OptionComment
	for _, opt := range opts {
		switch val := opt.(type) {
		case *livenote.OptionOneTimeMention:
			mentions = append(mentions, val)
			firstTimeHandles = append(firstTimeHandles, val.Users...)
		case *livenote.OptionComment:
			comments = append(comments, val)
			commentHandles = append(commentHandles, val.Users...)
		}
	}

	firstTimeTags, err := n.translateHandles(context.Background(), firstTimeHandles)
	if err != nil {
		return err
	}

	commentTags, err := n.translateHandles(context.Background(), commentHandles)
	if err != nil {
		return err
	}

	// format: mention slack user
	// <@U012AB3CD>

	if note.MessageID != "" {
		// If compare is enabled, we need to attach the comments

		// UpdateMessageContext returns channel, timestamp, text, err
		_, _, _, err := n.client.UpdateMessageContext(ctx, channel, note.MessageID, slackOpts...)
		if err != nil {
			return err
		}

		_ = commentTags

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

		_, _, err = n.client.PostMessageContext(ctx, channel,
			slack.MsgOptionText(joinTags(firstTimeTags), false),
			slack.MsgOptionTS(respTs))
		if err != nil {
			return err
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

		case types.SlackAttachmentCreator:
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

	case types.SlackAttachmentCreator:
		// convert object to slack attachment (if supported)
		opts = append(opts, slack.MsgOptionAttachments(append([]slack.Attachment{a.SlackAttachment()}, slackAttachments...)...))

	default:
		log.Errorf("slack message conversion error, unsupported object: %T %+v", a, a)

	}

	select {
	case n.taskC <- notifyTask{
		Channel: channel,
		Opts:    opts,
	}:
	case <-time.After(50 * time.Millisecond):
		return
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
