package clientopts

import (
	"go-spdsxpro/types"
)

func WithDebug(enabled bool) func(config *types.ClientConfig) {
	return func(config *types.ClientConfig) {
		config.Debug = enabled
	}
}
