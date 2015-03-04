package main

import (
	"cjones.org/hg/go-xmpp2.hg/xmpp"
	"encoding/json"
	"fmt"
	"time"
)

type Msg struct {
	Time time.Time
	From xmpp.JID `json:"-"`
	Name string
	Body string
	chat *FileHistory `json:"-"`
}

func MessageToMsg(m *xmpp.Message) *Msg {
	hdr := m.GetHeader()
	return &Msg{
		Time: time.Now(), // TODO: extract timestamp from the stanza
		From: hdr.From,
		Body: ConcatText(m.Body),
	}
}

func (m *Msg) Chat(c *FileHistory) *Msg {
	m.chat = c
	m.Name = m.From.Node()
	return m
}

func (m *Msg) Deliver() (int64, error) {
	if m.chat == nil {
		m.chat, m.Name = m.route()
	}
	var (
		err error
		n   int
	)

	if Conf.LogJSON {
		var b []byte
		b, err = json.Marshal(m)
		if err != nil {
			return 0, err
		}
		n, err = m.chat.Writer.Write(b)
		m.chat.Writer.Write([]byte("\n"))
	} else {
		n, err = fmt.Fprintf(m.chat.Writer, "%s %s: %s\n", m.Time.Format("15:04:05"), m.Name, m.Body)
	}
	return int64(n), err
}

func (m *Msg) route() (*FileHistory, string) {
	jid := m.From.Bare()
	if ri := Roster.Items[jid]; ri != nil {
		return ri.Chat, ri.Name
	}
	if muc := MUCs.Items[jid]; muc != nil {
		return muc.Chat, m.From.Resource()
	}
	return Roster.UnknownChat, string(m.From)
}
