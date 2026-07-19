package main

import (
	"bufio"
	"io"
	"strings"
)

func loadModerators(r io.Reader) (map[string]struct{}, error) {
	moderators := make(map[string]struct{})
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		name := strings.TrimSpace(scanner.Text())
		if name != "" {
			moderators[name] = struct{}{}
		}
	}
	return moderators, scanner.Err()
}

func backendAddresses(primary, backup string) []string {
	addresses := make([]string, 0, 2)
	if primary != "" {
		addresses = append(addresses, primary)
	}
	if backup != "" && backup != primary {
		addresses = append(addresses, backup)
	}
	return addresses
}

func shutdownTarget(address string, port int) (string, uint16, bool) {
	if address == "" || port <= 0 || port > 65535 {
		return "", 0, false
	}
	return address, uint16(port), true
}
