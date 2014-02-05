package main

import (
	"cjones.org/hg/go-xmpp2.hg/xmpp"
	"fmt"
	"io"
	"strings"
	"time"
)

type Msg struct {
	Time time.Time
	From xmpp.JID
	Body string
}

func concatText(t []xmpp.Text) string {
	s := make([]string, len(t))
	for i := range t {
		s[i] = string(t[i].Chardata)
	}
	return strings.Join(s, "\n")
}

func MessageToMsg(m *xmpp.Message) *Msg {
	return &Msg{
		Time: time.Now(), // TODO: extract timestamp from the stanza
		From: m.GetHeader().From,
		Body: concatText(m.Body),
	}
}

func (m *Msg) WriteTo(wr io.Writer) (int64, error) {
	n, e := fmt.Fprintf(wr, "%s %s: %s\n", m.Time.Format("15:04:05"), JidToName(m.From), m.Body)
	return int64(n), e
}
