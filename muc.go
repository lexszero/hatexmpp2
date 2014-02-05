package main

import (
	"code.google.com/p/go9p/p"
	"code.google.com/p/go9p/p/srv"
	"cjones.org/hg/go-xmpp2.hg/xmpp"
	"encoding/xml"
	"fmt"
	"log"
)

type MUC struct {
	srv.File
	RamBuffer
	Jid  xmpp.JID
	Chat *FileHistory
	Members map[string]*RosterItem
}

func (muc *MUC) Write(p []byte) (n int, err error) {
	m := &xmpp.Message{
		Header: xmpp.Header{
			To: muc.Jid,
			From: Client.Jid,
			Id: xmpp.NextId(),
			Type: "groupchat",
		},
		Body: []xmpp.Text{{
			XMLName:  xml.Name{Local: "body"},
			Chardata: string(p),
		}},
	}
	Client.Send <- m
	return len(p), nil
}

type FMUCDir struct {
	srv.File
	Items map[xmpp.JID]*MUC
}

func MakeMUCsDir(parent *srv.File) (m *FMUCDir) {
	m = &FMUCDir{
		Items: make(map[xmpp.JID]*MUC),
	}
	Must(m.Add(parent, "muc", User, Group, p.DMDIR|0700, m))
	return
}

func (m *FMUCDir) Create(fid *srv.FFid, name string, perm uint32) (*srv.File, error) {
	if  perm & p.DMDIR != 0 {
		jid := xmpp.JID(name)
		if _, ok := m.Items[jid]; ok {
			return nil, srv.Eexist
		}
		muc, err := NewMUC(&m.File, jid)
		if err != nil {
			return nil, err
		}
		m.Items[jid] = muc
		return &muc.File, nil
	}
	return nil, srv.Enotimpl
}

func NewMUC(parent *srv.File, jid xmpp.JID) (muc *MUC, err error) {
	muc = &MUC{
		Jid: jid,
		Members: make(map[string]*RosterItem),
	}
	muc.Chat = NewFileHistory(muc)
	m := &xmpp.Presence{
		Header: xmpp.Header{
			To: xmpp.JID(fmt.Sprintf("%s/%s", jid, Conf.Nick)),
			From: Client.Jid,
			Id: xmpp.NextId(),
			Innerxml: "<x xmlns='http://jabber.org/protocol/muc'/>",
		},
	}
	log.Print("joining MUC ", jid)
	Client.Send <- m
	reply := make(chan int)
	Client.SetCallback(m.Id, func(s xmpp.Stanza) {
		reply <- 1
	})
	log.Print("waiting reply")
	<-reply
	Must(muc.Add(parent, string(jid), User, Group, p.DMDIR|0700, muc))
	Must(muc.Chat.Add(&muc.File, "Chat", User, Group, 0600, muc.Chat))
	return
}
