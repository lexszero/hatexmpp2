// TODO: sane roster manager (e.g. separate goroutine)

package main

import (
	"cjones.org/hg/go-xmpp2.hg/xmpp"
	"code.google.com/p/go9p/p"
	"code.google.com/p/go9p/p/srv"
	"crypto/tls"
	"encoding/xml"
	"fmt"
	"log"
	"reflect"
	"sync"
)

func sendMsg(to xmpp.JID, t string, p []byte) *xmpp.Message {
	m := &xmpp.Message{
		Header: xmpp.Header{
			To:   to,
			From: Client.Jid,
			Id:   xmpp.NextId(),
			Type: t,
		},
		Body: []xmpp.Text{{
			XMLName:  xml.Name{Local: "body"},
			Chardata: string(p),
		}},
	}
	Client.Send <- m
	return m
}

type Resource struct {
	srv.File
	Jid      xmpp.JID
	Chat     *FileHistory
	Show     string
	Status   string
	Priority string
}

func (r *Resource) Write(p []byte) (n int, err error) {
	m := sendMsg(r.Jid, "chat", p)
	msg := MessageToMsg(m)
	msg.Chat(r.Chat).Deliver()
	msg.Chat(Roster.Items[r.Jid.Bare()].Chat).Deliver()
	return len(p), nil
}

type RosterItem struct {
	srv.File
	xmpp.RosterItem
	sync.Mutex
	Chat *FileHistory
	Resources map[string]*Resource
}

func (ri *RosterItem) Write(p []byte) (n int, err error) {
	m := sendMsg(ri.RosterItem.Jid, "chat", p)
	MessageToMsg(m).Chat(ri.Chat).Deliver()
	return len(p), nil
}

func (ri *RosterItem) AddResource(name string) *Resource {
	ri.Lock()
	defer ri.Unlock()

	if r := ri.Resources[name]; r != nil {
		return r
	}
	r := &Resource{
		Jid: xmpp.JID(fmt.Sprintf("%v/%v", ri.RosterItem.Jid, name)),
	}
	r.Chat = NewFileHistory(r)
	r.Add(ri.Find("Resources"), name, User, Group, p.DMDIR|0700, r)
	fp := &FilePrint{val: reflect.ValueOf(&r.Show)}
	fp.Add(&r.File, "Show", User, Group, 0400, fp)
	fp = &FilePrint{val: reflect.ValueOf(&r.Status)}
	fp.Add(&r.File, "Status", User, Group, 0400, fp)
	fp = &FilePrint{val: reflect.ValueOf(&r.Priority)}
	fp.Add(&r.File, "Priority", User, Group, 0400, fp)
	r.Chat.Add(&r.File, "Chat", User, Group, 0600, r.Chat)
	ri.Resources[name] = r
	return r
}

func (ri *RosterItem) RemoveResource(name string) {
	ri.Lock()
	defer ri.Unlock()

	r := ri.Resources[name]
	if r == nil {
		return
	}
	r.Chat.Stop()
	for _, name := range []string{"Show", "Status", "Priority", "Chat"} {
		r.Find(name).Remove()
	}
	r.Remove()

	delete(ri.Resources, name)
}

func (ri *RosterItem) Presence(p *xmpp.Presence) {
	hdr := p.GetHeader()
	resname := hdr.From.Resource()
	res := ri.Resources[resname]
	switch hdr.Type {
	case "unavailable":
		if res != nil {
			Log.Printf("%v: delete resource '%v'", ri.Jid, resname)
			ri.RemoveResource(resname)
		}
	case "subscribe", "subscribed", "unsubscribe", "unsubscribed", "probe", "error":

	default:
		if res == nil {
			Log.Printf("%v: new resource '%v'", ri.Jid, resname)
			res = ri.AddResource(resname)
		}
		// TODO: some fucking mutex
		MaybeSetData(&res.Show, p.Show)
		MaybeSetText(&res.Status, p.Status)
		MaybeSetData(&res.Priority, p.Priority)
	}
}

type FRoster struct {
	srv.File
	Items       map[xmpp.JID]*RosterItem
	UnknownChat *FileHistory
}

func MakeRoster(parent *srv.File) (roster *FRoster, err error) {
	stat := make(chan xmpp.Status)
	go func() {
		for s := range stat {
			Log.Printf("connection status %d", s)
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
		Items:       make(map[xmpp.JID]*RosterItem),
		UnknownChat: NewFileHistory(new(RamBuffer)),
	}
	Must(roster.Add(parent, "roster", User, nil, p.DMDIR|0700, roster))
	Must(roster.UnknownChat.Add(&roster.File, "UnknownChat", User, Group, 0600, roster.UnknownChat))
	for _, buddy := range Client.Roster.Get() {
		roster.AddItem(buddy)
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

func (r *FRoster) AddItem(buddy xmpp.RosterItem) (ri *RosterItem) {
	r.Lock()
	defer r.Unlock()

	nri := &RosterItem{
		RosterItem: buddy,
	}
	nri.Chat = NewFileHistory(nri)
	nri.Resources = make(map[string]*Resource)
	Must(nri.Add(&r.File, string(buddy.Jid), User, nil, p.DMDIR|0700, nri))
	fp := &FilePrint{val: reflect.ValueOf(&buddy.Name).Elem()}
	Must(fp.Add(&nri.File, "Name", User, Group, 0400, fp))
	fp = &FilePrint{val: reflect.ValueOf(&buddy.Subscription).Elem()}
	Must(fp.Add(&nri.File, "Subscription", User, Group, 0400, fp))
	Must(nri.Chat.Add(&nri.File, "Chat", User, Group, 0600, nri.Chat))
	resdir := &srv.File{}
	Must(resdir.Add(&nri.File, "Resources", User, Group, p.DMDIR|0700, resdir))
	r.Items[buddy.Jid] = nri
	return nri
}

func (r *FRoster) RemoveItem(jid xmpp.JID) {
	r.Lock()
	defer r.Unlock()

	ri := r.Items[jid]
	if ri == nil {
		return
	}
	ri.Chat.Stop()
	for res := range ri.Resources {
		ri.RemoveResource(res)
	}
	for _, name := range []string{"Name", "Subscription", "Chat", "Resources"} {
		ri.Find(name).Remove()
	}
	ri.Remove()
	delete(r.Items, jid)
}

func (r *FRoster) Create(fid *srv.FFid, name string, perm uint32) (dir *srv.File, err error) {
	// just stub
	if Conf.Nick == "" {
		return nil, srv.Enoent
	}
	return nil, srv.Enotimpl
}

func ProcessStanza(s xmpp.Stanza) {
	hdr := s.GetHeader()
	switch m := s.(type) {
	case *xmpp.Message:
		MessageToMsg(m).Deliver()
	case *xmpp.Presence:
		Log.Printf("presence from %v, type=%v", hdr.From, hdr.Type)
		from := hdr.From.Bare()
		if ri := Roster.Items[from]; ri != nil {
			ri.Presence(m)
		}
		if muc := MUCs.Items[from]; muc != nil {
			muc.Presence(m)
		}
	default:
		log.Printf("Unkown stanza: %+v", s)
	}
}
