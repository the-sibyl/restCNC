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
	"sync"
	"time"

	"github.com/the-sibyl/sysfsGPIO"
	"golang.org/x/exp/io/i2c"
)

type dacIO struct {
	// Hardware-related fields
	// Power supply enable pin
	psuEnaPin *sysfsGPIO.IOPin
	// MCP4725 I2C device
	i2cConn *i2c.Device
	// Address 0 pin for the MCP4725 breakout
	a0Pin *sysfsGPIO.IOPin

	// Calculation-related fields
	// Time delay between sending commands when ramping up/down RPM
	rampDelay time.Duration
	// Currently set voltage
	curVoltage int
	// Arbitrary scale factor. This should be improved upon if the project is expanded.
	dacScaleFactor float32

	// Emergency stop signal - using a channel because of possible future implementations
	EStop chan bool
	// Unexported internal variable
	eStop bool

	// General mutex for blocking multiple operations from other packages
	mutex *sync.Mutex
}

// Open the device
func Open(devString string, i2cAddress int, a0Pin int, psuEnaPin int) (*dacIO, error) {
	var d dacIO
	var err error

	d.mutex = &sync.Mutex{}

	// Default ramp delay
	d.rampDelay = 0

	// Default scale factor (guess)
	d.dacScaleFactor = 0.4

	// The LSB of the I2C address (A0) is configured from this GPIO. Set it low to make A0=0.
	d.a0Pin, err = sysfsGPIO.InitPin(a0Pin, "out")
	if err != nil {
		return &d, err
	}
	d.a0Pin.SetLow()
	// Set up I2C
	d.i2cConn, err = i2c.Open(&i2c.Devfs{Dev: devString}, i2cAddress)
	if err != nil {
		return &d, err
	}

	// Enable pin for the power supply, active low
	d.psuEnaPin, err = sysfsGPIO.InitPin(psuEnaPin, "out")
	if err != nil {
		return &d, err
	}
	d.psuEnaPin.SetHigh()

	d.EStop = make(chan bool)
	d.eStop = true

	go func() {
		for {
			select {
			case d.eStop = <-d.EStop:
				// Disable the power supply upon emergency stop
				if d.eStop {
					d.DisablePSU()
				} else {
					d.EnablePSU()
				}
			}
		}
	}()

	return &d, err
}

// Close the device
func (d *dacIO) Close() {
	d.a0Pin.ReleasePin()
	d.i2cConn.Close()
	d.psuEnaPin.ReleasePin()
}

// Set the sleep duration between when a voltage command is sent to ramp up/down
func (d *dacIO) SetRampDelay(t time.Duration) {
	d.mutex.Lock()

	d.rampDelay = t

	d.mutex.Unlock()
}

// Ramp upward or downward to a particular RPM value, LSB by LSB. This function may be interrupted by an EStop signal.
func (d *dacIO) RampToRPM(rpm int) {
	d.mutex.Lock()

	finalVoltage := int(float32(rpm) * d.dacScaleFactor)

	delta := 0

	if finalVoltage > d.curVoltage {
		delta = 1
	} else {
		delta = -1
	}

	for cur := d.curVoltage; cur != finalVoltage && cur >= 0; cur += delta {
		// Expected behavior: if the EStop signal is caught, the spindle will have to stop before it can be
		// restarted. If the EStop signal is VERY short (on the order of milliseconds), it may not be caught,
		// but such a signal is useless anyway.
		if d.eStop {
			delta = -1
			finalVoltage = 0
		}

		err := d.writeVoltage(cur)
		if err != nil {
			fmt.Println(err)
		}
		time.Sleep(d.rampDelay)
	}

	d.curVoltage = finalVoltage

	d.mutex.Unlock()
}

// Write the voltage to volatile memory
func (d *dacIO) writeVoltage(voltage int) error {
	if voltage < 0 || voltage >= 1<<12 {
		return errors.New("writeVoltage() voltage value out of range")
	}

	// The first four bits corresponding to Fast Mode and Power Down Select are zeroed
	highByte := byte(0x0F & (voltage >> 8))
	// Mask out all but the lowest byte for clarity (truncated anyway)
	lowByte := byte(0xFF & voltage)
	// The two bytes are repeated to complete a command
	err := d.i2cConn.Write([]byte{highByte, lowByte, highByte, lowByte})
	return err
}

// This function needs to be done one time per device to set the power-up default output to 0V
func (d *dacIO) WriteNVInit() error {
	// TODO: Complete this
	return nil
}

func (d *dacIO) EnablePSU() error {
	d.mutex.Lock()
	err := d.psuEnaPin.SetLow()
	d.mutex.Unlock()
	return err
}

func (d *dacIO) DisablePSU() error {
	d.mutex.Lock()
	err := d.psuEnaPin.SetHigh()
	d.mutex.Unlock()
	return err
}
