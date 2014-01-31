package main

import (
	"cjones.org/hg/go-xmpp2.hg/xmpp"
	"code.google.com/p/go9p/p"
	"code.google.com/p/go9p/p/srv"
	"crypto/tls"
	"encoding/xml"
	"log"
	"reflect"
)

type RosterItem struct {
	srv.File
	xmpp.RosterItem
	Chat *FileHistory
}

func (ri *RosterItem) Write(p []byte) (n int, err error) {
	m := &xmpp.Message{}
	m.To = ri.RosterItem.Jid
	m.From = Client.Jid
	m.Id = xmpp.NextId()
	m.Type = "chat"
	m.Body = []xmpp.Text{{
		XMLName: xml.Name{Local: "body"},
		Chardata: string(p),
	}}
	Client.Send <- m
	MessageToMsg(m).WriteTo(ri.Chat.BufWriter)
	return len(p), nil
}

type FRoster struct {
	srv.File
	Items map[xmpp.JID]*RosterItem
	UnknownChat *FileHistory
}

func MakeRoster(parent *srv.File) (roster *FRoster, err error) {
	stat := make(chan xmpp.Status)
	go func() {
		for s := range stat {
			log.Printf("connection status %d", s)
		}
	}()
	if Client, err = xmpp.NewClient(
		&Conf.Jid,
		Conf.Password,
		tls.Config{InsecureSkipVerify: true},
		nil, xmpp.Presence{}, stat); err != nil {
		log.Printf("xmpp.NewClient:", err)
		return
	}
	roster = &FRoster{
		Items: make(map[xmpp.JID]*RosterItem),
		UnknownChat: NewFileHistory(),
	}
	Must(roster.Add(parent, "roster", User, nil, p.DMDIR|0700, roster))
	Must(roster.UnknownChat.Add(&roster.File, "UnknownChat", User, Group, 0600, roster.UnknownChat))
	for _, buddy := range Client.Roster.Get() {
		if _, err = roster.MakeItem(buddy); err != nil {
			return
		}
	}
	go func(ch <-chan xmpp.Stanza) {
		for s := range ch {
			//log.Print(s)
			ProcessStanza(s)
		}
		log.Print("done reading")
	}(Client.Recv)
	return
}

func (r *FRoster) MakeItem(buddy xmpp.RosterItem) (ri *RosterItem, err error) {
	nri := &RosterItem{
		RosterItem: buddy,
		Chat: NewFileHistory(),
	}
	nri.Chat.Writer = nri;
	Must(nri.Add(&r.File, string(buddy.Jid), User, nil, p.DMDIR|0700, nri))
	fp := &FilePrint{val: reflect.ValueOf(&buddy.Name).Elem()}
	Must(fp.Add(&nri.File, "Name", User, Group, 0400, fp))
	fp = &FilePrint{val: reflect.ValueOf(&buddy.Subscription).Elem()}
	Must(fp.Add(&nri.File, "Subscription", User, Group, 0400, fp))
	Must(nri.Chat.Add(&nri.File, "Chat", User, Group, 0600, nri.Chat))
	r.Items[buddy.Jid] = nri
	return nri, nil
}

func (r *FRoster) Create(fid *srv.FFid, name string, perm uint32) (dir *srv.File, err error) {
	// just stub
	if Conf.Nick == "" {
		return nil, srv.Enoent
	}
	return nil, srv.Enotimpl
}

func JidToName(jid xmpp.JID) string {
	ri := Roster.Items[jid.Bare()]
	if ri != nil {
		return ri.Name
	}
	return jid.Node()
}

func JidToChat(jid xmpp.JID) *FileHistory {
	ri := Roster.Items[jid.Bare()]
	if ri != nil {
		return ri.Chat
	}
	return Roster.UnknownChat
}

func ProcessStanza(s xmpp.Stanza) {
	hdr := s.GetHeader()
	switch m := s.(type) {
	case *xmpp.Message:
		MessageToMsg(m).WriteTo(JidToChat(hdr.From).BufWriter)
	default:
		log.Print("Unkown stanza: %+v", s)
	}
}
