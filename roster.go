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
	"sync"
	"time"
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
	Jid      xmpp.JID     `9p:"-"`
	Chat     *FileHistory `9p:"mode=0600,nodir"`
	Show     string       `9p:"mode=0400"`
	Status   string       `9p:"mode=0400"`
	Priority string       `9p:"mode=0400"`
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
	xmpp.RosterItem `9p:"-"`
	sync.Mutex      `9p:"-"`
	Chat            *FileHistory         `9p:"mode=0600,nodir"`
	Resources       map[string]*Resource `9p:"-"`
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
	r.Chat = NewFileHistory(r, nil)
	Must(FileRecursiveAdd(ri.Find("resources"), r, name, p.DMDIR|0700))
	ri.Resources[name] = r
	return r
}

func (ri *RosterItem) RemoveResource(name string) {
	log.Printf("RemoveResource %s %s", ri.Name, name)
	ri.Lock()
	defer ri.Unlock()

	r := ri.Resources[name]
	if r == nil {
		return
	}
	r.Chat.Stop()
	for _, name := range []string{"show", "status", "priority", "chat"} {
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
	stop        chan bool
	stat        chan xmpp.Status
	pinger      *Pinger
}

func (roster *FRoster) connect() (err error) {
	log.Print("connecting")
	// First of all, we need to process status messages from xmpp lib
	roster.stop = make(chan bool)
	roster.stat = make(chan xmpp.Status)
	go func() {
		fatal := false
		defer log.Print("disconnected")
		defer func() {
			if fatal {
				roster.UnknownChat.Stop()
				roster.UnknownChat.Remove()
				roster.File.Remove()
			}
		}()
		for {
			select {
			case s, ok := <-roster.stat:
				if ok {
					log.Print("connection status ", s)
					if s.Fatal() { // whoops, shit happened
						roster.shutdown()
					} else if s == xmpp.StatusRunning {
						Log.Printf("connection established")
						// don't forget to cleanup
						defer func() {
							for jid := range roster.Items {
								roster.RemoveItem(jid)
							}
							Client.Close()
						}()

						// start pinger when connection is fully up
						pinger := MakePinger(xmpp.JID(Conf.Jid.Domain()),
							Conf.PingPeriod, Conf.PingTimeout,
							func(jid xmpp.JID) bool {
								roster.shutdown()
								return false
							})

						// clean up nicely when there will be time to die
						defer pinger.Stop()
					}
				} else {
					// just for sure
					roster.shutdown()
				}
			case fatal = <-roster.stop:
				log.Print("stop issued")
				if !fatal && Conf.Reconnect >= 0 {
					// this goroutine will resurrect everything up
					go func() {
						if Conf.Reconnect > 0 {
							log.Printf("sleeping %v seconds", Conf.Reconnect)
							time.Sleep(time.Duration(Conf.Reconnect) * time.Second)
						}
						roster.connect()
					}()
				}
				return
			}
		}
	}()

	Client, err = xmpp.NewClient(
		&Conf.Jid,
		Conf.Password,
		&tls.Config{InsecureSkipVerify: true},
		nil,
		xmpp.Presence{
			Header: xmpp.Header{
				From: Conf.Jid,
				Id:   xmpp.NextId(),
			},
		},
		roster.stat)
	if err != nil {
		log.Printf("xmpp.NewClient:", err)
		roster.shutdown()
		return
	}

	go func() {
		// get roster and finally begin useful work
		for _, buddy := range Client.Roster.Get() {
			roster.AddItem(buddy)
		}
		for s := range Client.Recv {
			ProcessStanza(s)
		}
	}()
	return
}

func (r *FRoster) shutdown() {
	log.Print("send stop")
	r.stop <- false
}

func MakeRoster(parent *srv.File) (roster *FRoster, err error) {
	roster = &FRoster{
		Items:       make(map[xmpp.JID]*RosterItem),
		UnknownChat: NewFileChat("unknown", nil),
	}

	Must(roster.Add(parent, "roster", User, Group, p.DMDIR|0700, roster))
	Must(roster.UnknownChat.Add(&roster.File, "unknown", User, Group, 0600, roster.UnknownChat))

	err = roster.connect()

	return
}

func (r *FRoster) Remove(fid *srv.FFid) error {
	log.Print("removing roster")
	r.stop <- true
	return nil
}

func (r *FRoster) AddItem(buddy xmpp.RosterItem) (ri *RosterItem) {
	nri := &RosterItem{
		RosterItem: buddy,
		Resources:  make(map[string]*Resource),
	}
	nri.Chat = NewFileChat(string(buddy.Jid), nri)
	Must(FileRecursiveAdd(&r.File, nri, string(buddy.Jid), p.DMDIR|0700))
	resdir := &srv.File{}
	Must(resdir.Add(&nri.File, "resources", User, Group, p.DMDIR|0700, resdir))
	r.Items[buddy.Jid] = nri
	return nri
}

func (r *FRoster) RemoveItem(jid xmpp.JID) {
	ri := r.Items[jid]
	if ri == nil {
		return
	}
	ri.Chat.Stop()
	for res := range ri.Resources {
		ri.RemoveResource(res)
	}
	for _, name := range []string{"chat", "resources"} {
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
	case *xmpp.Iq:
		Log.Printf("iq from %v, type=%v", hdr.From, hdr.Type)
	default:
		log.Printf("Unknown stanza: %+v", s)
	}
}
