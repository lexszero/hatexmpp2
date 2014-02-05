package main

import (
	"bytes"
	"code.google.com/p/go9p/p"
	"code.google.com/p/go9p/p/srv"
	"fmt"
	"io"
	"log"
	"os"
	"reflect"
)

func Must(err error) {
	if err != nil {
		panic(err)
	}
}

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

type RamBuffer struct {
	bytes.Buffer
}

func (b *RamBuffer) ReadAt(p []byte, off int64) (n int, err error) {
	if int(off) > b.Buffer.Len() {
		return 0, os.ErrInvalid
	}
	n = b.Buffer.Len() - int(off)
	if n > len(p) {
		n = len(p)
	}
	copy(p, b.Buffer.Bytes()[int(off):int(off)+n])
	return
}

type ReadRequest struct {
	p      []byte
	off    uint64
	result chan ReadResult
	fid    *srv.Fid
}

type ReadResult struct {
	n   int
	err error
}

type Appender interface {
	Append([]byte) (int, error)
}

type AppendingWriter struct {
	Appender
}

func (wr AppendingWriter) Write(b []byte) (int, error) {
	return wr.Append(b)
}

// History is something that we could read anything from, but only append to.
type History interface {
	Len() int
	io.ReaderAt
}

// A wrapper that allows access to a History by multiple clients
// TODO: maybe some more generic FileIO wrapper?
type FileHistory struct {
	srv.File
	History
	Writer io.Writer
	writer    io.Writer
	reads     chan ReadRequest
	writes    chan []byte
	cancels   chan *srv.Fid
}

func NewFileHistory(wr io.Writer) *FileHistory {
	b := &FileHistory{
		History: new(RamBuffer),
		writer:  wr,
		reads:   make(chan ReadRequest),
		writes:  make(chan []byte),
		cancels: make(chan *srv.Fid),
	}
	b.Writer = AppendingWriter{b}
	go func() {
		reads := make(map[*srv.Fid]ReadRequest)
		for {
			select {
			case r := <-b.reads:
				if !b.tryReadAt(&r) {
					reads[r.fid] = r
					Srv.Flushers[r.fid] = b
				}
			case data := <-b.writes:
				if wr, ok := b.History.(io.Writer); ok {
					wr.Write(data)
					for f, r := range reads {
						if b.tryReadAt(&r) {
							delete(reads, f)
						}
					}
				}
			case f := <-b.cancels:
				log.Printf("cancel fid=%v", f)
				delete(reads, f)
				if len(reads) == 0 {
					delete(Srv.Flushers, f)
				}
			}
		}
	}()
	return b
}

func (f *FileHistory) tryReadAt(r *ReadRequest) bool {
	if int(r.off) >= f.History.Len() {
		return false
	}
	n, err := f.History.ReadAt(r.p, int64(r.off))
	r.result <- ReadResult{n, err}
	return true
}

func (f *FileHistory) Flush(fid *srv.Fid) {
	f.cancels <- fid
}

func (f *FileHistory) Wstat(fid *srv.FFid, dir *p.Dir) error {
	return nil
}

func (f *FileHistory) Read(fid *srv.FFid, buf []byte, offset uint64) (int, error) {
	ch := make(chan ReadResult)
	f.reads <- ReadRequest{buf, offset, ch, fid.Fid}
	resp := <-ch
	return resp.n, resp.err
}

func (f *FileHistory) Write(fid *srv.FFid, buf []byte, offset uint64) (int, error) {
	log.Print("FileHistory.Write")
	return f.writer.Write(buf)
}

func (f *FileHistory) Append(buf []byte) (n int, err error) {
	log.Print("FileHistory.Append")
	if _, ok := f.History.(io.Writer); ok {
		f.writes <- buf
		return len(buf), nil
	}
	return 0, srv.Eperm
}
