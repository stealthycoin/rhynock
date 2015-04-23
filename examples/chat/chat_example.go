package main

import (
	"github.com/stealthycoin/rhynock"
	"net/http"
)

type Router struct {
	// Kind of sorta keep track of who is connected
	connections map[*rhynock.Conn]bool

	// Channel to recieve bottles through
	bottle     chan *rhynock.Bottle
}


// Satisfy the BottleDst interface
func (r *Router) GetBottleChan() (chan *rhynock.Bottle) {
	return r.bottle
}

// Satisfy the BottleDst interface
func (r *Router) ConnectionClosed(c *rhynock.Conn) {
	delete(r.connections, c)
}

// Satisfy the BottleDst interface
func (r *Router) ConnectionOpened(c *rhynock.Conn) {
	// This should put the connection into an authenticating mode
	// and then upon authentication the connection is added to a map
	// where the value points at some kind of profile object
	// can all c.Close() upon failing to authenticate
	c.Sender.SendMsg("Welcome to echochat. No names. No rules.")
	r.connections[c] = true
}

func main() {
	router := &Router{
		connections: make(map[*rhynock.Conn]bool),
		bottle: make(chan *rhynock.Bottle),
	}

	// Register the route to rhynock handler function and pass in our BottleDst
	http.HandleFunc("/socket/", func (w http.ResponseWriter, r *http.Request) {
		rhynock.ConnectionHandler(w, r, router)
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})

	// Start router listening routine
	go func() {
		for {
			// Listen for a bottle
			btl := <- router.bottle

			// Loop through all active connections
			for c, _ := range router.connections {
				// Send everyone the message
				c.Send <- btl.Message
			}
		}
	}()

	http.ListenAndServe("127.0.0.1:8000", nil)
}
