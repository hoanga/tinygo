//go:build crowpanel_esp32p4_10_1

package main

import "machine"

var i2s = &machine.I2S1

const (
	pinSCK = machine.I2S_BCLK
	pinWS  = machine.I2S_LRCLK
	pinSDO = machine.I2S_SDATA
)
