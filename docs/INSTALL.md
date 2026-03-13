# Installation

## One-command install

```bash
curl -fsSL https://raw.githubusercontent.com/derekurban/wabii-signal/main/install.sh | bash
```

PowerShell:

```powershell
irm https://raw.githubusercontent.com/derekurban/wabii-signal/main/install.ps1 | iex
```

The installer will:

1. Download the matching release asset from GitHub Releases.
2. Verify `checksums.txt`.
3. Verify the signed checksum payload when signature verification is enabled.
4. Install `wabsignal` into your user-local bin directory and optionally update `PATH`.

## Installer environment variables

Preferred variables:

- `WABSIGNAL_INSTALL_DIR`
- `WABSIGNAL_VERSION`
- `WABSIGNAL_AUTO_PATH`
- `WABSIGNAL_VERIFY_SIGNATURES`
- `WABSIGNAL_ALLOW_SOURCE_FALLBACK`
- `WABSIGNAL_COSIGN_VERSION`
- `WABSIGNAL_COSIGN_IDENTITY_RE`
- `WABSIGNAL_COSIGN_OIDC_ISSUER`

Compatibility aliases are still accepted for one transition window:

- `GRAFQUERY_*`
- `GRAFANA_QUERY_*`

## Fallback install

```bash
WABSIGNAL_ALLOW_SOURCE_FALLBACK=1 \
  curl -fsSL https://raw.githubusercontent.com/derekurban/wabii-signal/main/install.sh | bash
```

## Manual install from source

```bash
git clone https://github.com/derekurban/wabii-signal.git
cd wabii-signal
go build -o wabsignal .
install -m 0755 wabsignal ~/.local/bin/wabsignal
```

## Verify install

```bash
wabsignal --help
wabsignal version
```
