package app

import (
	"fmt"
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{CheckOrigin: check_origin}
var new_conn_chan = make(chan NewConnection, 64)

func check_origin(r *http.Request) bool {			// FIXME
	return true
}

// When a new connection comes in, various things happen:
//
// - Information about the connection is sent to the Hub.
//
// - An output goroutine starts; it reads from a channel and relays messages to the client.
//   The Hub can close the channel to cause this goroutine to exit.
//
// - An input goroutine starts; it reads from the client and sends messages into a channel.
//   The Hub should read these messages. The goroutine closes the channel and exits if the
//   connection is closed.

func handler(w http.ResponseWriter, r *http.Request) {

	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Printf("%v\n", err)
		return
	}

	cid := conn_id_generator.Next()
	in_chan := make(chan Message, 64)
	out_chan := make(chan string, 64)

	new_conn_chan <- NewConnection{Conn: c, Cid: cid, InChan: in_chan, OutChan: out_chan}

	// Connections support one concurrent reader and one concurrent writer.

	go read_loop(c, in_chan, cid)
	go write_loop(c, out_chan)
}

func read_loop(c *websocket.Conn, msg_to_hub chan Message, cid int) {

	for {
		_, b, err := c.ReadMessage()

		if err != nil {
			close(msg_to_hub)
			return
		}

		msg_to_hub <- Message{Cid: cid, Content: string(b)}
	}
}

func write_loop(c *websocket.Conn, msg_from_hub chan string) {

	for {
		msg, ok := <- msg_from_hub

		if ok == false {					// Channel was closed by Hub. We are done.
			return
		}

		c.WriteMessage(websocket.TextMessage, []byte(msg))
	}
}
