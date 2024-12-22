package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"relay"

	"github.com/gorilla/websocket"
)

type Message struct {
	Timestamp int64
	Sender    string
	Text      string
}

func createExchange() *relay.Exchange {
	upgrader := websocket.Upgrader{
		EnableCompression: true,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	exchange := relay.NewExchange()

	upgradeFunc := func(w http.ResponseWriter, r *http.Request) (*websocket.Conn, error) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return nil, err
		}

		conn.SetReadDeadline(time.Now().Add(time.Hour * 999))

		return conn, nil
	}

	exchange.OnUpgrade(upgradeFunc)

	exchange.OnConnect(func(s *relay.Session) error {
		log.Println("connection opened", s.ID())

		msg := &Message{
			Sender:    "system",
			Timestamp: time.Now().UnixMilli(),
			Text:      "connected",
		}

		data, err := json.Marshal(msg)
		if err != nil {
			return err
		}

		return s.Send(data)
	})

	exchange.OnMessage(func(s *relay.Session, msg []byte) error {
		message := &Message{}

		err := json.Unmarshal(msg, message)
		if err != nil {
			return err
		}

		message.Timestamp = time.Now().UnixMilli()

		data, err := json.Marshal(message)
		if err != nil {
			return err
		}

		exchange.Broadcast(nil, data)

		return nil
	})

	exchange.OnClose(func(s *relay.Session) error {
		log.Println("connection closed", s.ID())
		return nil
	})

	return exchange
}

func main() {
	mux := http.NewServeMux()
	exchange := createExchange()

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./public/index.html")
	})

	mux.HandleFunc("GET /ws", func(w http.ResponseWriter, r *http.Request) {
		exchange.ManageConnection(w, r)
	})

	log.Println("server started at: http://127.0.0.1:8080")

	http.ListenAndServe(":8080", mux)
}
