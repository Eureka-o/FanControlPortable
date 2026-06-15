package device

import (
	"fmt"
	"time"
)

const (
	deviceSendMaxAttempts = 3
	deviceSendRetryDelay  = 120 * time.Millisecond
)

func retryDeviceSend(label string, send func() error) error {
	if send == nil {
		return fmt.Errorf("%s send function is nil", label)
	}
	var lastErr error
	for attempt := 1; attempt <= deviceSendMaxAttempts; attempt++ {
		if err := send(); err == nil {
			return nil
		} else {
			lastErr = err
		}
		if attempt < deviceSendMaxAttempts {
			time.Sleep(time.Duration(attempt) * deviceSendRetryDelay)
		}
	}
	return fmt.Errorf("%s failed after %d attempts: %w", label, deviceSendMaxAttempts, lastErr)
}
