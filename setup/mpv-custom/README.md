# Custom mpv build

`mpv-0.36.0-rpi5-bookworm-arm64` is a pre-built `mpv` binary, deployed by
`rpi-setup.sh` to `/usr/local/bin/mpv-0.36.0-custom` and referenced directly
by `services/media-player.service`.

## Why this exists

The `mpv` shipped by Debian 12 "Bookworm" (which the current Raspberry Pi OS
is based on) is 0.35.1. On a Raspberry Pi 5, using that version with
`hwdec=drm` or `hwdec=drm-copy` to hardware-decode 4K HEVC video would
reliably **hang the entire Pi** (not just mpv — the whole system stopped
responding, including SSH, and needed a physical power cycle) as soon as an
actively hardware-decoding mpv process was terminated or told to load a new
file. Software-only decode avoids the hang but can't keep up with 4K HEVC in
real time on this hardware, causing severe stutter.

mpv 0.36.0 does not have this problem — verified repeatedly (SIGTERM and IPC
`quit` while hardware-decoding, and back-to-back `loadfile` transitions
between 4K HEVC files, all clean) against the same Pi 5 / kernel / ffmpeg.

mpv 0.36.0 is the *newest* mpv release still compatible with Bookworm's
system libraries:
- mpv >= 0.37 requires `libavcodec >= 60.31.102` (FFmpeg 6+); Bookworm ships
  FFmpeg 5.1.9 (`libavcodec` 59.x).
- mpv >= 0.37 also requires `libplacebo >= 6.338`; Bookworm ships 4.208.0.

0.36.0 is the last release where both of those are still optional/satisfied
by what's already in Bookworm's repos, so it builds against the *exact same*
system `ffmpeg`/`libplacebo`/etc. that the rest of the OS uses — no other
library needed rebuilding.

## Portability

This binary is dynamically linked against ~100 Bookworm-specific shared
libraries (`libavcodec.so.59`, `libplacebo.so.208`, ...). It will only run on
another device with the same OS base (Raspberry Pi OS / Debian 12 Bookworm,
arm64). It will **not** run as-is on a Raspberry Pi OS Trixie (Debian 13)
device — that OS ships mpv ~0.40 directly via apt, so this custom build
shouldn't be needed there; if the hang described above still reproduces on
Trixie, rebuild against that system's libraries instead (see below).

## How to rebuild

```bash
sudo apt-get update
echo "deb-src http://deb.debian.org/debian bookworm main contrib non-free non-free-firmware" \
  | sudo tee /etc/apt/sources.list.d/sources-temp.list
sudo apt-get update
sudo apt-get build-dep -y mpv
sudo apt-get install -y libplacebo-dev

git clone --depth 1 --branch v0.36.0 https://github.com/mpv-player/mpv.git mpv-build
cd mpv-build
meson setup build -Dlibmpv=false -Dtests=false \
  -Dmanpage-build=disabled -Dhtml-build=disabled -Dpdf-build=disabled
ninja -C build
strip build/mpv
```

The resulting `build/mpv` is the file checked in here.
