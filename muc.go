package main

import (
	"code.google.com/p/go9p/p"
	"code.google.com/p/go9p/p/srv"
	"cjones.org/hg/go-xmpp2.hg/xmpp"
	"encoding/xml"
	"fmt"
)

type MUC struct {
	srv.File
	RamBuffer
	Jid  xmpp.JID
	Chat *FileHistory
	Members map[string]*Resource
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

func (muc *MUC) Presence(p *xmpp.Presence) {
/*	hdr := p.GetHeader()
	mname := hdr.From.Resource()
	m := muc.Members[resname]
	switch hdr.Type {
	case "unavailable":
		if res != nil {
			Log.Printf("%v: delete member '%v'", muc.Jid, mname)
			delete(muc.Members, mname)
			if mf := ri.resdir.Find(resname); rf != nil {
				rf.Remove()
			}
		}
	case "subscribe", "subscribed", "unsubscribe", "unsubscribed", "probe", "error":

	default:
		if res == nil {
			Log.Printf("%v: new resource '%v'", ri.Jid, resname)
			res = new(Resource)
			ri.Resources[resname] = res
		}
		// TODO: some fucking mutex
		MaybeSetData(&res.Show, p.Show)
		MaybeSetText(&res.Status, p.Status)
		MaybeSetData(&res.Priority, p.Priority)
	}
*/
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
		Log.Print("joining MUC ", jid)
		muc, err := NewMUC(&m.File, jid)
		if err != nil {
			Log.Print("failed", err)
			return nil, err
		}
		m.Items[jid] = muc
		Log.Print("success")
		return &muc.File, nil
	}
	return nil, srv.Enotimpl
}

func NewMUC(parent *srv.File, jid xmpp.JID) (*MUC, error) {
	muc := &MUC{
		Jid: jid,
		Members: make(map[string]*Resource),
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
	Client.Send <- m
	err := make(chan error)
	Client.SetCallback(m.Id, func(s xmpp.Stanza) {
		switch s.(type) {
		case *xmpp.Presence:
			// TODO: error parsing
			err <- nil
		default:
			err <- srv.Eperm
		}
	})
	e := <-err
	if e != nil {
		return nil, e
	}
	Must(muc.Add(parent, string(jid), User, Group, p.DMDIR|0700, muc))
	Must(muc.Chat.Add(&muc.File, "chat", User, Group, 0600, muc.Chat))
	return muc, nil
}
