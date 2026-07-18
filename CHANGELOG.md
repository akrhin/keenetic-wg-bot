# Changelog

## v0.5.0 (2026-07-18)

### Security
- **install.sh**: real IP `192.168.1.139` → placeholder (×4)
- **README.md**: real IP in proxy example → placeholder
- **config.toml.example**: synthetic bot token → `YOUR_BOT_TOKEN`

### Documentation
- **README**: added AI disclaimer & license section
- **AGENTS.md**: CI pipeline diagram → parallel (was sequential)
- **ARCHITECTURE.md**: `2 jobs` → `4 jobs`, `every package has tests` → `most packages`

### Code Quality
- **internal/proxy/proxy_test.go**: 7 new unit tests
- **internal/bot/bot_test.go**: 6 white-box tests (`formatStatus`)
- **.golangci.yml**: v2 config with gosec excludes
- **go.mod**: BurntSushi/toml v1.4.0 → v1.6.0
- **Total tests**: 28 (was 15) across 7 packages

## v0.4.0 (2026-07-14)

- Pipeline audit: CI, docs, gitignore, config example
- Go 1.25
- Full CI/CD (lint + gosec + gitleaks + vet + govulncheck + test + build + release)

## v0.3.0 (2026-07-09)

- Inline keyboard interface
- Proxy configuration (SOCKS5)
- WireGuard management via wg-quick

## v0.2.x (2026-07-07..08)

- Initial releases
- Telegram bot with inline buttons
- Wake-on-LAN support
- Scheduler (auto-off timer)
