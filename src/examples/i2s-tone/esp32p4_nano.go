//go:build esp32p4_nano

package main

import "machine"

var i2s = &machine.I2S0

const (
	pinSCK = machine.I2S_BCLK
	pinWS  = machine.I2S_LRCLK
	pinSDO = machine.I2S_SDATA
)
