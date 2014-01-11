package main

import (
	"cjones.org/hg/go-xmpp2.hg/xmpp"
	"code.google.com/p/go9p/p"
	"code.google.com/p/go9p/p/srv"
	"crypto/tls"
	"log"
)

type RosterItem struct {
	srv.File
	xmpp.RosterItem
}

type Roster struct {
	srv.File
}

func MakeRoster(parent *srv.File) (dir *srv.File, err error) {
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
	roster := new(Roster)
	if err = roster.Add(parent, "roster", User, nil, p.DMDIR|0700, roster); err != nil {
		return
	}
	for _, buddy := range Client.Roster.Get() {
		if _, err = roster.MakeItem(buddy); err != nil {
			return
		}
	}
	dir = &roster.File
	go func(ch <-chan xmpp.Stanza) {
		for s := range ch {
			log.Print(s)
		}
		log.Print("done reading")
	}(Client.Recv)
	return
}

func (r *Roster) MakeItem(buddy xmpp.RosterItem) (ri *RosterItem, err error) {
	nri := new(RosterItem)
	nri.RosterItem = buddy
	if err = nri.Add(&r.File, string(buddy.Jid), User, nil, p.DMDIR|0700, nri); err != nil {
		return
	}
	return nri, nil
}

func (r *Roster) Create(fid *srv.FFid, name string, perm uint32) (dir *srv.File, err error) {
	// just stub
	if Conf.Nick == "" {
		return nil, srv.Enoent
	}
	return nil, srv.Enotimpl
}
