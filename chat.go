package main

import (
	"bytes"
	"io"
	"code.google.com/p/go9p/p"
	"code.google.com/p/go9p/p/srv"
	"log"
)

type ReadRequest struct {
	p []byte
	off uint64
	result chan ReadResult
	fid *srv.Fid
}

type ReadResult struct {
	n int
	err error
}

type ChanWriter chan []byte
func (c ChanWriter) Write(p []byte) (n int, err error) {
	c <- p
	return len(p), nil
}

type FileHistory struct {
	srv.File
	BufWriter ChanWriter
	Writer io.Writer
	buf bytes.Buffer
	reads chan ReadRequest
	cancels chan *srv.Fid
}

func NewFileHistory() *FileHistory {
	b := &FileHistory{
		reads: make(chan ReadRequest),
		BufWriter: make(ChanWriter),
		cancels: make(chan *srv.Fid),
	}
	go func() {
		reads := make(map[*srv.Fid]ReadRequest)
		for {
			select {
			case r := <-b.reads:
				if ! b.tryReadAt(&r) {
					reads[r.fid] = r
					Srv.Flushers[r.fid] = b
				}
			case data := <-b.BufWriter:
				b.buf.Write(data)
				for f, r := range reads {
					if b.tryReadAt(&r) {
						delete(reads, f)
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

func (b *FileHistory) tryReadAt(r *ReadRequest) bool {
	if r.off >= uint64(b.buf.Len()) {
		return false
	}
	n := b.buf.Len() - int(r.off)
	if (n > len(r.p)) {
		n = len(r.p)
	}
	copy(r.p, b.buf.Bytes()[int(r.off):int(r.off)+n])
	r.result <- ReadResult{n, nil}
	return true
}

func (b *FileHistory) Flush(f *srv.Fid) {
	b.cancels <- f
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

func (f *FileHistory) Write(fid *srv.FFid, buf []byte, offset uint64) (n int, err error) {
	if f.Writer != nil {
		return f.Writer.Write(buf)
	}
	return 0, srv.Eperm
}
