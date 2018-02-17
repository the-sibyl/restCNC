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

package dacIO

import (
	"errors"
	"fmt"
	"time"

	"golang.org/x/exp/io/i2c"
	"github.com/the-sibyl/sysfsGPIO"
	"github.com/the-sibyl/restCNC/CNCRestServer"
)


// This struct contains information for the DAC and the power supply enable pin
type dacIO struct {
	PSUEnaPin *sysfsGPIO.IOPin
	I2CConn *i2c.Device
	A0Pin *sysfsGPIO.IOPin
	// Time delay between sending commands when ramping up/down RPM
	RampDelay time.Duration
	// Currently set voltage
	CurVoltage int
	// Arbitrary scale factor. This should be improved upon if the project is expanded.
	DACScaleFactor float32
	// CNC REST Server struct to handle I/O from the LinuxCNC machine
	RestServer *CNCRestServer.RestServer
}

// Open the device
func Open(devString string, i2cAddress int, a0Pin int, psuEnaPin int) (*dacIO, error) {
	var d dacIO
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
	if err != nil {
		return &d, err
	}

	// Enable pin for the power supply, active low
	d.PSUEnaPin, err = sysfsGPIO.InitPin(psuEnaPin, "out")
	if err != nil {
		return &d, err
	}
	d.PSUEnaPin.SetHigh()

	d.RestServer = CNCRestServer.Open(":8080")
// TODO: Connect channels to a handler goroutine

	return &d, err
}

// Close the device
func (d *dacIO) Close() {
	d.A0Pin.ReleasePin()
	d.I2CConn.Close()
	d.PSUEnaPin.ReleasePin()
}

// Set the sleep duration between when a voltage command is sent to ramp up/down
func (d *dacIO) SetRampDelay(t time.Duration) {
	d.RampDelay = t
}

// Ramp upward or downward to a particular RPM value, LSB by LSB
func (d *dacIO) RampToRPM(rpm int) {
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
func (d *dacIO) WriteVoltage(voltage int) error {
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
func (d *dacIO) WriteNVInit() error {
	return nil
}

func (d *dacIO) EnablePSU() error {
	return d.PSUEnaPin.SetLow()
}

func (d *dacIO) DisablePSU() error {
	return d.PSUEnaPin.SetHigh()
}
