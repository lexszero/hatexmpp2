package main

import (
	"cjones.org/hg/go-xmpp2.hg/xmpp"
	"code.google.com/p/go9p/p"
	"code.google.com/p/go9p/p/srv"
	"flag"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
)

var (
	addr  = flag.String("addr", ":5640", "listen address")
	debug = flag.Int("d", 0, "print debug messages")

	/* FIXME: Os{User,Group}s are broken and useless stubs in go9p
	 * Linux 9p implementation doesn't like it much, hint: use version=9p2000.u
	 * Need to either fix it there or provide our own sane user/group types
	 */
	User  = p.OsUsers.Uid2User(os.Geteuid())
	Group = p.OsUsers.Gid2Group(os.Getegid())

	Client *xmpp.Client
)

type Root struct {
	srv.File
}

func (r *Root) Create(fid *srv.FFid, name string, perm uint32) (dir *srv.File, err error) {
	switch {
	case name == "roster" && (perm&p.DMDIR != 0):
		return MakeRoster(&r.File)
	}
	return nil, srv.Enotimpl
}

func main() {
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()
	flag.Parse()
	root := new(Root)
	Must(root.Add(nil, "/", User, Group, p.DMDIR|0700, root))
	MakeConfigDir(&root.File)
	s := srv.NewFileSrv(&root.File)
	s.Dotu = true
	s.Debuglevel = *debug
	s.Start(s)
	Must(s.StartNetListener("tcp", *addr))
}
