package app

import (
	"fmt"
	"math/rand"
	"time"
)


type Hub struct {
	connections			[]Connection
	pending_closures	[]int
	incoming_messages	[]Message
}


func (self *Hub) RegisterNewConnections() {

	for {

		select {

		case inc := <- new_conn_chan:

			var c = Connection{
						Conn:			inc.Conn,
						Cid:			inc.Cid,
						InChan:			inc.InChan,
						OutChan:		inc.OutChan,
						Authenticated:	false}

			self.connections = append(self.connections, c)

		default:

			return

		}
	}
}


func (self *Hub) GetIncomingMessages() {

	LoopOverConnections:

	for _, c := range self.connections {
		for {
			select {
			case msg, ok := <- c.InChan:
				if ok {
					self.incoming_messages = append(self.incoming_messages, msg)
				} else {
					self.pending_closures = append(self.pending_closures, c.Cid)
					continue LoopOverConnections
				}
			default:
				continue LoopOverConnections
			}
		}
	}

	return
}


func (self *Hub) HandleClosures() {

	for _, id := range self.pending_closures {
		for i := len(self.connections) - 1; i >= 0; i-- {
			if self.connections[i].Cid == id {
				self.connections[i].Conn.Close()
				close(self.connections[i].OutChan)		// This must only happen once.
				fmt.Printf("hub() has registered the closure of connection %d.\n", self.connections[i].Cid)
				self.connections = append(self.connections[:i], self.connections[i + 1:]...)
				break
			}
		}
	}

	self.pending_closures = nil
}


func (self *Hub) HandleMessages() {

	for _, m := range self.incoming_messages {
		fmt.Printf("%d: %s: %s\n", m.Cid, m.Type, m.Content)
	}

	self.incoming_messages = nil
}


func (self *Hub) Spin() {

	for {

		self.RegisterNewConnections()
		self.GetIncomingMessages()
		self.HandleClosures()
		self.HandleMessages()

		i := rand.Intn(20)
		if i < len(self.connections) {
			self.connections[i].OutChan <- Message{Type: "debug", Content: fmt.Sprintf("Randomly generated message. Connection count: %d", len(self.connections))}
		}

		time.Sleep(50 * time.Millisecond)
	}
}
