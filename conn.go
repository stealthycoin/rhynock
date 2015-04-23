package rhynock

import (
	"github.com/gorilla/websocket"
	"net/http"
	"time"
	"log"
)

// Connection handles our websocket
type Connection struct {
	// Exported
	Ws      *websocket.Conn
	Send    chan []byte
	Dst     BottleDst

	// Private
	quit    chan bool
}


const (
	writeWait = 10 * time.Second
	pongWait = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10
	maxMessageSize = 512
)

//
// Used to write a single message to the client and report any errors
//
func (c *Connection) write(t int, payload []byte) error {
	c.Ws.SetWriteDeadline(time.Now().Add(writeWait))
	return c.Ws.WriteMessage(t, payload)
}

//
// Maintains both a reader and a writer, cleans up both if one fails
//
func (c *Connection) read_write() {
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
				// Our channel was closed, gracefully close socket Connection
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

		case <- c.quit:
			// Quit signal was invoked by our Close function
			// The bottle destination wants this connection closed
			c.write(websocket.CloseMessage, []byte{})
			return
		}
	}

}

func (c *Connection) Close() {
	// Send ourself the quit signal provided by a function
	c.quit <- true
}

var upgrader = &websocket.Upgrader{ReadBufferSize: 1024, WriteBufferSize: 1024, CheckOrigin: func(r* http.Request) bool { return true }}

func ConnectionHandler(w http.ResponseWriter, r *http.Request, dst BottleDst) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	// Create new connection object
	c := &Connection{
		Send: make(chan []byte, 256),
		Ws: ws,
		Dst: dst,
		quit: make(chan bool),
	}

	dst.ConnectionOpened(c)

	// Start infinite read/write loop
	c.read_write()
}
