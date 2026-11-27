package plugins

import (
	"context"
	"testing"

	"github.com/Eureka-o/FanControlPortable/internal/types"
)

type fakeObserverPlugin struct {
	id         string
	name       string
	running    bool
	supported  bool
	reason     string
	seenSpeeds []uint16
	disconnect int
}

func (f *fakeObserverPlugin) ID() string   { return f.id }
func (f *fakeObserverPlugin) Name() string { return f.name }
func (f *fakeObserverPlugin) Start(context.Context) error {
	f.running = true
	return nil
}
func (f *fakeObserverPlugin) Stop() error {
	f.running = false
	return nil
}
func (f *fakeObserverPlugin) Status() Status {
	return Status{ID: f.id, Name: f.name, Running: f.running}
}
func (f *fakeObserverPlugin) Descriptor() Descriptor {
	return Descriptor{Kind: KindBuiltin, Category: "osd", Version: "1.0.0", Capabilities: []string{"osd"}}
}
func (f *fakeObserverPlugin) Available() (bool, string) {
	return f.supported, f.reason
}
func (f *fakeObserverPlugin) OnFanData(fanData *types.FanData) {
	if fanData != nil {
		f.seenSpeeds = append(f.seenSpeeds, fanData.CurrentRPM)
	}
}
func (f *fakeObserverPlugin) OnDeviceDisconnected() {
	f.disconnect++
}

func TestManagerSnapshotAndFanDataObserverDispatch(t *testing.T) {
	mgr := NewManager(nil)
	ready := &fakeObserverPlugin{id: "rtss-osd", name: "RTSS OSD", supported: true}
	unsupported := &fakeObserverPlugin{id: "legion-fnq", name: "Legion Fn+Q", supported: false, reason: "non-legion host"}
	mgr.Register(ready)
	mgr.Register(unsupported)

	if err := mgr.Start("rtss-osd"); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	mgr.PublishFanData(&types.FanData{CurrentRPM: 1850, SpeedUnit: types.FanSpeedUnitRPM})
	mgr.NotifyDeviceDisconnected()

	if len(ready.seenSpeeds) != 1 || ready.seenSpeeds[0] != 1850 || ready.disconnect != 1 {
		t.Fatalf("observer counts = %v / %d, want [1850] / 1", ready.seenSpeeds, ready.disconnect)
	}

	snap := mgr.Snapshot(map[string]bool{"rtss-osd": true, "legion-fnq": true})
	if len(snap) != 2 {
		t.Fatalf("snapshot len = %d, want 2", len(snap))
	}
	if snap[0].State != StateReady || !snap[0].Enabled || !snap[0].Running {
		t.Fatalf("ready plugin snapshot = %+v", snap[0])
	}
	if snap[1].State != StateUnsupported || snap[1].Enabled || snap[1].Reason != "non-legion host" {
		t.Fatalf("unsupported plugin snapshot = %+v", snap[1])
	}
}
