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
		// Note that any closed connections need to have their OutChan
		// closed so that absorb_remaining_outgoing() can return.

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
						dead_conn.Conn.Close()
						close(dead_conn.OutChan)		// See note above re absorb_remaining_outgoing() function
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
			connections[i].OutChan <- &Message{Type: "debug", Content: fmt.Sprintf("Randomly generated message. Connection count: %d", len(connections))}
		}

		time.Sleep(50 * time.Millisecond)
	}
}

func handler(w http.ResponseWriter, r *http.Request) {

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

	new_conn_chan <- &conn_info

	// Connections support one concurrent reader and one concurrent writer.

	go read_loop(&conn_info)
	go write_loop(&conn_info)
}

func read_loop(conn_info *Connection) {

	// This will generally be the function that spots the connection was closed by the client.

	for {
		_, b, err := conn_info.Conn.ReadMessage()

		if err != nil {
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

func write_loop(conn_info *Connection) {

	for {
		msg, ok := <- conn_info.OutChan

		if ok == false {							// Channel was closed by hub(). We can return.
			return
		}

		b, err := json.Marshal(msg)
		if err != nil {
			fmt.Printf("%v\n", err)
		} else {
			err = conn_info.Conn.WriteMessage(websocket.TextMessage, b)
			if err != nil {
				fmt.Printf("%v\n", err)
				dead_conn_chan <- conn_info
				absorb_remaining_outgoing(conn_info)
				return
			}
		}
	}
}

func absorb_remaining_outgoing(conn_info *Connection) {

	// Continue reading any messages from hub() that were intended
	// for a dead connection; this avoids deadlocks. It returns
	// when the OutChan is closed by hub().

	for {
		_, ok := <- conn_info.OutChan
		if ok == false {
			return
		}
	}
}

func main() {

	go hub()

	http.HandleFunc("/", handler)
	http.ListenAndServe("127.0.0.1:8080", nil)
}
