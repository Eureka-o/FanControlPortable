package device

// BlockWrites waits for the current device operation and rejects new writes.
func (m *Manager) BlockWrites() {
	m.writesBlocked.Store(true)
	m.operationControl.cancelActive()
	m.mutex.Lock()
	m.mutex.Unlock()
}

func (m *Manager) UnblockWrites() {
	m.mutex.Lock()
	m.writesBlocked.Store(false)
	m.mutex.Unlock()
}
