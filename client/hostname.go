//go:build !android

package client

import "os"

func machineHostname() string {
	hostname, _ := os.Hostname()
	return hostname
}
