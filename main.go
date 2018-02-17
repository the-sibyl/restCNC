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
	"fmt"

	"github.com/the-sibyl/sysfsGPIO"
	"github.com/the-sibyl/restCNC/dacIO"
//	"github.com/the-sibyl/restCNC/CNCRestServer"
)

func main() {
	// Enable the amber LED
	gpio22, _ := sysfsGPIO.InitPin(22, "out")
	defer gpio22.ReleasePin()
	// LED is wired for active low
	gpio22.SetLow()

	// Open the I2C DAC and set up the power supply enable
	d, err := dacIO.Open("/dev/i2c-1", 0x62, 4, 21)
	if err != nil {
		panic(err)
	}
	defer d.Close()

	go func() {
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
	} ()

}

