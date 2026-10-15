//go:build android

package vnet

import (
	"context"
	"io"
	"net"

	v1 "github.com/fatedier/frp/pkg/config/v1"
)

// Controller is a no-op stub on Android.
// Virtual network (TUN) is not supported in the gomobile AAR build,
// but we keep this type and its methods so that plugins and client code compile.
type Controller struct{}

// NewController returns a stub controller on Android.
func NewController(cfg v1.VirtualNetConfig) *Controller {
	_ = cfg
	return &Controller{}
}

// Init is a no-op on Android.
func (c *Controller) Init() error {
	return nil
}

// Run is a no-op on Android.
func (c *Controller) Run() error {
	return nil
}

// Stop is a no-op on Android.
func (c *Controller) Stop() error {
	return nil
}

// RegisterClientRoute is a no-op on Android.
func (c *Controller) RegisterClientRoute(ctx context.Context, name string, routes []net.IPNet, conn io.ReadWriteCloser) {
	_ = ctx
	_ = name
	_ = routes
	_ = conn
}

// UnregisterClientRoute is a no-op on Android.
func (c *Controller) UnregisterClientRoute(name string, conn io.Writer) bool {
	_ = name
	_ = conn
	return false
}

// StartServerConnReadLoop is a no-op on Android.
func (c *Controller) StartServerConnReadLoop(ctx context.Context, conn io.ReadWriteCloser, onClose func()) {
	_ = ctx
	_ = conn
	if onClose != nil {
		onClose()
	}
}

