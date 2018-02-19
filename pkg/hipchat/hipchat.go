package hipchat

import (
	"fmt"
	"net/http"
	"net/http/httputil"

	"github.com/kwiesmueller/roomsync/pkg/pipe"
	"github.com/playnet-public/libs/log"
	"github.com/tbruyelle/hipchat-go/hipchat"
	"go.uber.org/zap"
)

// Hipchat Implementation of pipe.End
type Hipchat struct {
	*Context
	Port string
}

// New Hipchat End
func New(log *log.Logger, token, channel, baseurl, port string) *Hipchat {
	c := &Context{
		baseURL: baseurl,
		log:     log.WithFields(zap.String("end", "hipchat"), zap.String("channel", channel)),
		rooms:   make(map[string]*RoomConfig),
		Token:   token,
		Channel: channel,
	}
	return &Hipchat{
		Context: c,
		Port:    port,
	}
}

// Connect to Hipchat
func (h *Hipchat) Connect() error {
	h.Context.Client = hipchat.NewClient(h.Token)
	//err := h.CreateWebhook()
	//return err
	return nil
}

// CreateWebhook without creating duplicates
func (h *Hipchat) CreateWebhook() error {
	hooks, resp, err := h.Client.Room.ListWebhooks(h.Channel, nil)
	if err != nil {
		dump, _ := httputil.DumpResponse(resp, true)
		h.log.Error("failed to get webhooks", zap.ByteString("resp", dump), zap.Error(err))
		return err
	}
	for _, webhook := range hooks.Webhooks {
		if webhook.Name == "roomsync" {
			return nil
		}
	}
	_, resp, err = h.Client.Room.CreateWebhook(h.Channel, &hipchat.CreateWebhookRequest{
		Name:    "roomsync",
		Event:   "room_message",
		Pattern: "",
		URL:     fmt.Sprintf("%s/hook", h.baseURL),
	})
	if err != nil {
		dump, _ := httputil.DumpResponse(resp, true)
		h.log.Error("webhook create error", zap.ByteString("resp", dump), zap.Error(err))
	}
	return nil
}

// Write to Hipchat
func (h *Hipchat) Write(m *pipe.Message) (err error) {
	h.log.Debug("writing msg", zap.String("msg", m.Content))
	msg := &hipchat.NotificationRequest{
		Color:         hipchat.ColorGray,
		Message:       m.Content,
		Notify:        true,
		From:          m.Author,
		MessageFormat: "text",
	}
	resp, err := h.Client.Room.Notification(h.Channel, msg)
	if err != nil {
		dump, _ := httputil.DumpResponse(resp, true)
		h.log.Error("notification error", zap.ByteString("resp", dump), zap.Error(err))
	}
	return err
}

// Listen to Hipchat
func (h *Hipchat) Listen(hook pipe.Hook) {
	h.Hook = hook
	r := h.routes()
	http.Handle("/", r)
	err := http.ListenAndServe(":"+h.Port, nil)
	if err != nil {
		h.log.Error("listen error", zap.Error(err))
	}
}
