package plugins_test

import (
	"errors"
	"testing"

	"gz-ia/packages/orchy/core"
	"gz-ia/packages/orchy/plugins"
)

type customPlugin struct {
	plugins.BasePlugin
	bootCalled     bool
	shutdownCalled bool
}

func (c *customPlugin) OnBoot(ctx *core.KernelContext) error {
	c.bootCalled = true
	return nil
}

func (c *customPlugin) OnShutdown(ctx *core.KernelContext) error {
	c.shutdownCalled = true
	return nil
}

type errPlugin struct {
	plugins.BasePlugin
}

func (e *errPlugin) OnBoot(ctx *core.KernelContext) error {
	return errors.New("boot err")
}

func (e *errPlugin) OnShutdown(ctx *core.KernelContext) error {
	return errors.New("shutdown err")
}

func TestPluginRegistry(t *testing.T) {
	reg := plugins.NewPluginRegistry()

	p1 := &customPlugin{
		BasePlugin: plugins.BasePlugin{
			PluginName:    "plugin-1",
			PluginVersion: "1.2.3",
		},
	}
	p2 := &customPlugin{
		BasePlugin: plugins.BasePlugin{
			PluginName: "plugin-2", // default version
		},
	}

	// Register
	if err := reg.Register(p1); err != nil {
		t.Fatalf("failed to register p1: %v", err)
	}
	if err := reg.Register(p2); err != nil {
		t.Fatalf("failed to register p2: %v", err)
	}

	// Default version test
	if p2.Version() != "1.0.0" {
		t.Fatalf("expected default version '1.0.0', got %q", p2.Version())
	}

	// Duplicate error
	if err := reg.Register(p1); err == nil {
		t.Fatalf("expected error registering duplicate plugin")
	}

	// Nil plugin error
	if err := reg.Register(nil); err == nil {
		t.Fatalf("expected error registering nil plugin")
	}

	// Count & Has & Get
	if reg.Count() != 2 {
		t.Fatalf("expected 2 plugins, got %d", reg.Count())
	}
	if !reg.Has("plugin-1") || reg.Has("missing") {
		t.Fatalf("unexpected Has() results")
	}
	gotP1, ok := reg.Get("plugin-1")
	if !ok || gotP1.Name() != "plugin-1" {
		t.Fatalf("failed to Get() p1")
	}

	// List
	list := reg.List()
	if len(list) != 2 || list[0].Name() != "plugin-1" || list[1].Name() != "plugin-2" {
		t.Fatalf("unexpected List(): %v", list)
	}

	// Lifecycle execution
	k := core.NewKernel()
	kCtx := k.GetContext()

	if err := reg.BootAll(kCtx); err != nil {
		t.Fatalf("BootAll failed: %v", err)
	}
	if !p1.bootCalled || !p2.bootCalled {
		t.Fatalf("expected both plugins to have OnBoot called")
	}

	if err := reg.ShutdownAll(kCtx); err != nil {
		t.Fatalf("ShutdownAll failed: %v", err)
	}
	if !p1.shutdownCalled || !p2.shutdownCalled {
		t.Fatalf("expected both plugins to have OnShutdown called")
	}

	// Remove
	if !reg.Remove("plugin-1") {
		t.Fatalf("expected Remove('plugin-1') to return true")
	}
	if reg.Count() != 1 {
		t.Fatalf("expected 1 plugin after remove, got %d", reg.Count())
	}

	// Clear
	reg.Clear()
	if reg.Count() != 0 {
		t.Fatalf("expected empty registry after Clear()")
	}
}

func TestPluginRegistry_LifecycleErrors(t *testing.T) {
	reg := plugins.NewPluginRegistry()
	_ = reg.Register(&errPlugin{BasePlugin: plugins.BasePlugin{PluginName: "err-p"}})

	k := core.NewKernel()
	kCtx := k.GetContext()

	if err := reg.BootAll(kCtx); err == nil {
		t.Fatalf("expected error from BootAll")
	}

	if err := reg.ShutdownAll(kCtx); err == nil {
		t.Fatalf("expected error from ShutdownAll")
	}
}
