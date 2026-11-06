//go:build android && !cgo

package client

import "os"

func machineHostname() string {
	hostname, _ := os.Hostname()
	return hostname
}
