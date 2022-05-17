package utils

import (
	"fmt"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"strings"
)

// Device returns the device name from the DeviceOS.
func Device(os protocol.DeviceOS) string {
	switch os {
	case protocol.DeviceAndroid:
		return "Android"
	case protocol.DeviceIOS:
		return "iOS"
	case protocol.DeviceOSX:
		return "MacOS"
	case protocol.DeviceFireOS:
		return "FireOS"
	case protocol.DeviceGearVR:
		return "Gear VR"
	case protocol.DeviceHololens:
		return "Hololens"
	case protocol.DeviceWin10:
		return "Windows 10"
	case protocol.DeviceWin32:
		return "Win32"
	case protocol.DeviceDedicated:
		return "Dedicated"
	case protocol.DeviceTVOS:
		return "TV"
	case protocol.DeviceOrbis:
		return "PlayStation"
	case protocol.DeviceNX:
		return "Nintendo"
	case protocol.DeviceXBOX:
		return "Xbox"
	case protocol.DeviceWP:
		return "Windows Phone"
	}
	return "Unknown"
}

// PrettyParameters converts the given parameters to a readable string.
func PrettyParameters(params map[string]any, condensed bool) string {
	if len(params) == 0 {
		// Don't waste time if there are no parameters.
		return "[]"
	}

	sb := &strings.Builder{}
	sb.WriteString("[")
	if !condensed {
		sb.WriteString("\n")
	}

	var ind int
	for k, v := range params {
		if !condensed {
			sb.WriteString(strings.Repeat(" ", 4))
		}
		sb.WriteString(fmt.Sprint(k))
		sb.WriteString(": ")
		sb.WriteString(fmt.Sprint(v))
		if condensed {
			if ind < len(params)-1 {
				sb.WriteString(", ")
			}
		} else {
			sb.WriteString(",\n")
		}
		ind++
	}

	sb.WriteString("]")
	return sb.String()
}
