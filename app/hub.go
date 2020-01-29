package app

import (
	"fmt"
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

	for _, cid := range self.pending_closures {
		self.CloseConnection(cid)
	}

	self.pending_closures = nil
}

func (self *Hub) CloseConnection(cid int) {

	// Close the actual underlying websocket conn.
	// Close the OutChan (results in the writer goroutine stopping).
	// Remove connection from our list of connections.

	for i := len(self.connections) - 1; i >= 0; i-- {
		if self.connections[i].Cid == cid {

			self.connections[i].Conn.Close()
			close(self.connections[i].OutChan)
			self.connections = append(self.connections[:i], self.connections[i + 1:]...)

			fmt.Printf("Hub has registered the closure of connection %d.\n", cid)
			return
		}
	}
}

func (self *Hub) HandleMessages() {

	for _, m := range self.incoming_messages {
		self.SendGlobally(fmt.Sprintf("Client %d: %s", m.Cid, m.Content))
	}

	self.incoming_messages = nil
}

func (self *Hub) SendGlobally(msg string) {
	for _, c := range self.connections {
		c.OutChan <- msg
	}
}

func (self *Hub) Spin() {

	for {

		self.RegisterNewConnections()
		self.GetIncomingMessages()
		self.HandleClosures()
		self.HandleMessages()

		time.Sleep(50 * time.Millisecond)
	}
}
