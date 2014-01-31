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

	Srv = &Server{
		Flushers: make(map[*srv.Fid]Flusher),
	}
	Client *xmpp.Client
	Roster *FRoster
)

type Server struct {
	srv.Fsrv
	Flushers map[*srv.Fid]Flusher
}

type Flusher interface {
	Flush(*srv.Fid)
}

func (s *Server) Flush(req *srv.Req) {
	if f, ok := s.Flushers[req.Fid]; ok {
		f.Flush(req.Fid)
	}
	req.Flush()
}

type FRoot struct {
	srv.File
}

func (r *FRoot) Create(fid *srv.FFid, name string, perm uint32) (dir *srv.File, err error) {
	switch {
	case name == "roster" && (perm&p.DMDIR != 0):
		Roster, err = MakeRoster(&r.File)
		return &Roster.File, err
	}
	return nil, srv.Enotimpl
}

func main() {
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()
	flag.Parse()
	root := new(FRoot)
	Must(root.Add(nil, "/", User, Group, p.DMDIR|0700, root))
	MakeConfigDir(&root.File)
	Srv.Fsrv.Root = &root.File
	Srv.Dotu = true
	Srv.Debuglevel = *debug
	Srv.Start(Srv)
	Must(Srv.StartNetListener("tcp", *addr))
}
