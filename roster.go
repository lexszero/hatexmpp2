package main

import (
	"log"
	"code.google.com/p/go9p/p"
	"code.google.com/p/go9p/p/srv"
	xmpp "code.google.com/p/goexmpp"
)

type RosterItem struct {
	srv.File
	xmpp.RosterItem
}

type Roster struct {
	srv.File
}

func MakeRoster(parent *srv.File) (dir *srv.File, err error) {
	if Client, err = xmpp.NewClient(&xmpp.JID{Conf.Username, Conf.Server, Conf.Resource}, Conf.Password, nil); err != nil {
		return
	}
	if err = Client.StartSession(true, &xmpp.Presence{}); err != nil {
		return
	}
	roster := new(Roster)
	if err = roster.Add(parent, "roster", User, nil, p.DMDIR | 0700, roster); err != nil {
		return
	}
	for _, buddy := range xmpp.Roster(Client) {
		if _, err = roster.MakeItem(buddy); err != nil {
			return
		}
	}
	dir = &roster.File
	go func(ch <-chan xmpp.Stanza) {
		for _ = range ch {
		}
		log.Print("done reading")
	}(Client.In)
	return
}

func (r *Roster) MakeItem(buddy xmpp.RosterItem) (ri *RosterItem, err error) {
	nri := new(RosterItem)
	nri.RosterItem = buddy
	if err = nri.Add(&r.File, buddy.Jid, User, nil, p.DMDIR | 0700, nri); err != nil {
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
