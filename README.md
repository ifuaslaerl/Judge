# Judge: Self-Hosted Competitive Programming Platform

A lightweight, self-hosted judging platform designed for running local contests or training sessions.
Built with Go, SQLite, and `isolate` for secure sandboxing.

## Prerequisites (Arch Linux)

This guide assumes you are running Arch Linux. You will need the standard development tools,
the Go language, and the `isolate` sandbox (available via AUR).

```bash
# 1. Install base dependencies and Go
sudo pacman -S go base-devel git openssl

# 2. Install Cloudflare Tunnel (for public access)
sudo pacman -S cloudflared

# 3. Install Isolate (Sandbox) from AUR
# You can use an AUR helper like yay or paru:
yay -S isolate-git
```

## Installation & Setup

### 1. Clone the Repository
```bash
git clone https://github.com/ifuaslaerl/Judge.git
cd Judge
```

### 2. Create Directory Structure
The application requires specific folders for storage and certificates.
```bash
mkdir -p storage/db
mkdir -p storage/submissions
mkdir -p certs
```

### 3. Generate TLS Certificates
The server runs strictly on HTTPS (port 8443). You must generate self-signed certificates.
```bash
openssl req -x509 -newkey rsa:4096 -sha256 -days 3650 \
  -nodes -keyout certs/server.key -out certs/server.crt \
  -subj "/CN=localhost"
```

### 4. Initialize Dependencies
```bash
go mod tidy
```

## Running the Server

Start the application. On the first run, it will automatically initialize the SQLite database.

```bash
go run cmd/server/main.go
```

* **Local Access:** `https://localhost:8443` (You will see a browser warning due to the self-signed cert).
* **Default Port:** `:8443`

## Public Access (Cloudflare Tunnel)

To expose the server to the internet safely using Cloudflare's Free Tier (Zero Trust), use the following command.
This establishes an encrypted tunnel while instructing Cloudflare to trust your self-signed certificate.

```bash
cloudflared tunnel --url https://localhost:8443 --no-tls-verify
```

* **Note:** If using a Quick Tunnel (no domain), the URL will change every time you restart the terminal command.
* **Limit:** The free plan supports up to 50 concurrent users.

## Maintenance & Administration

The application includes built-in CLI commands for managing the contest.

### Flush Sessions
Logs out all users by clearing the session table. Useful if tokens are compromised.
```bash
go run cmd/server/main.go --flush-sessions
```

### Factory Reset (Weekly Wipe)
**DANGER:** This deletes ALL submissions, users, and session data. It effectively resets the platform for a new contest.
```bash
go run cmd/server/main.go --wipe-all
```

### Orphaned File Cleanup (The Reaper)
The server automatically scans for and deletes "orphaned" submission files (files with no DB record) every time it boots.

## Architecture Notes

* **Database:** SQLite in WAL mode (located in `storage/db/judge.sqlite`).
* **Queue:** In-memory buffered channel (capacity 5000). If the server crashes, pending submissions are lost.
* **Sandbox:** Uses `isolate` with a 2.0s time limit and 256MB memory limit per submission.
