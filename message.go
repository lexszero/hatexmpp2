package main

import (
	"cjones.org/hg/go-xmpp2.hg/xmpp"
	"fmt"
	"time"
)

type Msg struct {
	Time time.Time
	From xmpp.JID
	Name string
	Body string
	chat *FileHistory
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
	n, e := fmt.Fprintf(m.chat.Writer, "%s %s: %s\n", m.Time.Format("15:04:05"), m.Name, m.Body)
	return int64(n), e
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
