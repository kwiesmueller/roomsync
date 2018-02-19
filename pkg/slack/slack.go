package slack

import (
	"strings"
	"time"

	"github.com/kwiesmueller/roomsync/pkg/pipe"
	"github.com/nlopes/slack"
	"github.com/playnet-public/libs/log"
	"go.uber.org/zap"
)

// Slack Implementation of pipe.End
type Slack struct {
	Token   string
	Channel string

	Client *slack.Client
	RTM    *slack.RTM

	log *log.Logger
}

// New Slack End
func New(log *log.Logger, token, channel string) *Slack {
	return &Slack{
		Token:   token,
		Channel: channel,
		log:     log.WithFields(zap.String("end", "slack"), zap.String("channel", channel)),
	}
}

// Connect to Slack
func (s *Slack) Connect() error {
	s.Client = slack.New(s.Token)
	slack.SetLogger(zap.NewStdLog(s.log.Logger))
	s.RTM = s.Client.NewRTM()
	go s.RTM.ManageConnection()
	return nil
}

// Write to Slack
func (s *Slack) Write(m *pipe.Message) (err error) {
	s.log.Debug("writing msg", zap.String("msg", m.Content))
	params := slack.NewPostMessageParameters()
	params.Channel = s.Channel

	uid := s.UserByName(m.Author)
	user, err := s.Client.GetUserInfo(uid)
	if err == nil {
		params.User = user.ID
		params.IconURL = user.Profile.Image48
	} else {
		s.log.Error("user get error", zap.String("username", m.Author), zap.String("uid", uid), zap.Error(err))
	}
	params.Username = m.Author
	_, _, err = s.Client.PostMessage(s.Channel, m.Content, params)
	if err != nil {
		s.log.Error("send message error", zap.String("msg", m.String()), zap.Error(err))
		return err
	}
	return nil
}

// UserByName as a helper (might be expensive)
func (s *Slack) UserByName(name string) string {
	users, err := s.Client.GetUsers()
	if err != nil {
		s.log.Error("failed to fetch users", zap.Error(err))
		return ""
	}
	for _, user := range users {
		if strings.HasSuffix(user.Name, name) {
			return user.ID
		}
	}
	s.log.Error("user not found", zap.String("name", name))
	return ""
}

// Listen to Slack
func (s *Slack) Listen(hook pipe.Hook) {
	for msg := range s.RTM.IncomingEvents {
		//s.log.Debug("event received")
		switch ev := msg.Data.(type) {
		case *slack.HelloEvent:
			// Ignore hello

		case *slack.ConnectedEvent:
			s.log.Debug("connected", zap.Int("count", ev.ConnectionCount), zap.String("url", ev.Info.URL))

		case *slack.MessageEvent:
			if ev.Msg.Channel != s.Channel {
				continue
			}
			// Skip threads
			if ev.ThreadTimestamp != "" {
				continue
			}
			// Skip bots
			if ev.BotID != "" {
				continue
			}
			s.log.Debug("message received", zap.String("msg", ev.Msg.Text), zap.String("type", ev.Type))
			user, err := s.Client.GetUserInfo(ev.User)
			var username string
			if err != nil {
				s.log.Error("unable to fetch user", zap.String("user", ev.User))
				username = ev.User
			} else {
				username = user.Name
			}
			msg := &pipe.Message{
				Author:    username,
				Timestamp: time.Now(),
				Source:    s.Channel,
				Content:   ev.Msg.Text,
			}
			err = hook(msg)
			if err != nil {
				s.log.Error("hook error", zap.String("msg", msg.String()), zap.Error(err))
			}

		case *slack.PresenceChangeEvent:
			s.log.Debug("presence changed", zap.String("user", ev.User), zap.String("presence", ev.Presence))

		case *slack.LatencyReport:
			s.log.Debug("latency report", zap.String("value", ev.Value.String()))

		case *slack.RTMError:
			s.log.Error("rtm error", zap.String("err", ev.Error()))

		case *slack.InvalidAuthEvent:
			s.log.Error("invalid credentials")
			return

		default:

			// Ignore other events..
			// s.log.Debugf("Unexpected: %v\n", msg.Data)
		}
	}
}
