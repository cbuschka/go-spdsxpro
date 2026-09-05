package spdsxpro

import "time"

type Client interface {
	Ping() error
	Close() error
	GetActiveKit() (int, error)
}

func NewClient(portPath string) (Client, error) {
	c := &linuxMidiClient{path: portPath, deviceID: DefaultDeviceID, timeout: 2 * time.Second}
	err := c.Connect()
	if err != nil {
		return nil, err
	}
	return c, nil
}
