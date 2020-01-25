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
	InChan		chan *Message		// Chan on which incoming messages are placed.
	OutChan		chan *Message		// Chan on which outgoing messages are placed.
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

var new_conn_chan = make(chan *Connection, 64)
var dead_conn_chan = make(chan *Connection, 64)

func check_origin(r *http.Request) bool {			// FIXME
	return true
}

func hub() {

	var connections []*Connection

	for {

		// Deal with new / closed connections...
		// Note that any closed connections need to have their OutChan closed, which allows handler() to return.

		ConnectDisconnectLoop:
		for {
			select {
			case new_conn := <- new_conn_chan:
				connections = append(connections, new_conn)
				new_conn.OutChan <- &Message{Type: "debug", Content: fmt.Sprintf("Hello client %d", new_conn.Id)}
			case dead_conn := <- dead_conn_chan:
				for i := len(connections) - 1; i >= 0; i-- {
					if connections[i] == dead_conn {
						connections = append(connections[:i], connections[i + 1:]...)
						close(dead_conn.OutChan)		// See note above.
					}
				}
			default:
				break ConnectDisconnectLoop
			}
		}

		// Deal with any incoming messages...

		for _, conn_info := range connections {
			IncomingMessageLoop:
			for {
				select {
				case msg := <- conn_info.InChan:
					fmt.Printf("%d: %s: %s\n", conn_info.Id, msg.Type, msg.Content);
				default:
					break IncomingMessageLoop
				}
			}
		}

		// Do whatever else we need to do...

		i := rand.Intn(20)
		if i < len(connections) {
			connections[i].OutChan <- &Message{Type: "debug", Content: fmt.Sprintf("Randomly generated message")}
		}

		time.Sleep(50 * time.Millisecond)
	}
}

func handler(w http.ResponseWriter, r *http.Request) {

	// This function notifies hub() about the new connection. It also
	// monitors a channel and passes on outgoing messages to the client.

	// This function must only return if the outgoing message channel is closed.

	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Printf("%v\n", err)
		return
	}

	conn_info := Connection{
		Conn:		c,
		Id:			conn_id_generator.Next(),
		InChan:		make(chan *Message, 64),		// Chan on which incoming messages are placed.
		OutChan:	make(chan *Message, 64),		// Chan on which outgoing messages are placed.
	}
	defer conn_info.Conn.Close()

	go read_loop(&conn_info)
	new_conn_chan <- &conn_info

	// It's convenient to actually send outgoing messages in this
	// function as it simplifies the logic of hub().

	RelayOutGoingMessages:
	for {
		select {

		case msg, ok := <- conn_info.OutChan:

			if ok == false {						// Channel is closed. We can return.
				return
			}

			b, err := json.Marshal(msg)
			if err != nil {
				fmt.Printf("%v\n", err)
			} else {
				err = c.WriteMessage(websocket.TextMessage, b)
				if err != nil {
					fmt.Printf("%v\n", err)
					break RelayOutGoingMessages		// Presumably, client disconnected.
				}
			}

		default:
			time.Sleep(50 * time.Millisecond)
		}
	}

	// Inform hub() that a WriteMessage() failed. Continue reading messages on the outgoing
	// channel until it's closed by the hub, for the sake of ensuring there's no deadlock...

	dead_conn_chan <- &conn_info

	for {
		_, ok := <- conn_info.OutChan
		if ok == false {
			break
		}
	}
}

func read_loop(conn_info *Connection) {

	for {

		_, b, err := conn_info.Conn.ReadMessage()

		if err != nil {								// Presumably, client disconnected.
			dead_conn_chan <- conn_info
			return
		}

		msg := new(Message)
		err = json.Unmarshal(b, msg)

		if err != nil {
			fmt.Printf("%v\n", err)
		} else {
			conn_info.InChan <- msg
		}
	}
}

func main() {

	go hub()

	http.HandleFunc("/", handler)
	http.ListenAndServe("127.0.0.1:8080", nil)
}
