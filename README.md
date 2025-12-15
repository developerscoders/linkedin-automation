# LinkedIn Automation Engine

⚠️ **EDUCATIONAL PURPOSE ONLY** - This tool is for technical demonstration and learning purposes only. Using automation on LinkedIn violates their Terms of Service and may result in account suspension.

## Overview

A sophisticated Go-based LinkedIn automation tool demonstrating advanced browser automation, anti-detection techniques, and clean architecture patterns.

## Features

### Core Automation
- ✅ **Automated Authentication**: Login with session persistence and encrypted cookies.
- ✅ **Smart Search**: Profile collection with duplicate detection (Bloom Filter).
- ✅ **Connection Manager**: Automated requests with rate limiting and personalization.
- ✅ **Messaging System**: Template-based messaging and new connection detection.

### Anti-Detection (8+ Techniques Implemented)
1. **Human-like Mouse Movement**: Cubic Bézier curves with variable speed and overshoot.
2. **Randomized Timing**: Gaussian-distributed delays for reading, thinking, and acting.
3. **Browser Fingerprint Masking**: Overriding navigator properties (WebDriver, Plugins, Hardware Concurrency).
4. **Natural Scrolling**: Acceleration/deceleration with random pauses and scroll-backs.
5. **Realistic Typing**: Variable WPM, typo simulation (2%), and corrections.
6. **Mouse Hovering**: Random hover events over interactive elements.
7. **Activity Scheduling**: Strict business hours enforcement with break patterns.
8. **Adaptive Rate Limiting**: Circuit breaker and exponential backoff.

## Architecture

```
linkedin-automation/
├── cmd/bot/              # CLI Entry Point
├── internal/
│   ├── auth/             # Login & Cookie Encryption
│   ├── browser/          # Rod Management & Stealth Injection
│   ├── connection/       # Request Logic, Rate Limiter, Tracker
│   ├── messaging/        # Sender, Templates, Detector
│   ├── search/           # Search Logic, Parser, Deduplication
│   ├── stealth/          # Core Stealth Primitives (Mouse, Timing, etc.)
│   ├── storage/          # SQLite DB & Repositories
│   └── config/           # Config Loading
├── pkg/logger/           # Structured Logging
└── configs/              # YAML Configuration
```

## Installation

### Prerequisites
- Go 1.21+
- Chrome/Chromium Browser

### Setup

1. **Clone the repository**
   ```bash
   git clone <repo-url>
   cd linkedin-automation
   ```

2. **Install Dependencies**
   ```bash
   go mod download
   ```
   *Note: If `go` command is missing ensuring Go is installed and in PATH.*

3. **Configuration**
   Copy the example config:
   ```bash
   cp configs/config.example.yaml configs/config.yaml
   cp .env.example .env
   ```
   Edit `.env` with your LinkedIn credentials.

## Usage

### Run Full Automation
```bash
go run cmd/bot/main.go run
```

### Individual Commands
- **Search Profiles**: `go run cmd/bot/main.go search`
- **Send Requests**: `go run cmd/bot/main.go connect`
- **Send Messages**: `go run cmd/bot/main.go message`

## Configuration Options

Customize behavior in `configs/config.yaml`:
- **Limits**: `daily_requests`, `hourly_requests`.
- **Stealth**: `mouse_speed`, `typing_wpm`, `typo_probability`.
- **Schedule**: `work_days`, `start_hour`, `end_hour`.

## Disclaimer

This software is provided "as is", without warranty of any kind. The authors are not responsible for any consequences of using this software.
