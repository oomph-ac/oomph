package utils

import (
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
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

// InputMode returns the input mode name from the InputMode.
func InputMode(mode int) string {
	switch mode {
	case packet.InputModeMouse:
		return "Keyboard/Mouse"
	case packet.InputModeTouch:
		return "Touch"
	case packet.InputModeGamePad:
		return "Gamepad"
	case 4: // legacy: packet.InputModeMotionController - removed as of 1.21.120
		return "Motion Controller"
	}
	return "Unknown"
}
