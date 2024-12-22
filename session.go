package relay

import (
	"sync"
	"sync/atomic"

	"github.com/gorilla/websocket"
)

type Session struct {
	id     int64
	conn   *websocket.Conn
	closed atomic.Bool
	lock   sync.Mutex
}

func (s *Session) ID() int64 {
	return s.id
}

func (s *Session) Send(msg []byte) error {
	if s.closed.Load() {
		return ErrConnectionClosed
	}

	s.lock.Lock()
	err := s.conn.WriteMessage(websocket.TextMessage, msg)
	s.lock.Unlock()
	return err
}

func (s *Session) Close() {
	s.conn.Close()
}

func (s *Session) WithConn(handler func(conn *websocket.Conn)) {
	s.lock.Lock()
	handler(s.conn)
	s.lock.Unlock()
}
