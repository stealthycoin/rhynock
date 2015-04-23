package rhynock

// A type to hold a message and the sender of that message
type Bottle struct {
	Sender *Conn
	Message []byte
}


// An interface to define a type that can recieve a bottle
// And can be told when a connection has been closed
type BottleDst interface {
	GetBottleChan() (chan *Bottle)
	ConnectionClosed(*Conn)
	ConnectionOpened(*Conn)
}
