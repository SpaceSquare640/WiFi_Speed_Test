//go:build !windows && !linux && !darwin

package netinfo

func platformProvider() Provider { return unsupported{} }
