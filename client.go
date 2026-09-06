package spdsxpro

import (
	"go-spdsxpro/internal"
	"go-spdsxpro/types"
)

func NewClient(portPath string, opts ...types.ClientOpt) (types.Client, error) {
	config := &types.ClientConfig{PortPath: portPath, Debug: false}
	for _, opt := range opts {
		opt(config)
	}
	return internal.NewClient(config)
}
