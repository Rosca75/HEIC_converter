# CR_01 — Dependencies & HEIC decoder initialisation

**Goal**: swap the decoder stack. Out: `github.com/jdeng/goheif` and
`github.com/rwcarlsen/goexif`. In: `github.com/Rosca75/heic`,
`github.com/bep/imagemeta`, `github.com/gen2brain/webp`,
`golang.org/x/image` (for `draw`, `tiff`). Also add the
`github.com/ebitengine/purego` dependency used by the libheif version probe on
Linux and macOS.

Do **not** delete any old code in this phase — only add/adjust dependencies
and the `initHEIC()` wrapper. Code that still uses `goheif` and `goexif` will
keep compiling because both packages remain in `go.sum` until phase 2 removes
them.

---

## 1.1 Rewrite `go.mod`

Replace the entire contents of `go.mod` with:

```go
module heic-converter

go 1.22.3

toolchain go1.24.7

require (
	github.com/Rosca75/heic v0.1.0
	github.com/bep/imagemeta v0.17.1
	github.com/ebitengine/purego v0.10.0
	github.com/gen2brain/webp v0.5.5
	github.com/wailsapp/wails/v2 v2.12.0
	golang.org/x/image v0.23.0
)

// Indirect dependencies are left to `go mod tidy` to populate.
```

Then run (on a machine that has outbound internet to `proxy.golang.org`):

```
go mod tidy
go build ./...
```

Expected outcome: `go.sum` is fully populated, `go build` succeeds. If
`go mod tidy` fails because `Rosca75/heic` has a different tag, query the
module proxy for the latest version:

```
curl -s https://proxy.golang.org/github.com/!rosca75/heic/@v/list
```

and substitute the newest `v0.x.y` tag.

> **PwC note for Oscar**: this step should be done at home. The PwC proxy
> blocks `proxy.golang.org` for most packages. If you must do it at PwC, set
> `GOPROXY=direct` and `GOSUMDB=off`, and configure git to disable SSL
> verification for GitHub (`git config --global http.sslVerify false`), same
> as you did for the portable-Git + Claude Code setup.

### Version pinning rationale

| Module | Version | Why this version |
|--------|---------|------------------|
| `Rosca75/heic` | `v0.1.0` | Matches what `dedup-photos` (preview) uses. |
| `bep/imagemeta` | `v0.17.1` | Matches `dedup-photos`. |
| `ebitengine/purego` | `v0.10.0` | Matches `dedup-photos`; required for libheif version probe. |
| `gen2brain/webp` | `v0.5.5` | Same WASM + purego pattern as `Rosca75/heic`. Any `v0.5.x` is fine. |
| `golang.org/x/image` | `v0.23.0` | Matches `dedup-photos`. Provides `draw`, `tiff`. |
| `wailsapp/wails/v2` | `v2.12.0` | **Do not bump** — project rule 5.7. |

---

## 1.2 Create `heic_init.go` at the project root

This file wraps the decoder initialisation and is called once from `main.go`.
It is placed at the root (package `main`) — not in `converter/` — because it
needs to run before any Wails window opens, and it conditionally imports
`purego` only on Unix via the platform files below.

```go
// heic_init.go — initialise the HEIC decoder at startup.
//
// The Rosca75/heic library (fork of gen2brain/heic) will, on first use,
// automatically try to load a dynamic libheif via purego and fall back to a
// WASM decoder (run under wazero) if the dynamic library is absent or too
// old. We force WASM mode when the system libheif is < 1.18 because older
// libheifs mis-decode iPhone HDR "tmap"-brand HEIC files — this is the same
// workaround dedup-photos applies.
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
```

---

## 1.3 Create `heic_version_linux.go`

```go
//go:build linux

package main

import "github.com/ebitengine/purego"

// heicDynamicVersionAtLeast returns true if the dynamically loaded libheif is
// at least the given major.minor version. If the library cannot be opened,
// returns true (safe default: do not force WASM mode unnecessarily).
func heicDynamicVersionAtLeast(major, minor int) bool {
	handle, err := purego.Dlopen("libheif.so", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return true
	}
	defer purego.Dlclose(handle)

	var getMajor func() uint32
	var getMinor func() uint32
	purego.RegisterLibFunc(&getMajor, handle, "heif_get_version_number_major")
	purego.RegisterLibFunc(&getMinor, handle, "heif_get_version_number_minor")

	maj := int(getMajor())
	min := int(getMinor())
	if maj != major {
		return maj > major
	}
	return min >= minor
}
```

---

## 1.4 Create `heic_version_darwin.go`

```go
//go:build darwin

package main

import "github.com/ebitengine/purego"

// heicDynamicVersionAtLeast returns true if the dynamically loaded libheif is
// at least the given major.minor version. Checks both the system library
// search path and the Homebrew default at /opt/homebrew/lib.
func heicDynamicVersionAtLeast(major, minor int) bool {
	var handle uintptr
	var err error
	for _, lib := range []string{"libheif.dylib", "/opt/homebrew/lib/libheif.dylib"} {
		handle, err = purego.Dlopen(lib, purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err == nil {
			break
		}
	}
	if err != nil {
		return true
	}
	defer purego.Dlclose(handle)

	var getMajor func() uint32
	var getMinor func() uint32
	purego.RegisterLibFunc(&getMajor, handle, "heif_get_version_number_major")
	purego.RegisterLibFunc(&getMinor, handle, "heif_get_version_number_minor")

	maj := int(getMajor())
	min := int(getMinor())
	if maj != major {
		return maj > major
	}
	return min >= minor
}
```

---

## 1.5 Create `heic_version_other.go`

```go
//go:build !linux && !darwin

package main

// heicDynamicVersionAtLeast returns true on platforms where we do not perform
// a dynamic libheif version probe (Windows, BSD, etc.). On Windows the
// Rosca75/heic library uses its bundled WASM decoder directly, so initHEIC()
// will have returned early after heic.Dynamic() reported absence.
func heicDynamicVersionAtLeast(major, minor int) bool {
	return true
}
```

---

## 1.6 Patch `main.go`

Add a single call to `initHEIC()` at the top of `main()`:

```go
func main() {
	initHEIC()          // ← add this line

	app := NewApp()
	err := wails.Run(&options.App{
		// ... existing options unchanged ...
	})
	if err != nil {
		panic(err)
	}
}
```

No other changes to `main.go`.

---

## 1.7 Acceptance checks for this phase

Run each of the following and confirm success before proceeding to phase 2:

1. `go build ./...` — compiles with no errors on Linux, macOS, and Windows.
2. `go vet ./...` — no vet complaints.
3. `gofmt -l .` — prints nothing (all files formatted).
4. Run the app with `wails dev`. It still opens the window and the old
   ImageMagick-based behaviour still works (phase 2 removes that code). The
   only visible change at runtime is a possible `[heic] Dynamic libheif …`
   log line on startup.
5. `go.mod` contains `github.com/Rosca75/heic`, `github.com/bep/imagemeta`,
   `github.com/ebitengine/purego`, `github.com/gen2brain/webp`, and
   `golang.org/x/image`. It still contains `jdeng/goheif` and
   `rwcarlsen/goexif` (these are removed in phase 2).

If any check fails, stop and fix before moving on. In particular, if
`go build` fails on Windows because `purego` cannot be built for `GOOS=windows`,
verify that `heic_version_linux.go` and `heic_version_darwin.go` carry their
`//go:build` constraints correctly — the `purego` import must only be visible
on Linux/macOS builds.
