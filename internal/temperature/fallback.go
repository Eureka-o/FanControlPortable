package temperature

import (
	"sync"
	"time"
)

const (
	fallbackFreshInterval = 5 * time.Second
	fallbackBackoffStart  = 15 * time.Second
	fallbackMaxInterval   = 60 * time.Second
)

type fallbackReading struct {
	cpuTemp int
	gpuTemp int
}

func (r fallbackReading) usable() bool {
	return r.cpuTemp > 0 || r.gpuTemp > 0
}

type fallbackState struct {
	mutex        sync.Mutex
	reading      fallbackReading
	at           time.Time
	interval     time.Duration
	needCPU      bool
	gpuNotPolled bool
	primed       bool
}

func (s *fallbackState) next(usable bool) time.Duration {
	if usable {
		return fallbackFreshInterval
	}
	if s.interval < fallbackBackoffStart {
		return fallbackBackoffStart
	}
	return min(s.interval*2, fallbackMaxInterval)
}

func (r *Reader) readFallback(gpuNotPolled bool) fallbackReading {
	reading := fallbackReading{cpuTemp: r.readSensorCPUTemperature()}
	needCPU := reading.cpuTemp <= 0
	if !needCPU && gpuNotPolled {
		return reading
	}

	external := r.readThrottledFallback(needCPU, gpuNotPolled)
	if needCPU {
		reading.cpuTemp = external.cpuTemp
	}
	reading.gpuTemp = external.gpuTemp
	return reading
}

func (r *Reader) readThrottledFallback(needCPU, gpuNotPolled bool) fallbackReading {
	now := readTimeNow()
	r.fallback.mutex.Lock()
	defer r.fallback.mutex.Unlock()

	selectionChanged := r.fallback.primed && (r.fallback.needCPU != needCPU || r.fallback.gpuNotPolled != gpuNotPolled)
	if r.fallback.primed && !selectionChanged && now.Sub(r.fallback.at) < r.fallback.interval {
		return r.fallback.reading
	}

	var reading fallbackReading
	var wg sync.WaitGroup
	if needCPU {
		wg.Add(1)
		go func() {
			defer wg.Done()
			reading.cpuTemp = r.readWindowsCPUTemp()
		}()
	}
	if !gpuNotPolled {
		wg.Add(1)
		go func() {
			defer wg.Done()
			reading.gpuTemp = r.readGPUTemperature()
		}()
	}
	wg.Wait()

	r.fallback.reading = reading
	r.fallback.at = now
	r.fallback.interval = r.fallback.next(reading.usable())
	r.fallback.needCPU = needCPU
	r.fallback.gpuNotPolled = gpuNotPolled
	r.fallback.primed = true
	return reading
}
