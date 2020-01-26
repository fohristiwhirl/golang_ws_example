package app

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/websocket"
)

type Message struct {
	Cid				int					`json:"-"`
	Type			string				`json:"type"`
	Content			string				`json:"content"`
}

type NewConnection struct {				// Contains the minimum info needed to register a new connection.
	Conn			*websocket.Conn
	Cid				int
	InChan			chan Message
	OutChan			chan Message
}

type Connection struct {				// This could contain additional state as needed.
	Conn			*websocket.Conn
	Cid				int
	InChan			chan Message
	OutChan			chan Message
	Authenticated	bool
}

var upgrader = websocket.Upgrader{CheckOrigin: check_origin}
var new_conn_chan = make(chan NewConnection, 64)

func check_origin(r *http.Request) bool {			// FIXME
	return true
}

func handler(w http.ResponseWriter, r *http.Request) {

	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Printf("%v\n", err)
		return
	}

	cid := conn_id_generator.Next()
	in_chan := make(chan Message, 64)
	out_chan := make(chan Message, 64)

	new_conn_chan <- NewConnection{Conn: c, Cid: cid, InChan: in_chan, OutChan: out_chan}

	// Connections support one concurrent reader and one concurrent writer.

	go read_loop(c, in_chan, cid)
	go write_loop(c, out_chan)
}

func read_loop(c *websocket.Conn, msg_to_hub chan Message, cid int) {

	for {
		_, b, err := c.ReadMessage()

		if err != nil {
			close(msg_to_hub)				// Lets the Hub know the connection is closed.
			return
		}

		var msg Message
		err = json.Unmarshal(b, &msg)

		if err != nil {
			fmt.Printf("%v\n", err)
		} else {
			msg.Cid = cid					// Make sure the message has the client's unique ID.
			msg_to_hub <- msg
		}
	}
}

func write_loop(c *websocket.Conn, msg_from_hub chan Message) {

	for {
		msg, ok := <- msg_from_hub

		if ok == false {					// Channel was closed by Hub. We are done.
			return
		}

		b, err := json.Marshal(msg)
		if err != nil {
			fmt.Printf("%v\n", err)
		} else {
			c.WriteMessage(websocket.TextMessage, b)
		}
	}
}
