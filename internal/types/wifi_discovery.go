package types

import (
	"context"
	"sync"
	"time"
)

const (
	WiFiDiscoveryModeNormal  = "normal"
	WiFiDiscoveryModeDeep    = "deep"
	WiFiDiscoveryModeDynamic = "dynamic"

	WiFiScanControlPause  = "pause"
	WiFiScanControlResume = "resume"
	WiFiScanControlCancel = "cancel"
)

type WiFiDiscoveryControl struct {
	mutex    sync.RWMutex
	paused   bool
	canceled bool
}

func NewWiFiDiscoveryControl() *WiFiDiscoveryControl {
	return &WiFiDiscoveryControl{}
}

func (c *WiFiDiscoveryControl) Reset() {
	if c == nil {
		return
	}
	c.mutex.Lock()
	c.paused = false
	c.canceled = false
	c.mutex.Unlock()
}

func (c *WiFiDiscoveryControl) Pause() {
	if c == nil {
		return
	}
	c.mutex.Lock()
	if !c.canceled {
		c.paused = true
	}
	c.mutex.Unlock()
}

func (c *WiFiDiscoveryControl) Resume() {
	if c == nil {
		return
	}
	c.mutex.Lock()
	if !c.canceled {
		c.paused = false
	}
	c.mutex.Unlock()
}

func (c *WiFiDiscoveryControl) Cancel() {
	if c == nil {
		return
	}
	c.mutex.Lock()
	c.canceled = true
	c.paused = false
	c.mutex.Unlock()
}

func (c *WiFiDiscoveryControl) IsCanceled() bool {
	if c == nil {
		return false
	}
	c.mutex.RLock()
	canceled := c.canceled
	c.mutex.RUnlock()
	return canceled
}

func (c *WiFiDiscoveryControl) Wait(ctx context.Context) bool {
	if c == nil {
		return ctx.Err() == nil
	}
	for {
		c.mutex.RLock()
		paused := c.paused
		canceled := c.canceled
		c.mutex.RUnlock()
		if canceled || ctx.Err() != nil {
			return false
		}
		if !paused {
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(100 * time.Millisecond):
		}
	}
}

type WiFiDiscoveryParams struct {
	Mode          string                `json:"mode"`
	Endpoint      string                `json:"endpoint,omitempty"`
	ProfileID     string                `json:"profileId,omitempty"`
	ProfileName   string                `json:"profileName,omitempty"`
	StateEndpoint string                `json:"stateEndpoint,omitempty"`
	TimeoutMs     int                   `json:"timeoutMs,omitempty"`
	Control       *WiFiDiscoveryControl `json:"-"`
}

type WiFiDiscoveryScope struct {
	Source         string `json:"source"`
	Network        string `json:"network"`
	CandidateCount int    `json:"candidateCount"`
}

type WiFiDiscoveredDevice struct {
	Name          string `json:"name"`
	ProfileID     string `json:"profileId,omitempty"`
	Transport     string `json:"transport"`
	Endpoint      string `json:"endpoint"`
	IP            string `json:"ip"`
	Port          string `json:"port,omitempty"`
	Source        string `json:"source"`
	Network       string `json:"network,omitempty"`
	Speed         int    `json:"speed,omitempty"`
	TargetSpeed   int    `json:"targetSpeed,omitempty"`
	Temperature   int    `json:"temperature,omitempty"`
	LatencyMs     int64  `json:"latencyMs,omitempty"`
	StateEndpoint string `json:"stateEndpoint,omitempty"`
}

type WiFiDiscoveryResult struct {
	Mode           string                 `json:"mode"`
	Found          bool                   `json:"found"`
	Canceled       bool                   `json:"canceled,omitempty"`
	Devices        []WiFiDiscoveredDevice `json:"devices,omitempty"`
	Scopes         []WiFiDiscoveryScope   `json:"scopes,omitempty"`
	CandidateCount int                    `json:"candidateCount"`
	ScannedCount   int                    `json:"scannedCount"`
	ElapsedMs      int64                  `json:"elapsedMs"`
	Error          string                 `json:"error,omitempty"`
}
