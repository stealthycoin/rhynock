package main

import (
	"github.com/stealthycoin/rhynock"
	"net/http"
)

type Router struct {
	// Kind of sorta keep track of who is connected
	connections map[*rhynock.Connection]bool

	// Channel to recieve bottles through
	bottle     chan *rhynock.Bottle
}


// Satisfy the BottleDst interface
func (r *Router) GetBottleChan() (chan *rhynock.Bottle) {
	return r.bottle
}

// Satisfy the BottleDst interface
func (r *Router) ConnectionClosed(c *rhynock.Connection) {
	delete(r.connections, c)
}

// Satisfy the BottleDst interface
func (r *Router) ConnectionOpened(c *rhynock.Connection) {
	// This should put the connection into an authenticating mode
	// and then upon authentication the connection is added to a map
	// where the value points at some kind of profile object
	// can all c.Close() upon failing to authenticate
	r.connections[c] = true
}

func main() {
	router := &Router{
		connections: make(map[*rhynock.Connection]bool),
		bottle: make(chan *rhynock.Bottle),
	}

	// Register the route to rhynock handler function and pass in our BottleDst
	http.HandleFunc("/", func (w http.ResponseWriter, r *http.Request) {
		rhynock.ConnectionHandler(w, r, router)
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

	http.ListenAndServe("127.0.0.1:8002", nil)
}
