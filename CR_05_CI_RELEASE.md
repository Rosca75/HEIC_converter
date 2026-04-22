# CR_05 — CI & Release (GitHub Actions)

**Goal**: two workflow files that build the Wails app on Windows and Linux on
every tag push, attach both binaries to a GitHub Release, and run `go vet` +
`go test` on every commit to main. Mirror the `dedup-photos/.github/workflows/`
shape exactly — it is known to work on Ubuntu 24.04.

**Depends on**: phases 1–4 complete. The code must build locally with
`wails build` on both Windows and Linux before CI is turned on, otherwise the
first tag push will fail in the cloud and waste Actions minutes.

---

## 5.1 Create `.github/workflows/ci.yml`

```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  build:
    name: Build & Test
    runs-on: ${{ matrix.os }}
    strategy:
      fail-fast: false
      matrix:
        os: [ubuntu-latest, windows-latest, macos-latest]

    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version: "1.24"

      - name: Install Linux system dependencies
        # Required for `go build` on Linux because Wails imports GTK/WebKit
        # headers transitively. Matches the release workflow to keep CI and
        # release environments consistent.
        if: runner.os == 'Linux'
        run: |
          sudo apt-get update
          sudo apt-get install -y libgtk-3-dev libwebkit2gtk-4.1-dev pkg-config

      - name: Download dependencies
        run: go mod download

      - name: Run go vet
        run: go vet ./...

      - name: Check formatting
        if: runner.os != 'Windows'
        run: |
          if [ -n "$(gofmt -l .)" ]; then
            echo "The following files are not formatted:"
            gofmt -l .
            exit 1
          fi

      - name: Build
        run: go build -v ./...

      - name: Test
        run: go test ./...
```

> **Difference from dedup-photos ci.yml**: we install Linux system
> dependencies before the build step. dedup-photos gets away without them
> because its non-wails build target is pure Go; our `go build ./...` pulls
> in Wails, which requires the GTK/WebKit headers even on a plain build.

---

## 5.2 Create `.github/workflows/release.yml`

```yaml
name: Release

# Triggers when a version tag is pushed (e.g. `git tag v1.1.0 && git push origin v1.1.0`)
on:
  push:
    tags:
      - 'v*'

jobs:

  # ──────────────────────────────────────────────────────────────
  # Windows build — produces heic-converter.exe
  # Must run on windows-latest: Wails embeds WebView2 which
  # requires Windows SDK headers at compile time.
  # ──────────────────────────────────────────────────────────────
  build-windows:
    runs-on: windows-latest

    steps:
      - name: Checkout repository
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.22'

      - name: Set up Node.js (required by Wails to bundle the frontend)
        uses: actions/setup-node@v4
        with:
          node-version: '20'

      - name: Install Wails CLI
        run: go install github.com/wailsapp/wails/v2/cmd/wails@latest

      - name: Build Windows binary
        # -o writes the output to build/bin/ with the given filename.
        run: wails build -platform windows/amd64 -o heic-converter.exe

      - name: Upload Windows binary as workflow artifact
        uses: actions/upload-artifact@v4
        with:
          name: windows-binary
          path: build/bin/heic-converter.exe

  # ──────────────────────────────────────────────────────────────
  # Linux build — produces heic-converter-linux (ELF amd64)
  # Must run on ubuntu-latest: Wails uses WebKit2GTK which requires
  # GTK3 + WebKit2GTK dev headers at compile time.
  #
  # Ubuntu 24.04 ships webkit2gtk-4.1, not 4.0. The Wails fix is to
  # install libwebkit2gtk-4.1-dev and pass `-tags webkit2_41` to the
  # build command to activate Wails' built-in 4.1 support.
  # ──────────────────────────────────────────────────────────────
  build-linux:
    runs-on: ubuntu-latest

    steps:
      - name: Checkout repository
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.22'

      - name: Set up Node.js (required by Wails to bundle the frontend)
        uses: actions/setup-node@v4
        with:
          node-version: '20'

      - name: Install Linux system dependencies
        # libgtk-3-dev           -> GTK3 headers (Wails window chrome)
        # libwebkit2gtk-4.1-dev  -> WebKit2GTK headers (Ubuntu 24.04 package name)
        # pkg-config             -> used by cgo to locate the above libraries
        run: |
          sudo apt-get update
          sudo apt-get install -y libgtk-3-dev libwebkit2gtk-4.1-dev pkg-config

      - name: Install Wails CLI
        run: go install github.com/wailsapp/wails/v2/cmd/wails@latest

      - name: Build Linux binary
        # -tags webkit2_41 activates Wails' built-in webkit2gtk-4.1 support,
        # which is required on Ubuntu 24.04 where webkit2gtk-4.0 no longer exists.
        run: wails build -platform linux/amd64 -o heic-converter-linux -tags webkit2_41

      - name: Upload Linux binary as workflow artifact
        uses: actions/upload-artifact@v4
        with:
          name: linux-binary
          path: build/bin/heic-converter-linux

  # ──────────────────────────────────────────────────────────────
  # Release — waits for both builds, then creates the GitHub
  # Release and attaches the two binaries as downloadable assets.
  # ──────────────────────────────────────────────────────────────
  release:
    needs: [build-windows, build-linux]
    runs-on: ubuntu-latest
    permissions:
      contents: write   # required to create releases and upload assets

    steps:
      - name: Download Windows binary
        uses: actions/download-artifact@v4
        with:
          name: windows-binary
          path: artifacts/

      - name: Download Linux binary
        uses: actions/download-artifact@v4
        with:
          name: linux-binary
          path: artifacts/

      - name: Create GitHub Release and attach binaries
        uses: softprops/action-gh-release@v2
        with:
          files: |
            artifacts/heic-converter.exe
            artifacts/heic-converter-linux
          # Auto-generates release notes from commits and merged PRs since the last tag
          generate_release_notes: true
```

---

## 5.3 Add / verify `.gitignore`

Copy the same file from `dedup-photos/.gitignore` with two small substitutions
(replace the binary names). If the repo already has a `.gitignore`, merge;
do not overwrite blindly.

```gitignore
# Binary outputs
heic-converter
heic-converter.exe
heic-converter-linux
build/bin/

# Wails build scaffolding
build/
frontend/dist/

# Go build artifacts
*.o
*.a
*.so
*.test
*.out

# IDE files
.vscode/
.idea/
*.swp
*.swo
*~

# OS files
.DS_Store
Thumbs.db
Desktop.ini

# Test coverage
coverage.out
coverage.html
*.coverprofile

# Vendor directory
vendor/

# Sample HEICs used in local tests — too large to ship in-repo
samples/
converter/testdata/*.heic
converter/testdata/*.HEIC
```

---

## 5.4 Local pre-flight before the first tag push

Do **not** push the first tag until all of the following succeed on
Oscar's actual Windows machine:

1. `go mod tidy && go build ./...` — clean compile.
2. `wails build -platform windows/amd64 -o heic-converter.exe` — produces a
   working binary in `build/bin/`.
3. Launch `build/bin/heic-converter.exe`, drag in a HEIC folder, verify the
   listing + conversion both work without a `magick.exe` anywhere on PATH.
4. On a Linux VM (or WSL Ubuntu 24.04):
   - `sudo apt-get install -y libgtk-3-dev libwebkit2gtk-4.1-dev pkg-config`
   - `wails build -platform linux/amd64 -o heic-converter-linux -tags webkit2_41`
   - Run the binary: `./build/bin/heic-converter-linux`. Same smoke test.

If either platform build fails locally, fixing it in CI by trial-and-error
burns Actions minutes; fix it locally first.

---

## 5.5 Tagging the first release

Once local pre-flight passes:

```bash
git checkout main
git pull
git tag v0.2.0 -m "Pure-Go HEIC pipeline; parallel conversion; subfolder scan"
git push origin v0.2.0
```

Watch the Actions tab at `https://github.com/Rosca75/HEIC_converter/actions`.
Both build jobs must succeed before the `release` job runs; if one fails,
delete the tag locally and remotely (`git tag -d v0.2.0 && git push origin :refs/tags/v0.2.0`),
fix, and re-tag.

---

## 5.6 Acceptance checks for this phase

1. Pushing a commit to `main` triggers `ci.yml` and it turns green on all
   three matrix OSes.
2. Pushing a tag `vX.Y.Z` triggers `release.yml`; both `build-windows` and
   `build-linux` jobs succeed; the `release` job creates a new GitHub
   Release page with `heic-converter.exe` and `heic-converter-linux`
   attached.
3. Downloading the Windows .exe from the Release page and running it on a
   fresh Windows 11 machine with no ImageMagick installed works — this is
   the whole point of the pure-Go rewrite.
4. Downloading the Linux binary and running it on Ubuntu 24.04 works (the
   user still needs `libgtk-3-0` and `libwebkit2gtk-4.1-0` runtime
   libraries, which are standard on most desktop installs).
