//go:build !windows

package main

func acquireCoreInstanceLock() (func(), bool, error) {
	return func() {}, false, nil
}
