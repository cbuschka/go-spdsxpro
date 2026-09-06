package internal

import (
	"time"
)

func NewClient(portPath string) (*linuxMidiClient, error) {
	c := &linuxMidiClient{path: portPath, deviceID: DefaultDeviceID, timeout: 2 * time.Second}
	err := c.Connect()
	if err != nil {
		return nil, err
	}
	return c, nil
}
