package oomph

import "github.com/sandertv/gophertunnel/minecraft/protocol"

/*
   1651113805
   1909043648
   1835298427
*/

var DeviceOSToString = map[protocol.DeviceOS]string{
	protocol.DeviceAndroid:   "Android",
	protocol.DeviceIOS:       "iOS",
	protocol.DeviceOSX:       "MacOS",
	protocol.DeviceFireOS:    "Fire OS",
	protocol.DeviceGearVR:    "Gear VR",
	protocol.DeviceHololens:  "Hololens",
	protocol.DeviceWin10:     "Windows 10",
	protocol.DeviceWin32:     "Win32",
	protocol.DeviceDedicated: "Dedicated",
	protocol.DeviceTVOS:      "TV",
	protocol.DeviceOrbis:     "PlayStation",
	protocol.DeviceNX:        "Nintendo",
	protocol.DeviceXBOX:      "Xbox",
	protocol.DeviceWP:        "Windows Phone",
}
