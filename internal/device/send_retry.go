package device

import (
	"context"
	"fmt"
	"time"
)

const (
	deviceSendMaxAttempts = 3
	deviceSendRetryDelay  = 120 * time.Millisecond
)

func retryDeviceSend(label string, send func() error) error {
	return retryDeviceSendContext(context.Background(), label, send)
}

func retryDeviceSendContext(ctx context.Context, label string, send func() error) error {
	if send == nil {
		return fmt.Errorf("%s send function is nil", label)
	}
	var lastErr error
	for attempt := 1; attempt <= deviceSendMaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := send(); err == nil {
			return nil
		} else {
			lastErr = err
		}
		if attempt < deviceSendMaxAttempts {
			timer := time.NewTimer(time.Duration(attempt) * deviceSendRetryDelay)
			select {
			case <-ctx.Done():
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				return ctx.Err()
			case <-timer.C:
			}
		}
	}
	return fmt.Errorf("%s failed after %d attempts: %w", label, deviceSendMaxAttempts, lastErr)
}
