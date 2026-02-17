---
name: run-script
description: Execute bash scripts for server management (start/stop/restart/status), builds, and testing. Use for server lifecycle operations and build tasks.
---

# Run Script Skill

Execute bash scripts in the `scripts/` directory for server management, builds, and testing.

## Available Scripts

| Script | Description |
|---|---|
| `scripts/run.sh` | Server lifecycle (start/stop/restart/status) |
| `scripts/build.sh` | Build binaries, configs, and assets |
| `scripts/verify-auth.sh` | Verify authentication setup |
| `scripts/test-scripts.sh` | Test script execution |
| `scripts/create-favicon.sh` | Generate favicon assets |

## Server Management

### Start server
```bash
./scripts/run.sh start
```

### Stop server
```bash
./scripts/run.sh stop
```

### Restart server (rebuild + reload)
```bash
./scripts/run.sh restart
```

### Check server status
```bash
./scripts/run.sh status
```

## Build

Build binaries, configs, and assets to `bin/`:
```bash
./scripts/build.sh
```

## Health Check

Wait for server to be healthy:
```bash
until curl -sf http://localhost:${PORTAL_PORT:-8500}/api/health > /dev/null; do sleep 1; done
```

## Output Directory

Scripts that generate output should write to `.kilocode/workdir/`:
```bash
mkdir -p .kilocode/workdir
./scripts/run.sh status > .kilocode/workdir/status.txt
```

## Rules

1. **Use `scripts/run.sh restart`** after code changes to rebuild and reload
2. **Wait for health** before running tests or browser checks
3. **Check status** if server seems unresponsive
4. **Output to `.kilocode/workdir/`** for any generated files
