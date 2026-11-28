//go:build esp32c3 || esp32c6 || esp32s3

package main

import "machine"

var i2s = &machine.I2S0

const (
	pinSCK = machine.GPIO3
	pinWS  = machine.GPIO4
	pinSDO = machine.GPIO5
)
