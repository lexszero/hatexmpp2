package main

import (
	"cjones.org/hg/go-xmpp2.hg/xmpp"
	"code.google.com/p/go9p/p/srv"
)

type Config struct {
	srv.File
	Jid      xmpp.JID
	Password string
	Priority int
	Nick     string
	LogDir   string
}

var Conf = Config{
	Nick: "goHateXMPP",
}
