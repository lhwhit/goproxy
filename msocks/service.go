package msocks

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/shell909090/goproxy/sutils"
	"io"
	"net"
	"os"
	"strings"
)

type MsocksService struct {
	userpass map[string]string
	dialer   sutils.Dialer
}

func LoadPassfile(filename string) (userpass map[string]string, err error) {
	log.Notice("load passfile from file %s.", filename)

	file, err := os.Open(filename)
	if err != nil {
		log.Error("%s", err)
		return
	}
	defer file.Close()
	userpass = make(map[string]string, 0)

	reader := bufio.NewReader(file)
QUIT:
	for {
		line, err := reader.ReadString('\n')
		switch err {
		case io.EOF:
			if len(line) == 0 {
				break QUIT
			}
		case nil:
		default:
			return nil, err
		}
		f := strings.SplitN(line, ":", 2)
		if len(f) < 2 {
			err = fmt.Errorf("format wrong: %s", line)
			log.Error("%s", err)
			return nil, err
		}
		userpass[strings.Trim(f[0], "\r\n ")] = strings.Trim(f[1], "\r\n ")
	}

	log.Info("userinfo loaded %d record(s).", len(userpass))
	return
}

func NewService(auth map[string]string, dialer sutils.Dialer) (ms *MsocksService, err error) {
	if dialer == nil {
		err = errors.New("empty dialer")
		log.Error("%s", err)
		return
	}
	ms = &MsocksService{dialer: dialer}

	if auth != nil {
		ms.userpass = auth
	}
	return
}

func (ms *MsocksService) on_auth(stream io.ReadWriteCloser) bool {
	f, err := ReadFrame(stream)
	if err != nil {
		log.Error("%s", err)
		return false
	}

	ft, ok := f.(*FrameAuth)
	if !ok {
		log.Error("unexpected package type")
		return false
	}

	log.Notice("auth with username: %s, password: %s.",
		ft.Username, ft.Password)
	if ms.userpass != nil {
		password1, ok := ms.userpass[ft.Username]
		if !ok || (ft.Password != password1) {
			log.Error("auth failed.")
			fb := NewFrameFAILED(ft.Streamid, ERR_AUTH)
			buf, err := fb.Packed()
			_, err = stream.Write(buf.Bytes())
			if err != nil {
				log.Error("%s", err)
				return false
			}
			return false
		}
	}
	fb := NewFrameOK(ft.Streamid)
	buf, err := fb.Packed()
	if err != nil {
		log.Error("%s", err)
		return false
	}
	_, err = stream.Write(buf.Bytes())
	if err != nil {
		log.Error("%s", err)
		return false
	}

	log.Info("auth passed.")
	return true
}

func (ms *MsocksService) Handler(conn net.Conn) {
	log.Notice("connection come from: %s => %s.",
		conn.RemoteAddr(), conn.LocalAddr())

	if !ms.on_auth(conn) {
		conn.Close()
		return
	}

	sess := NewSession(conn)
	sess.dialer = ms.dialer
	sess.Run()
	log.Notice("server session %p quit: %s => %s.",
		sess, conn.RemoteAddr(), conn.LocalAddr())
}

func (ms *MsocksService) Serve(listener net.Listener) (err error) {
	var conn net.Conn

	for {
		conn, err = listener.Accept()
		if err != nil {
			log.Error("%s", err)
			return
		}
		go func() {
			defer conn.Close()
			ms.Handler(conn)
		}()
	}
	return
}
