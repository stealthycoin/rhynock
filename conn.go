package rhynock

import (
	"github.com/gorilla/websocket"
	"net/http"
	"time"
	"log"
)

// Some defaults for pinging
// Needs to be settable from outside
const (
	writeWait = 10 * time.Second
	pongWait = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10
	maxMessageSize = 512
)

// Conn encapsulates our websocket
type Conn struct {
	// Exported so everything can be messed with from outside
	Ws      *websocket.Conn
	Send    chan []byte
	Dst     BottleDst
	Quit chan []byte
}

//
// Convenience function so you dont have to use the Send channel
//
func (c *Conn) SendMsg(message string) {
	// Basically just typecasting for convenience
	c.Send <- []byte(message)
}

//
// Convenience function to call the quit channel with a message
//
func (c *Conn) CloseMsg(message string) {
	c.Quit <- []byte(message)
}

//
// Used to write a single message to the client and report any errors
//
func (c *Conn) write(t int, payload []byte) error {
	c.Ws.SetWriteDeadline(time.Now().Add(writeWait))
	return c.Ws.WriteMessage(t, payload)
}

//
// Maintains both a reader and a writer, cleans up both if one fails
//
func (c *Conn) read_write() {
	// Ping timer
	ticker := time.NewTicker(pingPeriod)

	// Clean up Connection and Connection resources
	defer func() {
		ticker.Stop()
		c.Ws.Close()
	}()

	// Config websocket settings
	c.Ws.SetReadLimit(maxMessageSize)
	c.Ws.SetReadDeadline(time.Now().Add(pongWait))
	c.Ws.SetPongHandler(func(string) error {
		// Give each client pongWait seconds after the ping to respond
		c.Ws.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	// Start a reading goroutine
	// The reader will stop when the c.Ws.Close is called at
	// in the defered cleanup function, so we do not manually
	// have to close the reader
	go func() {
		for {
			// This blcoks until it reads EOF or an error
			// occurs trying to read, the error can be
			// used to detect when the client closes the Connection
			_, message, err := c.Ws.ReadMessage()
			if err != nil {
				break // If we get an error escape the loop
			}

			// Bottle the message with its sender
			bottle := &Bottle{
				Sender: c,
				Message: message,
			}

			// Send to the destination for processing
			c.Dst.GetBottleChan() <- bottle
		}
		// The reader has been terminated

	}()

	// Main handling loop
	for {
		select {
		case message, ok := <- c.Send:
			// Our send channel has something in it or the channel closed
			if !ok {
				// Our channel was closed, gracefully close socket Conn
				c.write(websocket.CloseMessage, []byte{})
				return
			}
			// Attempt to write the message to the websocket
			if err := c.write(websocket.TextMessage, message); err != nil {
				// If we get an error we can no longer communcate with client
				// return, no need to send CloseMessage since that would
				// just yield another error
				return
			}

		case <- ticker.C:
			// Ping ticker went off. We need to ping to check for connectivity.
			if err := c.write(websocket.PingMessage, []byte{}); err != nil {
				// We got an error pinging, return and call defer
				// defer will close the socket which will kill the reader
				return
			}

		case bytes := <- c.Quit:
			// Close connection and send a final message
			c.write(websocket.TextMessage, bytes)
			c.write(websocket.CloseMessage, []byte{})
			return
		}
	}

}

//
// This function chews through the power cables
//
func (c *Conn) Close() {
	// Send ourself the quit signal with no message
	c.Quit <- []byte("")
}

var upgrader = &websocket.Upgrader{ReadBufferSize: 1024, WriteBufferSize: 1024, CheckOrigin: func(r* http.Request) bool { return true }}

//
// Hanlder function to start a websocket connection
//
func ConnectionHandler(w http.ResponseWriter, r *http.Request, dst BottleDst) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	// Create new connection object
	c := &Conn{
		Send: make(chan []byte, 256),
		Ws: ws,
		Dst: dst,
		Quit: make(chan []byte),
	}

	// Alert the destination that a new connection has opened
	dst.ConnectionOpened(c)

	// Start infinite read/write loop
	c.read_write()
}
