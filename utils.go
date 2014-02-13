package main

import (
	"bytes"
	"cjones.org/hg/go-xmpp2.hg/xmpp"
	"code.google.com/p/go9p/p"
	"code.google.com/p/go9p/p/srv"
	"fmt"
	"io"
	"log"
	"os"
	"reflect"
	"strconv"
	"strings"
)

func Must(err error) {
	if err != nil {
		panic(err)
	}
}

func MustVal(x interface{}, err error) interface{} {
	Must(err)
	return x
}

func ConcatText(t []xmpp.Text) string {
	s := make([]string, len(t))
	for i := range t {
		s[i] = string(t[i].Chardata)
	}
	return strings.Join(s, "\n")
}

func MaybeSetData(s *string, d *xmpp.Data) {
	if s == nil || d == nil {
		return
	}
	*s = d.Chardata
}

func MaybeSetText(s *string, t []xmpp.Text) {
	if s == nil || t == nil {
		return
	}
	*s = ConcatText(t)
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

	b := []byte(fmt.Sprint(f.val.Elem().Interface()))
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
		_, err = fmt.Sscan(s, f.val.Interface())
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

type LogFile struct {
	*os.File
}

func NewLogFile(name string) *LogFile {
	return &LogFile{MustVal(os.OpenFile(fmt.Sprintf("%s/%s", Conf.LogDir, name), os.O_RDWR|os.O_APPEND|os.O_CREATE, 0644)).(*os.File)}
}

func (l *LogFile) Len() int {
	fi := MustVal(l.Stat()).(os.FileInfo)
	return int(fi.Size())
}

func (l *LogFile) ReadAt(p []byte, off int64) (n int, err error) {
	sz := l.Len()
	if int(off) > sz {
		return 0, os.ErrInvalid
	}
	n = sz - int(off)
	if n > len(p) {
		n = len(p)
	}
	return l.File.ReadAt(p[:n], off)
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
	Writer  io.Writer
	writer  io.Writer
	reads   chan ReadRequest
	writes  chan []byte
	cancels chan *srv.Fid
	stop    chan bool
}

func NewFileHistory(wr io.Writer, h History) *FileHistory {
	b := &FileHistory{
		History: h,
		writer:  wr,
		reads:   make(chan ReadRequest),
		writes:  make(chan []byte),
		cancels: make(chan *srv.Fid),
		stop:    make(chan bool),
	}
	if b.History == nil {
		b.History = new(RamBuffer)
	}
	b.Writer = AppendingWriter{b}
	go func() {
		reads := make(map[*srv.Fid]ReadRequest)
Loop:
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
				delete(reads, f)
				if len(reads) == 0 {
					delete(Srv.Flushers, f)
				}
			case <-b.stop:
				for f, r := range reads {
					r.result <- ReadResult{0, io.EOF}
					delete(reads, f)
					delete(Srv.Flushers, f)
				}
				break Loop
			}
		}
		close(b.reads)
		close(b.writes)
		close(b.cancels)
		close(b.stop)
	}()
	return b
}

func (f *FileHistory) Stop() {
	f.stop <- true
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
	if f.writer != nil {
		return f.writer.Write(buf)
	}
	return len(buf), nil
}

func (f *FileHistory) Append(buf []byte) (n int, err error) {
	if _, ok := f.History.(io.Writer); ok {
		f.writes <- buf
		return len(buf), nil
	}
	return 0, srv.Eperm
}

func NewFileChat(name string, wr io.Writer) *FileHistory {
	if Conf.LogDir != "" {
		return NewFileHistory(wr, NewLogFile(name))
	}
	return NewFileHistory(wr, nil)
}

type tag9p map[string]string
func tag9pParse(str reflect.StructField) (t tag9p) {
	t = make(tag9p)
	ss := strings.Split(str.Tag.Get("9p"), ",")
	for _, s := range ss {
		kv := strings.Split(s, "=")
		if len(kv) == 2 {
			t[kv[0]] = kv[1]
		}
		if len(kv) == 1 && kv[0] != "" {
			t[kv[0]] = "true"
		}
	}
	return
}

func (t tag9p) IsSet(name string) bool {
	_, r := t[name]
	return r
}

func fileRecursiveAddTV(parent *srv.File, t reflect.Type, v reflect.Value, name string, mode uint32) (err error) {
	switch t.Kind() {
	case reflect.Ptr:
		return fileRecursiveAddTV(parent, t.Elem(), v.Elem(), name, mode)
	case reflect.String, reflect.Int:
		var f interface{}
		if mode & 0222 != 0 {
			f = &FilePrintScan{FilePrint: FilePrint{val: v}}
		} else {
			f = &FilePrint{val: v}
		}
		fFile := reflect.ValueOf(f).Elem().FieldByName("File").Addr().Interface().(*srv.File)
		return fFile.Add(parent, name, User, Group, mode, f)
	case reflect.Struct:
		dir := v.Interface()
		dirFile := v.Elem().FieldByName("File").Addr().Interface().(*srv.File)
		if err = dirFile.Add(parent, name, User, Group, mode, dir); err != nil {
			return
		}
		if mode & p.DMDIR == 0 {
			return
		}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if f.Name == "File" {
				continue
			}
			fTyp := f.Type
			fVal := v.Elem().Field(i)
			if fTyp.Kind() == reflect.Ptr {
				fTyp = fTyp.Elem()
				fVal = fVal.Elem()
			}
			tag := tag9pParse(f)
			if tag.IsSet("-") {
				continue
			}
			fName := strings.ToLower(f.Name)
			if s, ok := tag["name"]; ok {
				fName = s
			}
			fMode := uint32(0600)
			if s, ok := tag["mode"]; ok {
				fMode = uint32(MustVal(strconv.ParseUint(s, 8, 32)).(uint64))
			}
			if f.Type.Kind() == reflect.Struct && f.Tag.Get("nodir") == "" {
				fMode |= p.DMDIR
			}
			Must(fileRecursiveAddTV(dirFile, fTyp, fVal.Addr(), fName, fMode))
		}
		return
	}
	return srv.Enotimpl
}

func FileRecursiveAdd(parent *srv.File, x interface{}, name string, mode uint32) (err error) {
	xt := reflect.TypeOf(x)
	xv := reflect.ValueOf(x)
	if xt.Kind() == reflect.Ptr {
		xt = xt.Elem()
	}
	return fileRecursiveAddTV(parent, xt, xv, name, mode)
}
