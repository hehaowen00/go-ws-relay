package relay

import (
	"log"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/gorilla/websocket"
)

type Exchange struct {
	clients   map[int64]*Session
	lock      sync.RWMutex
	idCounter atomic.Int64

	upgradeHandler func(w http.ResponseWriter, r *http.Request) (*websocket.Conn, error)
	connectHandler func(s *Session) error
	messageHandler func(s *Session, msg []byte) error
	closeHandler   func(s *Session) error
}

func NewExchange() *Exchange {
	return &Exchange{
		clients: map[int64]*Session{},
	}
}

func (ex *Exchange) OnUpgrade(
	callback func(w http.ResponseWriter, r *http.Request) (*websocket.Conn, error),
) {
	ex.upgradeHandler = callback
}

func (ex *Exchange) OnConnect(
	callback func(s *Session) error,
) {
	ex.connectHandler = callback
}

func (ex *Exchange) OnMessage(
	callback func(s *Session, msg []byte) error,
) {
	ex.messageHandler = callback
}

func (ex *Exchange) OnClose(
	callback func(s *Session) error,
) {
	ex.closeHandler = callback
}

func (ex *Exchange) ManageConnection(w http.ResponseWriter, r *http.Request) {
	conn, err := ex.upgradeHandler(w, r)
	if err != nil {
		log.Println(err)
		return
	}
	defer conn.Close()

	session := ex.NewSession(conn)
	defer func() {
		ex.closeHandler(session)
		session.closed.Store(true)
		ex.lock.Lock()
		delete(ex.clients, session.ID())
		ex.lock.Unlock()
	}()

	err = ex.connectHandler(session)
	if err != nil {
		return
	}

	go func() {
		select {
		case <-r.Context().Done():
			session.closed.Store(true)
			return
		}
	}()

	for {
		if r.Context().Err() != nil {
			return
		}

		_, data, err := conn.ReadMessage()
		if err != nil {
			log.Println(err)
			return
		}

		err = ex.messageHandler(session, data)
		if err != nil {
			log.Println(err)
			continue
		}
	}
}

func (ex *Exchange) Send(id int64, msg []byte) error {
	ex.lock.RLock()
	session, ok := ex.clients[id]
	ex.lock.RUnlock()

	if ok {
		session.lock.Lock()
		err := session.conn.WriteMessage(websocket.TextMessage, msg)
		session.lock.Unlock()

		if err != nil {
			return err
		}
	}

	return nil
}

func (ex *Exchange) Broadcast(ids []int64, msg []byte) {
	ex.lock.RLock()

	if len(ids) == 0 {
		for id, session := range ex.clients {
			_ = id
			go session.Send(msg)
		}
	} else {
		for _, id := range ids {
			session, ok := ex.clients[id]
			if ok {
				go session.Send(msg)
			}
		}
	}

	ex.lock.RUnlock()
}

func (ex *Exchange) NewSession(conn *websocket.Conn) *Session {
	ex.lock.Lock()

	s := &Session{
		id:   ex.idCounter.Add(1),
		conn: conn,
	}
	s.closed.Store(false)
	ex.clients[s.id] = s

	ex.lock.Unlock()

	return s
}
