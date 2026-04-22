// heic_init.go — initialise the HEIC decoder at startup.
//
// The Rosca75/heic library (fork of gen2brain/heic) will, on first use,
// automatically try to load a dynamic libheif via purego and fall back to a
// WASM decoder (run under wazero) if the dynamic library is absent or too
// old. We force WASM mode when the system libheif is < 1.18 because older
// libheifs mis-decode iPhone HDR "tmap"-brand HEIC files.
package main

import (
	"log"

	"github.com/Rosca75/heic"
)

// initHEIC picks between dynamic libheif and the bundled WASM decoder.
// It should be called exactly once, before Wails opens its window.
func initHEIC() {
	// heic.Dynamic() returns a non-nil error when no dynamic libheif is
	// available. In that case the library will use WASM automatically and
	// there is nothing for us to do.
	if heic.Dynamic() != nil {
		return
	}
	// If dynamic libheif is present but its major.minor version is < 1.18,
	// force WASM mode. HDR iPhone HEIC files produced since iOS 16 use the
	// "tmap" brand, which older libheifs do not decode correctly.
	if !heicDynamicVersionAtLeast(1, 18) {
		heic.ForceWasmMode = true
		log.Println("[heic] Dynamic libheif < 1.18; using WASM decoder for full compatibility")
	}
}
