```
   ____ ____  __  __    ______                  
  / ___/ ___||  \/  |  / / ___|_ __ ___  _ __  
 | |   \___ \| |\/| | | | |   | '__/ _ \| '_ \ 
 | |___ ___) | |  | | | | |___| | | (_) | | | |
  \____|____/|_|  |_| |_|\____|_|  \___/|_| |_|
                       terminal CRM (alpha)
```

> **CRM-Term** is a lightning-fast, keyboard-native CRM that runs entirely in your terminal. No tabs, no bloat—just the data and workflows you need, reachable in a few keystrokes on macOS, Windows, or Linux.

---

## Contents
- [Why CRM-Term?](#why-crm-term)
- [Feature Tour](#feature-tour)
- [Quick Start](#quick-start)
- [Daily Driving](#daily-driving)
- [Data & Configuration](#data--configuration)
- [Architecture Sketch](#architecture-sketch)
- [Roadmap](#roadmap)
- [Contributing](#contributing)
- [Troubleshooting](#troubleshooting)

## Why CRM-Term?
- **Snappy from the ground up** – the UI is built on [Bubble Tea](https://github.com/charmbracelet/bubbletea) and renders instantly, even on modest hardware.
- **Offline-first** – your accounts, notes, and events live in a local SQLite database. Sync hooks can be layered in later, but nothing depends on the cloud.
- **Muscle-memory navigation** – every screen is reachable via number keys *or* fuzzy typing (`da` → Dashboard, `set` → Settings, etc.). Slash commands (`/`, `exit.`) are consistent across flows.
- **Readable color palette** – themed with lipgloss so status colors stay bright and legible across macOS Terminal, iTerm2, Windows Terminal, and more.

## Feature Tour
| Area | Highlights |
| ---- | ---------- |
| **Dashboard** | Daily events in green, upcoming in yellow, recently elapsed in red. Toggle to “Activity” to see the latest accounts/notes/events you created. |
| **Accounts** | Instant search (`find>`). Optional fields stay optional—leave them blank without breaking scans. Duplicate account names are prevented. |
| **Account Creation** | Guided wizard, `/` steps back, `exit.` cancels. Captures creator + creation time automatically. |
| **Notes / Events** | Choose note or event, optionally link to an account, and the app records your name/timezone-aware timestamp automatically. Events accept `YYYY-MM-DD HH:MM` in your configured timezone. |
| **Settings & Help** | Update your display name + timezone, review shortcuts, and see where future sync settings will land. |

## Quick Start
```bash
# prerequisites
# - Go 1.19+
# - A C toolchain for CGO (Xcode CLI tools on macOS, MSYS2/MinGW or WSL on Windows)

# clone and enter (replace with your repo path)
$ git clone git@github.com:yourname/crm-term.git
$ cd crm-term

# run directly
$ go run ./cmd/crm-term

# or build a binary
$ go build -o crm-term ./cmd/crm-term
$ ./crm-term
```
> 💡 On first launch the app seeds a SQLite database and config file under your OS config directory. You can safely delete them to reset the app.

## Daily Driving
```
Main Menu
1. Dashboard          4. Create note/event
2. View accounts      5. Settings / Help
3. Add account        6. Quit
> type the number or the start of the word, then press Enter
```

### Global Commands
- `exit.` – jump back to the main menu from anywhere.
- `/` – step back within multi-stage workflows.
- `Ctrl/Cmd+C` – quit immediately.

### Keyboard Shortcuts By Screen
- **Dashboard** – type `t` then Enter to toggle Activity view; `r` + Enter to refresh.
- **Account search** – keep typing to filter; Enter accepts the search; `/` or `exit.` exits.
- **Create note/event** – blank optional answers are OK; `YYYY-MM-DD HH:MM` timestamps respect your timezone.
- **Settings** – type `1`/`2` or partial words (`nam`, `tz`) to edit name or timezone.

## Data & Configuration
| Path | Description |
| ---- | ----------- |
| `~/Library/Application Support/crmterm/` (macOS) | Default root for both config and database. |
| `%AppData%\crmterm\` (Windows) | Same, adjusted for Windows. |
| `~/.config/crmterm/` (Linux) | Same for freedesktop platforms. |
| `config.json` | Stores the display name + timezone. |
| `crmterm.db` | SQLite database with tables: `accounts`, `notes`, `events`. |

All timestamps are stored in UTC. Rendering converts to the timezone stored in `config.json`.

## Architecture Sketch
```
cmd/
└── crm-term/          # thin entry point
internal/
├── config/            # load/save user config
├── storage/           # SQLite persistence, migrations, domain helpers
├── theme/             # lipgloss styles + palette
└── ui/                # Bubble Tea model, views, navigation stack
```
- The Bubble Tea model keeps the UI state machine organized into screens (main menu, dashboard, accounts, create flows, settings).
- Storage exposes high-level helpers: `ListAccounts`, `CreateAccount`, `ListEvents`, `SplitEvents`, etc.
- A small theming package centralizes colors so you can reskin the app in one place.

## Roadmap
- 🔄 Remote sync targets (configure a sync URL in Settings).
- 🧱 Account detail pane with per-account note/event timelines.
- 🔔 Reminders for upcoming events (local notifications when possible).
- 📦 Installer tooling (Homebrew, Scoop, MSI) for easier distribution.

## Contributing
Found a bug or want to help shape the roadmap? Feel free to open an issue or submit a PR. Ideas especially welcome around:
1. Additional terminal widgets (tables, modals, etc.).
2. Multi-user sync strategies (background jobs, conflict resolution).
3. Telemetry-free analytics for usage insights.

## Troubleshooting
| Symptom | Fix |
| ------- | --- |
| `malformed LC_DYSYMTAB` warning on macOS build | Harmless CGO quirk with SQLite—binary still runs. |
| `open /dev/tty: device not configured` | Run the app in an interactive terminal; piping input hides the TTY. |
| `sql: Scan error ... converting NULL to string` | Update to the latest build—optional fields are now safely handled. |
| Windows build fails with `gcc` not found | Install a C compiler (MSYS2/MinGW) or enable WSL; CGO needs it. |

---

Happy closing deals 🚀

*“The fastest CRM is the one you actually enjoy using.”*
