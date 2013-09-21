package main

import (
	_ "net/http/pprof"
	"net/http"
	"log"
	"flag"
	"os"
	"code.google.com/p/go9p/p"
	"code.google.com/p/go9p/p/srv"
	xmpp "code.google.com/p/goexmpp"
	"crypto/tls"
)

var (
	addr = flag.String("addr", ":5640", "listen address")
	debug = flag.Int("d", 0, "print debug messages")
	User = p.OsUsers.Uid2User(os.Geteuid())
	Client *xmpp.Client
)

type Root struct {
	srv.File
}

func (r *Root) Create(fid *srv.FFid, name string, perm uint32) (dir *srv.File, err error) {
	switch {
	case name == "roster" && (perm & p.DMDIR != 0):
		return MakeRoster(&r.File)
	}
	return nil, srv.Enotimpl
}

type StdLogger struct {
}

func (s *StdLogger) Log(v ...interface{}) {
	 log.Println(v...)
}

func (s *StdLogger) Logf(fmt string, v ...interface{}) {
	 log.Printf(fmt, v...)
}

func init() {
	 logger := &StdLogger{}
	 //xmpp.Debug = logger
	 xmpp.Info = logger
	 xmpp.Warn = logger
	 xmpp.TlsConfig = tls.Config{InsecureSkipVerify: true}
}

func main() {
	var err error
	defer func() {
		if err != nil {
			log.Fatal(err)
		}
	}()
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()
	flag.Parse()
	root := new(Root)
	if err = root.Add(nil, "/", User, nil, p.DMDIR|0700, root); err != nil {
		return
	}
	
	MakeConfigDir(&root.File)
	s := srv.NewFileSrv(&root.File)
	s.Dotu = true
	s.Debuglevel = *debug
	s.Start(s)
	if err = s.StartNetListener("tcp", *addr); err != nil {
		return
	}
}
