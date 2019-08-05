package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync/atomic"

	"github.com/digitalrebar/provision/v4/models"
	"github.com/gorilla/websocket"
)

type Bot struct {
	msgId      int64
	slackToken string
	slackConn  *websocket.Conn
	auth       *apiAuth
	channels   []apiChannel
	groups     []apiGroup
}

// base type for incoming messages.  This will be embedded into other types.
type evtBase struct {
	Type    string `json:"type,omitempty"`
	Subtype string `json:"subtype,omitempty"`
}

/* Not allowed by RTM - could be used with ChatAPI
type Field struct {
	Title string `json:"title:omitempty"`
	Value string `json:"value:omitempty"`
	Short bool   `json:"short:omitempty"`
}

type Attachment struct {
	Fallback   string  `json:"fallback,omitempty"`
	Color      string  `json:"color,omitempty"`
	Pretext    string  `json:"pretext,omitempty"`
	AuthorName string  `json:"author_name,omitempty"`
	AuthorLink string  `json:"author_link,omitempty"`
	AutherIcon string  `json:"author_icon,omitempty"`
	Title      string  `json:"title,omitempty"`
	TitleLink  string  `json:"title_link,omitempty"`
	Text       string  `json:"text,omitempty"`
	Fields     []Field `json:"fields,omitempty"`
	ImageURL   string  `json:"image_url,omitempty"`
	ThumbURL   string  `json:"thumb_url,omitempty"`
	footer     string  `json:"footer,omitempty"`
	footerIcon string  `json:"footer_icon,omitempty"`
	Time       int64   `json:"ts,omitempty"`
}
*/

// All outgoing messages to Slack will be of this type.
type Msg struct {
	ID      int64  `json:"id"`
	Channel string `json:"channel,omitempty"`
	Text    string `json:"text,omitempty"`
	Type    string `json:"type,omitempty"`
}

type apiBase struct {
	OK      bool   `json:"ok"`
	Error   string `json:"error"`
	Warning string `json:"warning"`
}

func (sb *apiBase) Ok() bool {
	return sb.OK
}

func (sb *apiBase) ErrOrNil() error {
	if sb.Ok() {
		return nil
	}
	return errors.New(sb.Error)
}

type Apier interface {
	Ok() bool
	ErrOrNil() error
}

type apiConnResp struct {
	apiBase
	URL  string `json:"url"`
	Team struct {
		ID             string `json:"id"`
		Name           string `json:"name"`
		Domain         string `json:"domain"`
		EnterpriseId   string `json:"enterprise_id"`
		EnterpriseName string `json:"enterprise_name"`
	} `json:"team"`
	Self struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"self"`
}

type apiAuth struct {
	apiBase
	Team   string `json:"team"`
	User   string `json:"user"`
	TeamID string `json:"team_id"`
	UserID string `json:"user_id"`
}

// this only includes the data we care about.
type apiChannel struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	IsArchived bool   `json:"is_archived"`
	IsMember   bool   `json:"is_member"`
}

type apiChannels struct {
	apiBase
	Channels []apiChannel `json:"channels"`
}

type apiGroup struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	IsArchived bool   `json:"is_archived"`
}

type apiGroups struct {
	apiBase
	Groups []apiGroup `json:"groups"`
}

func (s *Bot) apiGet(v Apier, api string, args ...string) error {
	url := fmt.Sprintf("https://slack.com/api/%s?token=%s", api, s.slackToken)
	if len(args) > 0 {
		url = fmt.Sprintf("%s&%s", url, strings.Join(args, "&"))
	}
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	if resp.Body != nil {
		defer resp.Body.Close()
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Slack API failed with status %d", resp.StatusCode)
	}
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&v); err != nil {
		return err
	}
	return v.ErrOrNil()
}

func (s *Bot) wsEventLoop() {
	for {
		msg := map[string]interface{}{}
		if err := s.slackConn.ReadJSON(&msg); err != nil {
			log.Fatalf("Error reading frame from Slack: %v", err)
		}
		log.Printf("Received: %v", msg)
		msgType, ok := msg["type"].(string)
		if !ok || msgType == "" {
			log.Printf("Invalid message received from Slack, ignoring")
		}

		// Add handling code for appropriate message types here
		if msgType == "message" {
			message := msg["text"].(string)
			log.Printf("Message: %s\n", message)

		} else {
			log.Printf("Message type %s received", msgType)
		}
	}
}

func (s *Bot) Send(msg Msg) error {
	msg.ID = atomic.AddInt64(&s.msgId, 1)
	if msg.Type == "" {
		msg.Type = "message"
	}
	return s.slackConn.WriteJSON(&msg)
}

func (s *Bot) Publish(e *models.Event) error {
	m := e.Text()

	log.Printf("Sending event: %v\n", m)

	for _, channel := range s.channels {
		if err := s.Send(Msg{Channel: channel.ID, Text: m}); err != nil {
			return err
		}
	}
	for _, group := range s.groups {
		if err := s.Send(Msg{Channel: group.ID, Text: m}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Bot) DialSlack() error {
	whoami := &apiAuth{}
	channels := &apiChannels{}
	groups := &apiGroups{}
	if err := s.apiGet(whoami, "auth.test"); err != nil {
		return err
	}
	s.auth = whoami
	if err := s.apiGet(channels, "channels.list", "exclude_archived=true", "exclude_members=true"); err != nil {
		return err
	}
	s.channels = []apiChannel{}
	for _, channel := range channels.Channels {
		if !channel.IsMember {
			continue
		}
		log.Printf("%s is a member of channel %s", s.auth.User, channel.Name)
		s.channels = append(s.channels, channel)
	}
	if err := s.apiGet(groups, "groups.list", "exclude_archived=true"); err != nil {
		return err
	}
	s.groups = groups.Groups
	for _, group := range groups.Groups {
		log.Printf("%s is a member of group %s", s.auth.User, group.Name)
	}
	wsConn := &apiConnResp{}
	if err := s.apiGet(wsConn, "rtm.connect"); err != nil {
		return err
	}
	var dialer *websocket.Dialer
	ws, _, err := dialer.Dial(wsConn.URL, nil)
	if err != nil {
		return err
	}
	s.slackConn = ws
	go s.wsEventLoop()

	for _, channel := range s.channels {
		s.Send(Msg{Channel: channel.ID, Text: fmt.Sprintf("%s lives!", s.auth.User)})
	}
	for _, group := range s.groups {
		s.Send(Msg{Channel: group.ID, Text: fmt.Sprintf("%s lives!", s.auth.User)})
	}
	return nil
}

func (s *Bot) Shutdown() error {
	return s.slackConn.Close()
}

func NewSlackBot(slackToken string) (*Bot, error) {
	res := &Bot{
		slackToken: slackToken,
	}
	if err := res.DialSlack(); err != nil {
		return nil, err
	}
	return res, nil
}
