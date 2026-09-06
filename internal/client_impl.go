package internal

import (
	"go-spdsxpro/internal/log"
	"go-spdsxpro/types"
	"time"
)

func NewClient(config *types.ClientConfig) (*linuxMidiClient, error) {
	c := &linuxMidiClient{path: config.PortPath, deviceID: DefaultDeviceID, timeout: 2 * time.Second}
	log.SetDebug(config.Debug)
	err := c.Connect()
	if err != nil {
		return nil, err
	}
	return c, nil
}
