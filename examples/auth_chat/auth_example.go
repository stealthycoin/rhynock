package main

import (
	"github.com/stealthycoin/rhynock"
	"net/http"
)

// Super amazing database of users
var (
	people = map[string]string{
		"Steve": "password1",
		"Joe": "12345",
		"Anna": "hunter2",
	}
)


// Profile object to keep track of login status
type Profile struct {
	authed bool // Is this person authenticated yet?
	name string // Who is this person?
}

type Router struct {
	// Do a decent job keeping track of who is logged in
	connections map[*rhynock.Conn]*Profile

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
	// Create an empty profile for them
	r.connections[c] = &Profile{}

	// Send them a login prompt
	c.SendMsg("Enter your username")
}

// Function to manage login process
func (r *Router) login(b *rhynock.Bottle) {
	// For ease of use
	profile := r.connections[b.Sender]

	// Is the message a username or a password?
	// Username comes first so check if its empty
	if profile.name == "" {
		// Dont know their name yet so this message should be their name
		profile.name = string(b.Message)

		// Now tell them to enter their password
		b.Sender.SendMsg("Enter password")
	} else {

		// Their name is set so this is their password
		// Check our super super database to see if they
		// authenticated correctly
		pass := string(b.Message)
		if db_pass, ok := people[profile.name]; ok {
			// The name was in the list now lets check their password
			if pass == db_pass {
				// Huzzah they logged in successfully
				profile.authed = true

				// Alert them they are logged in
				b.Sender.SendMsg("Logged in as " + profile.name)
			} else {
				// They typed the password wrong
				b.Sender.CloseMsg("Invalid password.")
			}
		} else {
			// The name wasn't in the list so they fail
			b.Sender.CloseMsg("Invalid username.")
		}
	}
}

func main() {
	router := &Router{
		connections: make(map[*rhynock.Conn]*Profile),
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

			// If this person is not authenticated forward the bottle to the login function
			if !router.connections[btl.Sender].authed {
				router.login(btl)
			} else {
				// If they are logged in...
				// reconstruct the message to include the username
				message := []byte(router.connections[btl.Sender].name + ": " + string(btl.Message))

				// Loop through all active connections
				for c, _ := range router.connections {
					// Send everyone the message
					c.Send <- message
				}
			}
		}
	}()

	http.ListenAndServe("127.0.0.1:8000", nil)
}
