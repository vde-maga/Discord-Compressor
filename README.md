# 🎬 Discord Video Compressor

A lightweight, robust, and universal Command-Line Interface (CLI) tool designed to compress videos to strictly meet Discord's file size limits (e.g., 20MB or 50MB) using modern codecs (AV1/VP9 + Opus).

Built with **Go**, engineered with **Clean Architecture** principles, and packaged with **Nix** for absolute reproducibility.

[![Go Report Card](https://goreportcard.com/badge/github.com/vde-maga/Discord-Compressor)](https://goreportcard.com/report/github.com/vde-maga/Discord-Compressor)
[![Build and Release](https://github.com/vde-maga/Discord-Compressor/actions/workflows/release.yml/badge.svg)](https://github.com/vde-maga/Discord-Compressor/actions/workflows/release.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

---

## ✨ Key Features

- **Dynamic Bitrate Calculation**: Intelligently calculates the exact video and audio bitrates, and adapts the resolution (720p, 480p, 360p) to ensure the final file strictly respects the target size limit with a 5% safety margin.
- **Smart Fast Path**: Detects if the input file is already an optimized `.webm` and under the size limit. If so, it performs an instant, lossless OS-level copy, saving CPU cycles and time.
- **Modern Codecs**: Defaults to `libsvtav1` (AV1) for maximum compression efficiency, with seamless fallback to `libvpx-vp9` via the `--fast` or `--vp9` flags.
- **2-Pass Encoding**: Guarantees the highest possible visual quality within the strict file size constraint.
- **Zero Bloat**: Written in pure Go using only the standard library. The compiled binary is ~2.6MB, statically linked, and requires no runtime (like Python or Node.js).
- **Fail-Fast Safety**: Immediately aborts and warns the user if a video's duration makes the target size mathematically impossible, preventing hours of wasted CPU time.

---

## 📋 Prerequisites

This tool acts as an intelligent orchestrator. It requires **FFmpeg** (which includes `ffprobe`) to be installed on your system, compiled with support for:

- `libsvtav1` (recommended for best results)
- `libvpx-vp9`
- `libopus`

---

## 🚀 Installation

### Option 1: Pre-compiled Binaries (Recommended)

Download the latest statically linked binary for your OS (Linux, macOS, or Windows) directly from the [Releases page](https://github.com/vde-maga/Discord-Compressor/releases).

### Option 2: Nix (Reproducible Environment)

If you use Nix, you can run the tool directly without installing it globally, or spawn a fully configured development environment:

```bash
# Run directly from the repository
nix run github:vde-maga/Discord-Compressor -- <video.mp4>

# Enter the development shell (includes Go, FFmpeg, linters, etc.)
nix develop
```

### Option 3: Build from Source

```bash
git clone https://github.com/vde-maga/Discord-Compressor.git
cd Discord-Compressor

# Build an optimized, stripped binary (~2.6MB)
make build
# Or manually: go build -ldflags="-s -w" -o discord-compress .
```

---

## 💻 Usage

```bash
# Basic usage (defaults to AV1, 20MB target)
./discord-compress video.mp4

# Compress for Discord Nitro (50MB limit)
./discord-compress video.mp4 --target-mb 50

# Prioritize encoding speed over maximum compression (uses VP9)
./discord-compress video.mp4 --fast

# View all available options
./discord-compress --help
```

**Help Output:**

```text
Usage of discord-compress:
  -fast
        Modo rápido (prioriza velocidade)
  -target-mb int
        Limite de tamanho em MB (default 20)
  -vp9
        Forçar o uso do codec VP9
```

---

## 🏗️ Architecture & Engineering Principles

This project was built to showcase professional software engineering practices, moving beyond a simple scripting approach:

1. **Separation of Concerns (SOC)**: The codebase is strictly modularized:
   - `domain.go`: Pure business logic (bitrate math, resolution rules). Easily testable without external dependencies.
   - `ffmpeg.go`: Infrastructure layer. Handles `ffprobe` JSON parsing and `ffmpeg` process orchestration with concurrent stdout reading for the progress bar.
   - `main.go`: Application layer. Handles CLI argument parsing (using Go's robust `flag` package), UI rendering, and the retry orchestration loop.
2. **SOLID & DRY**: Responsibilities are isolated. The retry logic reuses the same argument builder, preventing code duplication.
3. **YAGNI (You Aren't Gonna Need It)**: No external Go frameworks or heavy UI libraries were used. The standard library provides everything needed for a robust, fast, and maintainable CLI.
4. **Reliability**: Includes unit tests (`domain_test.go`) to mathematically validate compression parameters without needing to invoke FFmpeg.

---

## 🧪 Testing & Development

The repository includes a `Makefile` to streamline common development tasks:

```bash
make test   # Runs unit tests (go test -v ./...)
make build  # Compiles the optimized binary
make clean  # Removes local binaries and FFmpeg 2-pass logs
```

---

## 📄 License

Distributed under the MIT License. See [`LICENSE`](LICENSE) for more information.

---

> **💡 Pro Tip:** If you are compressing a video that is _already_ a `.webm` file and under your target size, the tool will detect it and perform an instant, lossless copy instead of re-encoding!
