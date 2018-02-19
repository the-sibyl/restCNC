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

// Structure for JSON marshalling. Tags are on the right.
// JSON from LinuxCNC machine
type spindleDataIn struct {
	// Spindle enable signal
	enable bool `json:"enable"`
	// Set point: the current commanded RPM
	setpoint float64 `json:"setpoint"`
}

// JSON to LinuxCNC machine
type spindleDataOut struct {
	// Ramping state: if false, the spindle is not yet to speed
	ramping bool `json:"ramping"`
	// Current Setpoint
	currentSetpoint float64 `json:"currentsetpoint"`
	// Current RPM: the actual current RPM
	currentRPM float64 `json:"currentrpm"`
}

// Root HTTP response for debugging
func httpIndex(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "CNC Milling Machine REST Server")
}

// Return the current spindle state to the client
// This can be tested with curl.
// curl -i -X GET -H "Content-Type:application/json" http://localhost:8080/spindle
func (c *RestServer) httpGetSpindle(w http.ResponseWriter, r *http.Request) {
	currentState := spindleDataOut{
		ramping: c.ramping,
		currentSetpoint: c.currentSetpoint,
		currentRPM: c.currentRPM,
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
// This can be tested with curl.
// curl -i -X POST -H "Content-Type:application/json" http://localhost:8080/spindle -d '{"value":12345}'
func (c *RestServer) httpPostSpindle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)

	var newState spindleDataIn

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

	if !newState.enable {
		c.hw.EStop <- true
	} else if !c.ramping {
		c.ramping = true
		c.hw.EStop <- false
		c.currentSetpoint = newState.setpoint
		c.hw.RampToRPM(int(newState.setpoint))
		// Once RampToRPM() has returned, assume that the desired values are all true. This is a rudimentary
		// approach.
		c.currentRPM = newState.setpoint
	}
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

func Open(addr string) (*RestServer) {
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
	c.httpServer.ListenAndServe()

	c.currentSetpoint = 0
	c.currentRPM = 0
	c.ramping = false
	c.spindleEnable = false

// TODO: set c.hw

	return &c
}

func (c *RestServer) Close() {
	c.httpServer = nil
	c.router = nil
}
