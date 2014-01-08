package main

import (
	"log"
	"reflect"
	"fmt"
	"code.google.com/p/go9p/p"
	"code.google.com/p/go9p/p/srv"
	"cjones.org/hg/go-xmpp2.hg/xmpp"
)

type Config struct {
	Jid xmpp.JID
	Password string
	Priority int
	Nick string
}

var Conf = Config{
	Nick: "goHateXMPP",
}

type ConfigFile struct {
	srv.File
	val reflect.Value
}

func (cf *ConfigFile) Read(fid *srv.FFid, buf []byte, offset uint64) (int, error) {
	cf.Lock()
	defer cf.Unlock()

	b := []byte(fmt.Sprint(cf.val.Interface()))
	have := len(b)
	off := int(offset)
	if off >= have {
		return 0, nil
	}
	return copy(buf, b[off:]), nil
}

func (cf *ConfigFile) Write(fid *srv.FFid, buf []byte, offset uint64) (n int, err error) {
	cf.Lock()
	defer cf.Unlock()

	s := string(buf)
	switch cf.val.Kind() {
	case reflect.String:
		cf.val.SetString(s)
	default:
		_, err = fmt.Sscan(s, cf.val.Addr().Interface())
	}
	if err != nil {
		log.Print(err)
	}
	n = len(buf)
	return
}

func (cf *ConfigFile) Wstat(fid *srv.FFid, dir *p.Dir) error {
	cf.Lock()
	defer cf.Unlock()

	return nil
}

func MakeConfigDir(parent *srv.File) {
	dir := new(srv.File)
	if err := dir.Add(parent, "config", User, nil, p.DMDIR|0700, dir); err != nil {
		panic(err)
	}
	t := reflect.TypeOf(Conf)
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		cf := new(ConfigFile)
		cf.val = reflect.ValueOf(&Conf).Elem().Field(i)
		if err := cf.Add(dir, field.Name, User, nil, 0600, cf); err != nil {
			panic(err)
		}
	}
}
