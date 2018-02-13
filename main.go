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
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"golang.org/x/exp/io/i2c"
	"github.com/gorilla/mux"
	"github.com/the-sibyl/sysfsGPIO"
)

// A struct with tags. The Name and one of the Value fields are required. This is a quick and dirty implementation.
type SpindleRPM struct {
	Value float64 `json:"value"`
}

var spindleRPM SpindleRPM

type dac struct {
	I2CConn *i2c.Device
	A0Pin *sysfsGPIO.IOPin
	// Time delay between sending commands when ramping up/down RPM
	RampDelay time.Duration
	// Currently set voltage
	CurVoltage int
	// Arbitrary scale factor. This should be improved upon if the project is expanded.
	DACScaleFactor float32
}

// Open the device
func Open(devString string, i2cAddress int, a0Pin int) (*dac, error) {
	var d dac
	var err error

	// Default ramp delay
	d.RampDelay = 0

	// Default scale factor (guess)
	d.DACScaleFactor = 0.4

	// The LSB of the I2C address (A0) is configured from this GPIO. Set it low to make A0=0.
	d.A0Pin, err = sysfsGPIO.InitPin(a0Pin, "out")
	if err != nil {
		return &d, err
	}
	d.A0Pin.SetLow()
	// Set up I2C
	d.I2CConn, err = i2c.Open(&i2c.Devfs{Dev: devString}, i2cAddress)

	return &d, err
}

// Close the device
func (d *dac) Close() {
	d.A0Pin.ReleasePin()
	d.I2CConn.Close()
}

// Set the sleep duration between when a voltage command is sent to ramp up/down
func (d *dac) SetRampDelay(t time.Duration) {
	d.RampDelay = t
}

// Ramp upward or downward to a particular RPM value, LSB by LSB
func (d *dac) RampToRPM(rpm int) {
	finalVoltage := int(float32(rpm) * d.DACScaleFactor)
	fmt.Println("rpm:", rpm, "final voltage:", finalVoltage)

	delta := 0

	if finalVoltage > d.CurVoltage {
		delta = 1
	} else {
		delta = -1
	}
	for cur := d.CurVoltage; cur != finalVoltage; cur += delta {
		err := d.WriteVoltage(cur)
		if err != nil {
			fmt.Println(err)
		}
		time.Sleep(d.RampDelay)
	}

	d.CurVoltage = finalVoltage
}

// Write the voltage to volatile memory
func (d *dac) WriteVoltage(voltage int) error {
	if voltage < 0 || voltage >= 1 << 12 {
		return errors.New("WriteVoltage() voltage value out of range")
	}

	// The first four bits corresponding to Fast Mode and Power Down Select are zeroed
	highByte := byte(0x0F & (voltage >> 8))
	// Mask out all but the lowest byte for clarity (truncated anyway)
	lowByte := byte(0xFF & voltage)
	// The two bytes are repeated to complete a command
	err := d.I2CConn.Write([]byte{highByte, lowByte, highByte, lowByte})
	return err
}

// This function needs to be done one time per device to set the power-up default output to 0V
func (d *dac) WriteNVInit() error {
	return nil
}

func main() {
	// Enable the amber LED
	gpio22, _ := sysfsGPIO.InitPin(22, "out")
	defer gpio22.ReleasePin()
	// LED is wired for active low
	gpio22.SetLow()

	//a.I2CConnd, err = i2c.Open(&i2c.Devfs{Dev: "/dev/i2c-1"}, 0x62)

	// Open the I2C ADC
	d, err := Open("/dev/i2c-1", 0x62, 4)
	if err != nil {
		panic(err)
	}
	defer d.Close()

	gpio21, err := sysfsGPIO.InitPin(21, "out")
	if err != nil {
		fmt.Println(err)
	}

	go func() {
		for {
			gpio21.SetHigh()
			time.Sleep(time.Second * 5)
			gpio21.SetLow()
			time.Sleep(time.Second * 5)
		}
	} ()

	for {
		fmt.Println("Ramping")
		d.RampToRPM(10000)
		d.RampToRPM(5000)
		d.RampToRPM(10000)
		d.RampToRPM(6000)
		d.RampToRPM(8000)
		d.RampToRPM(100)
		d.RampToRPM(1000)
		d.RampToRPM(500)
		d.RampToRPM(10)
		d.RampToRPM(7000)
		d.RampToRPM(0)
		d.RampToRPM(3850)
		d.RampToRPM(350)
	}

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
