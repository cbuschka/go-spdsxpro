package spdsxpro

import (
	"go-spdsxpro/internal"
	"go-spdsxpro/types"
)

func NewClient(portPath string) (types.Client, error) {
	return internal.NewClient(portPath)
}
