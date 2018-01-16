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

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/gorilla/mux"
)

// A struct with tags. The Name and one of the Value fields are required. This is a quick and dirty implementation.
type SpindleRPM struct {
	Value float64 `json:"value"`
}

var spindleRPM SpindleRPM

func main() {
	spindleRPM.Value = -12345

	router := mux.NewRouter().StrictSlash(true)
	router.HandleFunc("/", httpIndex)
	router.HandleFunc("/spindle", httpGetSpindle).Methods("GET")
	router.HandleFunc("/spindle", httpPostSpindle).Methods("POST")
	http.ListenAndServe(":8080", router)
}

func httpIndex(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "CNC Milling Machine REST Server")
}

// Return the current spindle RPM value to the client
// This can be tested with curl.
// curl -i -X GET -H "Content-Type:application/json" http://localhost:8080/spindle
func httpGetSpindle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)

	// Convert the spindle RPM value to JSON
	j, err := json.Marshal(spindleRPM)
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

// Set a new spindle RPM value
// This can be tested with curl.
// curl -i -X POST -H "Content-Type:application/json" http://localhost:8080/spindle -d '{"value":12345}'
func httpPostSpindle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)

	var newRPM SpindleRPM

	// Read incoming HTTP text
	incomingText, err := ioutil.ReadAll(r.Body)
	if err != nil {
		fmt.Println("Read Error")
		fmt.Println(err)
	}

	err = json.Unmarshal(incomingText, &newRPM)
	if err != nil {
		fmt.Println("JSON Error")
		fmt.Println(err)
	}

	spindleRPM = newRPM
	fmt.Println("New RPM value:", newRPM.Value)
}
