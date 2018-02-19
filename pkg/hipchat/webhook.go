package hipchat

import (
	"encoding/json"
	"html/template"
	"net/http"
	"net/http/httputil"
	"path"
	"strconv"
	"time"

	"github.com/kwiesmueller/roomsync/pkg/pipe"

	"github.com/bborbe/http/util"
	"github.com/gorilla/mux"
	"github.com/playnet-public/libs/log"
	"github.com/tbruyelle/hipchat-go/hipchat"
	"go.uber.org/zap"
)

// RoomConfig holds information to send messages to a specific room
type RoomConfig struct {
	token *hipchat.OAuthAccessToken
	hc    *hipchat.Client
	name  string
}

// Context keep context of the running application
type Context struct {
	baseURL string
	static  string

	log *log.Logger

	Token   string
	Channel string
	Hook    func(*pipe.Message) error
	Client  *hipchat.Client

	//rooms per room OAuth configuration and client
	rooms map[string]*RoomConfig
}

func (c *Context) healthcheck(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode([]string{"OK"})
}

func (c *Context) atlassianConnect(w http.ResponseWriter, r *http.Request) {
	lp := path.Join("./static", "atlassian-connect.json")
	vals := map[string]string{
		"LocalBaseUrl": c.baseURL,
	}
	tmpl, err := template.ParseFiles(lp)
	if err != nil {
		c.log.Error("config parse error", zap.Error(err))
		return
	}
	tmpl.ExecuteTemplate(w, "config", vals)
}

func (c *Context) installable(w http.ResponseWriter, r *http.Request) {
	authPayload, err := util.DecodePostJSON(r, true)
	if err != nil {
		c.log.Error("parsing auth data failed", zap.Error(err))
		return
	}

	credentials := hipchat.ClientCredentials{
		ClientID:     authPayload["oauthId"].(string),
		ClientSecret: authPayload["oauthSecret"].(string),
	}
	roomName := strconv.Itoa(int(authPayload["roomId"].(float64)))
	tok, _, err := c.Client.GenerateToken(
		credentials,
		[]string{
			hipchat.ScopeSendNotification,
			hipchat.ScopeViewMessages,
			hipchat.ScopeViewRoom,
		},
	)
	if err != nil {
		c.log.Error("failed to get access token", zap.Error(err))
		return
	}
	rc := &RoomConfig{
		name: roomName,
		hc:   tok.CreateClient(),
	}
	c.rooms[roomName] = rc

	json.NewEncoder(w).Encode([]string{"OK"})
}

func (c *Context) webhook(w http.ResponseWriter, r *http.Request) {
	payload, err := DecodePostJSON(r)
	if err != nil {
		c.log.Error("parsing auth data failed", zap.Error(err))
		return
	}
	roomID := strconv.Itoa(int((payload["item"].(map[string]interface{}))["room"].(map[string]interface{})["id"].(float64)))

	dump, _ := httputil.DumpRequest(r, true)
	w.Write(dump)
	c.log.Debug("converting hook")
	if payload["event"].(string) != "room_message" {
		return
	}
	if roomID != c.Channel {
		return
	}

	msg := &pipe.Message{}
	msg.Author = payload["item"].(map[string]interface{})["message"].(map[string]interface{})["from"].(map[string]interface{})["mention_name"].(string)
	msg.Timestamp = time.Now()
	msg.Source = roomID
	msg.Content = payload["item"].(map[string]interface{})["message"].(map[string]interface{})["message"].(string)

	c.log.Debug("triggering hook", zap.String("room", roomID), zap.ByteString("dump", dump))
	err = c.Hook(msg)
	if err != nil {
		c.log.Error("hook error", zap.String("msg", msg.String()), zap.Error(err))
	}
}

// routes all URL routes for app add-on
func (c *Context) routes() *mux.Router {
	r := mux.NewRouter()
	r.Path("/").Methods("GET").HandlerFunc(c.atlassianConnect)
	r.Path("/healthcheck").Methods("GET").HandlerFunc(c.healthcheck)
	r.Path("/atlassian-connect.json").Methods("GET").HandlerFunc(c.atlassianConnect)

	// HipChat specific API routes
	r.Path("/installable").Methods("POST").HandlerFunc(c.installable)
	r.Path("/hook").Methods("POST").HandlerFunc(c.webhook)

	//r.PathPrefix("/").Handler(http.FileServer(http.Dir(c.static)))
	return r
}

// DecodePostJSON into a ma[string]interface{}
func DecodePostJSON(r *http.Request) (map[string]interface{}, error) {
	var err error
	var payLoad map[string]interface{}
	decoder := json.NewDecoder(r.Body)
	err = decoder.Decode(&payLoad)
	return payLoad, err
}
