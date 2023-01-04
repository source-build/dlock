package dlock

import (
	"errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"net"
	"time"
)

func Run() {
	listen, err := net.Listen("tcp", "0.0.0.0:"+viper.GetString("server.port"))
	if err != nil {
		logger.WithFields(logrus.Fields{"type": "init", "theme": "run", "err": err}).Error()
		return
	}
	defer listen.Close()

	for {
		conn, err := listen.Accept()
		if err != nil {
			logger.WithFields(logrus.Fields{"type": "runtime", "theme": "run", "err": err}).Error()
			continue
		}
		go newConn(conn)
	}
}

func newConn(conn net.Conn) {
	srv := newServe(conn)
	defer srv.close()
	err := srv.authentication()
	if err != nil {
		return
	}

	buf := make([]byte, 256)
	nd := newNode(conn)
	nd.status = onlineStatus
	for {
		if err := conn.SetReadDeadline(time.Now().Add(time.Second * 60)); err != nil {
			return
		}
		n, err := conn.Read(buf)
		if err != nil {
			nd.status = offlineStatus
			nd.discard = true
			nd.unLockProcess()
			nd.quit()
			return
		}
		rest := buf[:n]
		go nd.Handler(rest)
	}
}

type serve struct {
	conn net.Conn
}

func newServe(conn net.Conn) *serve {
	return &serve{conn: conn}
}

func (s *serve) authentication() error {
	buf := make([]byte, 256)
	if err := s.conn.SetReadDeadline(time.Now().Add(time.Second * 10)); err != nil {
		return err
	}
	n, err := s.conn.Read(buf)
	if err != nil {
		logger.WithFields(logrus.Fields{"type": "auth", "theme": "read", "err": err}).Error()
		return err
	}
	event, body, err := decode(buf[:n])
	if err != nil {
		if body, err := encode(decodeFailEvent); err == nil {
			_, _ = s.conn.Write(body)
		}
		return err
	}
	if event != authEvent {
		if body, err := encode(typeErrorEvent); err == nil {
			_, _ = s.conn.Write(body)
		}
		return errors.New("unrecognized packet")
	}
	if string(body) != viper.GetString("server.secretKey") {
		if body, err := encode(authFailEvent); err == nil {
			_, _ = s.conn.Write(body)
		}
		return errors.New("there is a problem with the data package")
	}
	if body, err := encode(authOKEvent); err == nil {
		_, _ = s.conn.Write(body)
	}
	return nil
}

func (s *serve) close() {
	err := s.conn.Close()
	if err != nil {
		logger.WithFields(logrus.Fields{"type": "close", "theme": "conn close", "err": err}).Error()
	}
}
