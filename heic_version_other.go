//go:build !linux && !darwin

package main

// heicDynamicVersionAtLeast returns true on platforms where we do not perform
// a dynamic libheif version probe (Windows, BSD, etc.). On Windows the
// Rosca75/heic library uses its bundled WASM decoder directly, so initHEIC()
// will have returned early after heic.Dynamic() reported absence.
func heicDynamicVersionAtLeast(major, minor int) bool {
	return true
}
