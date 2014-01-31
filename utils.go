package main

import (
	"fmt"
	"log"
	"reflect"
	"code.google.com/p/go9p/p"
	"code.google.com/p/go9p/p/srv"
)

func Must(err error) {
	if err != nil {
		panic(err)
	}
}

type FRead func(fid *srv.FFid, buf []byte, offset uint64) (int, error)
type FWrite func(fid *srv.FFid, data []byte, offset uint64) (int, error)

type FilePrint struct {
	srv.File
	val reflect.Value
}

func (f *FilePrint) Wstat(fid *srv.FFid, dir *p.Dir) error {
	return nil
}

func (f *FilePrint) Read(fid *srv.FFid, buf []byte, offset uint64) (int, error) {
	f.Lock()
	defer f.Unlock()

	log.Print("read")
	b := []byte(fmt.Sprint(f.val.Interface()))
	have := len(b)
	off := int(offset)
	if off >= have {
		return 0, nil
	}
	return copy(buf, b[off:]), nil
	return 0, srv.Eperm
}

type FilePrintScan struct {
	FilePrint
}

func (f *FilePrintScan) Write(fid *srv.FFid, buf []byte, offset uint64) (n int, err error) {
	f.Lock()
	defer f.Unlock()

	s := string(buf)
	switch f.val.Kind() {
	case reflect.String:
		f.val.SetString(s)
	default:
		_, err = fmt.Sscan(s, f.val.Addr().Interface())
	}
	n = len(buf)
	return
}
