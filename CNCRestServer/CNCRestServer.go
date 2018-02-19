/*
Copyright (c) 2018 Forrest Sibley <My^Name^Without^The^Surname@ieee.org>
Permission is hereby granted, free of charge, to any person obtaining
a copy of this software and associated documentation files (the
"Software"), to deal in the Software without restriction, including
without limitation the rights to use, copy, modify, merge, publish,
distribute, sublicense, and/or sell copies of the Software, and to
permit persons to whom the Software is furnished to do so, subject to
the following conditions:
The above copyright notice and this permission notice shall be
included in all copies or substantial portions of the Software.
THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND
NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE
LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION
OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION
WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
*/

package CNCRestServer

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/the-sibyl/restCNC/dacIO"
)

// Structure for JSON marshalling. Tags are on the right. Both the struct name and its fields must be capitalized to be
// exported properly.
// JSON from LinuxCNC machine
type SpindleDataIn struct {
	// Spindle enable signal
	Enable bool `json:"enable"`
	// Set point: the current commanded RPM
	Setpoint float64 `json:"setpoint"`
}

// JSON to LinuxCNC machine
type SpindleDataOut struct {
	// Ramping state: if false, the spindle is not yet to speed
	Ramping bool `json:"ramping"`
	// Current Setpoint
	CurrentSetpoint float64 `json:"currentsetpoint"`
	// Current RPM: the actual current RPM
	CurrentRPM float64 `json:"currentrpm"`
}

// Root HTTP response for debugging
func httpIndex(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "CNC Milling Machine REST Server")
}

// Return the current spindle state to the client
func (c *RestServer) httpGetSpindle(w http.ResponseWriter, r *http.Request) {
	currentState := SpindleDataOut{
		Ramping: c.ramping,
		CurrentSetpoint: c.currentSetpoint,
		CurrentRPM: c.currentRPM,
	}

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)

	// Convert the spindle state to JSON
	j, err := json.Marshal(currentState)
	if err != nil {
		fmt.Println("JSON Error")
		fmt.Println(err)
	}

	// Send the value back to the client
	writeCode, err := w.Write(j)
	if err != nil {
		fmt.Println("Write Error", writeCode)
	}
}

// Set a new spindle state
func (c *RestServer) httpPostSpindle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)

	var newState SpindleDataIn

	// Read incoming HTTP text
	incomingText, err := ioutil.ReadAll(r.Body)
	if err != nil {
		fmt.Println("Read Error")
		fmt.Println(err)
	}

	err = json.Unmarshal(incomingText, &newState)
	if err != nil {
		fmt.Println("JSON Error")
		fmt.Println(err)
	}

	fmt.Println(newState)

	// The hardware may take some time, so spin this off into a goroutine
	go func() {
		if !newState.Enable {
			c.hw.EStop <- true
		} else if !c.ramping {
			c.ramping = true
			c.hw.EStop <- false
			c.currentSetpoint = newState.Setpoint
			c.hw.RampToRPM(int(newState.Setpoint))
			// Once RampToRPM() has returned, assume that the desired values are all true. This is a rudimentary
			// approach.
			c.currentRPM = newState.Setpoint
			c.ramping = false
		}
	}()
}

type RestServer struct {
	router *mux.Router
	httpServer *http.Server
	hw *dacIO.DacIO

	// Parameters for the spindle
	currentSetpoint float64
	currentRPM float64
	ramping bool
	spindleEnable bool
}

func Open(addr string, d *dacIO.DacIO) (*RestServer) {
	var c RestServer

	// Set up REST/HTTP portions
	c.router = mux.NewRouter().StrictSlash(true)
	c.router.HandleFunc("/", httpIndex)
	c.router.HandleFunc("/spindle", c.httpGetSpindle).Methods("GET")
	c.router.HandleFunc("/spindle", c.httpPostSpindle).Methods("POST")
	c.httpServer = new(http.Server)
	if addr != "" {
		c.httpServer.Addr = addr
	} else {
		c.httpServer.Addr = ":8080"
	}
	c.httpServer.Handler = c.router
	go c.httpServer.ListenAndServe()

	c.currentSetpoint = 0
	c.currentRPM = 0
	c.ramping = false
	c.spindleEnable = false

	c.hw = d

	return &c
}

func (c *RestServer) Close() {
	c.httpServer = nil
	c.router = nil
}
