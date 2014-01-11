package main

import (
	"cjones.org/hg/go-xmpp2.hg/xmpp"
	"code.google.com/p/go9p/p"
	"code.google.com/p/go9p/p/srv"
	"reflect"
)

type Config struct {
	Jid      xmpp.JID
	Password string
	Priority int
	Nick     string
}

var Conf = Config{
	Nick: "goHateXMPP",
}

func MakeConfigDir(parent *srv.File) {
	dir := new(srv.File)
	Must(dir.Add(parent, "config", User, Group, p.DMDIR|0700, dir))
	t := reflect.TypeOf(Conf)
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		f := &FilePrintScan{val: reflect.ValueOf(&Conf).Elem().Field(i)}
		f.File.Add(dir, field.Name, User, Group, 0666, f)
	}
}
