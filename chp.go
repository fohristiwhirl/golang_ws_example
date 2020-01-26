package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"time"

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

type ConnIdGenerator struct {
	val				int
}

func (self *ConnIdGenerator) Next() int {
	self.val += 1
	return self.val
}

var upgrader = websocket.Upgrader{CheckOrigin: check_origin}
var conn_id_generator = ConnIdGenerator{}
var new_conn_chan = make(chan NewConnection, 64)

func check_origin(r *http.Request) bool {			// FIXME
	return true
}

func hub() {

	// Note that this must not directly read or write messages to the connections.
	// Rather, the read_loop() and write_loop() goroutines do that.

	var connections []Connection
	var pending_closures []int
	var incoming_messages []Message

	for {

		// Register new connections...

		ConnectLoop:
		for {

			select {

			case inc := <- new_conn_chan:

				var c = Connection{
							Conn:			inc.Conn,
							Cid:			inc.Cid,
							InChan:			inc.InChan,
							OutChan:		inc.OutChan,
							Authenticated:	false}

				connections = append(connections, c)

			default:
				break ConnectLoop
			}
		}

		// Learn about incoming messages. We may also learn of connections that
		// were closed by the client (in which case InChan will have been closed).

		LoopOverConnections:
		for _, c := range connections {
			for {
				select {
				case msg, ok := <- c.InChan:
					if ok {
						incoming_messages = append(incoming_messages, msg)
					} else {
						pending_closures = append(pending_closures, c.Cid)
						continue LoopOverConnections
					}
				default:
					continue LoopOverConnections
				}
			}
		}

		// Actually deal with the incoming messages...
		// Don't assume the client responsible for the message is still connected.

		for _, m := range incoming_messages {
			fmt.Printf("%d: %s: %s\n", m.Cid, m.Type, m.Content)
		}

		incoming_messages = nil

		// Finalise the closure of dead or closing connections...

		for _, id := range pending_closures {
			for i := len(connections) - 1; i >= 0; i-- {
				if connections[i].Cid == id {
					connections[i].Conn.Close()
					close(connections[i].OutChan)		// This must only happen once.
					fmt.Printf("hub() has registered the closure of connection %d.\n", connections[i].Cid)
					connections = append(connections[:i], connections[i + 1:]...)
					break
				}
			}
		}

		pending_closures = nil

		// Do whatever else we need to do...

		i := rand.Intn(20)
		if i < len(connections) {
			connections[i].OutChan <- Message{Type: "debug", Content: fmt.Sprintf("Randomly generated message. Connection count: %d", len(connections))}
		}

		// If we ever want to close a connection here, it's as follows. Note that
		// messages added to the OutChan just prior to this likely won't make it.
		//
		// pending_closures = append(pending_closures, connections[i].Id)

		time.Sleep(50 * time.Millisecond)
	}
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
			close(msg_to_hub)				// Lets the hub know the connection is closed.
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

		if ok == false {					// Channel was closed by hub(). We are done.
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

func main() {

	go hub()

	http.HandleFunc("/", handler)
	http.ListenAndServe("127.0.0.1:8080", nil)
}
