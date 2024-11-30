package utils

import (
	"strings"

	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sandertv/gophertunnel/minecraft/text"
)

// MC_CenterAlignText takes an array of strings and centers them based on the longest string.
func MC_CenterAlignText(msg ...string) string {
	if len(msg) == 0 {
		return ""
	}
	for i, m := range msg {
		msg[i] = text.Colourf(m)
	}

	// Find the length of the longest string
	max_length := 0
	for _, line := range msg {
		if clean_length := len(clean_line(line)); clean_length > max_length {
			max_length = clean_length
		}
	}

	var centered_lines []string
	for _, line := range msg {
		clean_length := len(clean_line(line))
		if clean_length == max_length {
			centered_lines = append(centered_lines, line)
			continue
		}

		total_spaces := int((float64(max_length-clean_length)/10.0)*6.0) + 1
		centered_line := strings.Repeat(" ", total_spaces) + line
		centered_lines = append(centered_lines, centered_line)
	}

	// Join the centered lines with newlines
	return strings.Join(centered_lines, "\n")
}

func clean_line(line string) string {
	return strings.ReplaceAll(text.Clean(line), "\n", "")
}

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
	case packet.InputModeMotionController:
		return "Motion Controller"
	}

	return "Unknown"
}
