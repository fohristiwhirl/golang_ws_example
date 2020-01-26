package app

import (
	"github.com/gorilla/websocket"
)

type Message struct {
	Cid					int					`json:"-"`
	Type				string				`json:"type"`
	Content				string				`json:"content"`
}

type NewConnection struct {					// Contains the minimum info needed to register a new connection.
	Conn				*websocket.Conn
	Cid					int
	InChan				chan Message
	OutChan				chan Message
}

type Connection struct {					// This could contain additional state as needed.
	Conn				*websocket.Conn
	Cid					int
	InChan				chan Message
	OutChan				chan Message
	Authenticated		bool
}
