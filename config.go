package main

import (
	"github.com/lexszero/go-xmpp2/xmpp"
	"code.google.com/p/go9p/p/srv"
)

type Config struct {
	srv.File
	Jid         xmpp.JID
	Password    string
	Priority    int
	Nick        string
	LogDir      string
	PingPeriod  int
	PingTimeout int
	Reconnect   int
	LogJSON     bool
}

var Conf = Config{
	Nick:        "goHateXMPP",
	PingPeriod:  2,
	PingTimeout: 5,
	Reconnect:   2,
	LogJSON:     false,
}
