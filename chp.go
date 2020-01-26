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
	Type		string				`json:"type"`
	Content		string				`json:"content"`
}

type Connection struct {
	Conn		*websocket.Conn
	Id			int
	InChan		chan Message
	OutChan		chan Message
}

type ConnIdGenerator struct {
	val			int
}

func (self *ConnIdGenerator) Next() int {
	self.val += 1
	return self.val
}

var upgrader = websocket.Upgrader{CheckOrigin: check_origin}
var conn_id_generator = ConnIdGenerator{}
var new_conn_chan = make(chan Connection, 64)

func check_origin(r *http.Request) bool {			// FIXME
	return true
}

func hub() {

	// Note that this must not directly read or write messages to the connections.
	// Rather, the read_loop() and write_loop() goroutines do that.

	var connections []Connection
	var pending_closures []int

	for {

		// Register new connections...

		ConnectLoop:
		for {
			select {
			case new_conn := <- new_conn_chan:
				connections = append(connections, new_conn)
			default:
				break ConnectLoop
			}
		}

		// Deal with any incoming messages. We may also learn of connections that
		// were closed by the client (in which case InChan will have been closed).

		LoopOverConnections:
		for _, conn_info := range connections {
			for {
				select {
				case msg, ok := <- conn_info.InChan:
					if ok {
						fmt.Printf("%d: %s: %s\n", conn_info.Id, msg.Type, msg.Content);
					} else {
						pending_closures = append(pending_closures, conn_info.Id)
						continue LoopOverConnections
					}
				default:
					continue LoopOverConnections
				}
			}
		}

		// Finalise the closure of dead or closing connections...

		for _, id := range pending_closures {
			for i := len(connections) - 1; i >= 0; i-- {
				if connections[i].Id == id {
					connections[i].Conn.Close()
					close(connections[i].OutChan)		// This must only happen once.
					fmt.Printf("hub() has registered the closure of connection %d.\n", connections[i].Id)
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

	in_chan := make(chan Message, 64)
	out_chan := make(chan Message, 64)

	new_conn_chan <- Connection{Conn: c, Id: conn_id_generator.Next(), InChan: in_chan, OutChan: out_chan}

	// Connections support one concurrent reader and one concurrent writer.

	go read_loop(c, in_chan)
	go write_loop(c, out_chan)
}

func read_loop(c *websocket.Conn, incoming_messages chan Message) {

	for {
		_, b, err := c.ReadMessage()

		if err != nil {
			close(incoming_messages)				// Lets the hub know the connection is closed.
			return
		}

		var msg Message
		err = json.Unmarshal(b, &msg)

		if err != nil {
			fmt.Printf("%v\n", err)
		} else {
			incoming_messages <- msg
		}
	}
}

func write_loop(c *websocket.Conn, outgoing_messages chan Message) {

	for {
		msg, ok := <- outgoing_messages

		if ok == false {							// Channel was closed by hub(). We are done.
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
