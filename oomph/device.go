package oomph

import "github.com/sandertv/gophertunnel/minecraft/protocol"

var TitleIds = map[string]protocol.DeviceOS{
	"1739947436": protocol.DeviceAndroid,
	"1810924247": protocol.DeviceIOS,
	"1944307183": protocol.DeviceFireOS,
	"896928775":  protocol.DeviceWin10,
	"2044456598": protocol.DeviceOrbis,
	"2047319603": protocol.DeviceNX,
	"1828326430": protocol.DeviceXBOX,
	"1916611344": protocol.DeviceWP,
}

/*
   1651113805
   1909043648
   1835298427
*/

var DeviceOSToString = map[protocol.DeviceOS]string{
	protocol.DeviceAndroid:   "Android",
	protocol.DeviceIOS:       "IOS",
	protocol.DeviceOSX:       "MacOS",
	protocol.DeviceFireOS:    "Fire OS",
	protocol.DeviceGearVR:    "Gear VR",
	protocol.DeviceHololens:  "Hololens",
	protocol.DeviceWin10:     "Win10",
	protocol.DeviceWin32:     "Win32",
	protocol.DeviceDedicated: "Dedicated",
	protocol.DeviceTVOS:      "TV",
	protocol.DeviceOrbis:     "Playstation",
	protocol.DeviceNX:        "Nintendo",
	protocol.DeviceXBOX:      "Xbox",
	protocol.DeviceWP:        "Windows Phone",
}
