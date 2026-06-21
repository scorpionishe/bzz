# Bzz

[![Go Version](https://img.shields.io/badge/go-1.26+-blue.svg)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Platform: macOS](https://img.shields.io/badge/platform-macOS-lightgrey.svg)](https://www.apple.com/macos/)
[![Build Status](https://img.shields.io/badge/build-passing-brightgreen.svg)](#building-from-source)

Automatic keyboard layout switcher for macOS. Types "ghbdtn" in QWERTY by mistake? Bzz instantly converts it to "привет" before you hit Enter.

![Bzz demo](docs/demo.gif)

> **Bzz is the open-source MIT alternative to Caramba Switcher** for macOS users who need automatic Cyrillic↔Latin layout switching — a 98K-word morphological dictionary, on-device only (no telemetry), and a one-time 490 ₽ price for Pro features instead of a yearly subscription. It also fills the gap left by Punto Switcher, which has been abandoned for macOS since 2017.

## Changes in this fork

This fork ([scorpionishe/bzz](https://github.com/scorpionishe/bzz)) makes Bzz **layout-neutral** and hardens the manual hotkey:

- **Never switches your system input source.** Upstream cycled the active macOS layout to the *next* source after every correction. That "wandered" across mixed-language sentences and broke whenever you have more than two selectable sources (e.g. ABC + Russian + the Character Viewer), landing on the wrong layout and garbling every other word. This fork only rewrites the *text* in place — for both auto-correction and the `Cmd+Shift+X` manual hotkey — and leaves the active layout alone (classic Punto-style behavior). You keep typing in one layout; Bzz just fixes the words.
- **Hardened `Cmd+Shift+X`.** It releases stuck modifiers before and after the conversion, so a *synthetic* hotkey (e.g. one remapped from Caps Lock via Karabiner) can no longer leak `Shift` into the internal `Cmd+C` (the "no selection detected" failure) or leave `Cmd` logically held, which used to turn your next Space into `Cmd+Space` (Spotlight). It also clears the auto-correction buffer when triggered, so the following space can't re-fire on the stale keystrokes and double-convert (`привет` → `привета`).

See commits `fcd0851` and `da0f2d1`. The build is ad-hoc signed (not notarized) — see [Installation](#installation) for the Gatekeeper/quarantine step.

## Features

- **Instant auto-correction**: Detects wrong keyboard layout and fixes on the fly
- **Smart dictionary**: 98K Russian words with Snowball stemmer for accurate detection
- **Fuzzy matching**: Catches typos within 1 character edit distance
- **Undo in 5 seconds**: Cmd+Z to revert the last correction
- **Context-aware**: Understands Russian/English context to avoid false positives
- **Zero configuration**: Works out of the box with sensible defaults
- **System tray**: Pause/resume with a single click (⚡ active, 💤 paused)
- **Auto-start**: Launches automatically at login via LaunchAgent
- **Lightweight**: ~4 MB binary, minimal CPU usage

## Requirements

- **macOS 10.15+**
- **Accessibility permission** (required for keyboard interception)

Windows support is planned.

## Installation

### Download Pre-built Binary (Recommended)

1. Download the latest `.dmg` from [Releases](https://github.com/zlopixatel/bzz/releases)
2. Open the DMG and drag **Bzz** to Applications
3. Launch Bzz from Applications
4. **Important**: First launch requires granting Accessibility permission:
   - macOS will prompt you to allow Bzz in **System Settings → Privacy & Security → Accessibility**
   - If prompted by Gatekeeper ("Cannot open because it is from an unidentified developer"):
     - Right-click Bzz.app → **Open Anyway**
     - Or: `xattr -d com.apple.quarantine /Applications/Bzz.app`

### Build from Source

**Requirements**: Go 1.26+ and `make`

```bash
git clone https://github.com/zlopixatel/bzz.git
cd bzz
make app          # Creates Bzz.app in ./build/
make install      # Copies to ~/Applications/
```

Or build the binary only:

```bash
go build -ldflags="-s -w" -o Bzz .
./Bzz
```

## First Run

1. **Grant Accessibility permission**:
   - Go to **System Settings → Privacy & Security → Accessibility**
   - Add Bzz to the allowed apps list
   - Restart Bzz if needed

2. **Configure (optional)**:
   - Bzz creates `~/Library/Application Support/Bzz/config.yaml` on first run
   - Default settings work for most users — no action required

3. **Check tray icon**:
   - Look for the Bzz icon in the menu bar (top-right corner)
   - ⚡ icon = active, 💤 icon = paused

## Usage

### Automatic Correction

Just type normally. Bzz watches for wrong keyboard layout:

```
Type: ghbdtn [Space]     → Auto-corrects to: привет
Type: GHBDTN [Space]     → Auto-corrects to: ПРИВЕТ
Type: Lfvecz, [Enter]    → Auto-corrects to: Привет, [then submits]
```

**When does it correct?**
- When you press Space, Enter, or punctuation (. , ! ? ; : ' ")
- If the word is in the Russian dictionary
- Or within 1 typo of a known Russian word (6+ characters)

### Undo

Press **Cmd+Z** within **5 seconds** of a correction to revert:

```
ghbdtn [Space] → привет
[Cmd+Z]        → ghbdtn (reverted)
```

The undo window closes after 5 seconds or if you type something else.

### Pause/Resume

Click the tray icon to toggle:
- **⚡ Active** (normal operation)
- **💤 Paused** (Bzz is disabled)

Or quit from the tray menu.

## Configuration

Edit `~/Library/Application Support/Bzz/config.yaml`:

```yaml
enabled: true                    # Enable/disable the app
primary_language: ru             # Primary language (ru or en)
min_word_length: 2               # Minimum word length to check
excluded_apps:                   # Apps where Bzz is disabled
  - idea                         # Example: JetBrains IDEs
```

Changes take effect immediately — no restart needed.

## Architecture

### Core Components

- **Hook** (`hook_darwin.go`): CGEventTap intercepts keyboard events in real-time
- **Detector** (`detector.go`): Analyzes text to determine if layout switch occurred
- **Dictionary** (`dict.go`): 98K Russian words + Snowball stemmer for lemmatization
- **Replacer** (`replacer_darwin.go`): Sends backspace/character events to fix text
- **Tray** (`tray_darwin.go`): System menu bar integration for pause/resume
- **Buffer** (`buffer.go`): Accumulates characters to form words at phrase boundaries

### Detection Algorithm

1. **Script detection**: Identifies if text is Latin or Cyrillic
2. **Dictionary lookup**: Checks if the converted word exists in Russian dictionary
3. **Stem matching**: Uses Snowball stemmer for verb/noun variations
4. **Fuzzy matching**: For 6+ character words, finds corrections within 1 edit distance (Levenshtein)
5. **Context tracking**: Remembers recent language to avoid false positives
6. **Trailing punctuation**: Handles mixed QWERTY/Russian punctuation correctly

### Example Detection

```
Input:      "ghbdtn"
Convert:    "привет" (QWERTY → Russian keymap)
Lookup:     ✓ Found in dictionary
Result:     Correct to "привет"

Input:      "ghbdtna" (typo: extra 'a')
Convert:    "приветf" 
Fuzzy:      Within 1 edit of "приветф" or "привет"?
Result:     Correct to "привет"
```

## Building from Source

### macOS Binary

```bash
go build -ldflags="-s -w" -o Bzz .
```

### macOS App Bundle

```bash
make app      # Creates Bzz.app with icon
```

### macOS Installer (DMG)

```bash
make dmg      # Creates Bzz.dmg
```

### Windows Executable (Cross-compile from macOS)

Windows support requires MinGW on macOS:

```bash
brew install mingw-w64
make build-windows
```

Creates `Bzz.exe`.

### Release Artifacts

```bash
make release  # Builds both macOS .dmg and Windows .exe
```

## Testing

Run the test suite:

```bash
go test ./... -v
```

Key tests:
- `detect_test.go`: Dictionary lookup and fuzzy matching logic
- `shifted_test.go`: Shifted key handling (Caps Lock, numbers)
- `integration_test.go`: End-to-end behavior with buffer

## Contributing

Bzz is open-source under the MIT license. Contributions welcome!

### Adding Support for Other Languages

1. Add a dictionary file to `dicts/<lang>_freq.txt` (one word per line)
2. Add stemmer support in `dict.go` if available
3. Update keymap in `keymap.go` for the target language
4. Test with `detect_test.go`

### Improving Russian Dictionary

The dictionary at `dicts/ru_freq.txt` is frequency-ranked. To improve:

- Submit PRs with additional common Russian words
- Include frequency data if available
- Test with real-world usage patterns

## Architecture Decisions

- **CGEventTap**: Chosen over Quartz Event Services for consistent, low-latency event interception
- **Snowball Stemmer**: Preferred over hand-written rules for Russian morphology
- **Active undo (5s window)**: Short enough to not interfere with typing, long enough for reflexive corrections
- **Exact EN match**: Prevents "if", "the", "and" from being corrected when typed on Russian layout
- **Lightweight dictionary**: Embedded (no network calls), fast in-memory lookup with 98K words covering 99% of common Russian

## Open Core Model

Bzz core is free under the MIT License — forever. This includes auto-correction,
fuzzy matching, Cmd+Z undo with per-app learning, Cmd+Shift+X manual selection
conversion, tray icon, LaunchAgent auto-start, and per-app exclusions via
config.yaml. None of these will ever move to a paid tier.

**Bzz Pro (v0.4, planned Q3 2026)** — one-time 490 ₽ purchase, lifetime:

1. **Custom Dictionary** — add your own terms, client names, professional jargon
   the auto-corrector should respect
2. **Additional layouts** — Ukrainian, Kazakh, Belarusian, German, French
   (one included in Pro, others +200 ₽ upgrade)
3. **Per-app exception UI** — graphical management of the rules Cmd+Z learns

See [marketing/PRO_FEATURES.md](marketing/PRO_FEATURES.md) for the canonical
list and rationale. Pro will be sold through a Telegram bot with offline
Ed25519-signed license files — no servers, no telemetry, no recurring charges.

Until Pro ships, you can support development via Boosty / GitHub Sponsors
(TBA) — supporters do not get extra features (so we stay honest about
what Pro will be).

## Privacy

Bzz processes keystrokes locally only. There is no HTTP client in the
binary — `grep -rE "net/http|net.Dial" *.go` returns nothing. Read the
full [Privacy Policy](https://zlopixatel.github.io/bzz/privacy.html).

## Troubleshooting

### "Bzz cannot be opened" (Gatekeeper)

```bash
xattr -d com.apple.quarantine /Applications/Bzz.app
open /Applications/Bzz.app
```

Or: Right-click Bzz.app → Open Anyway

### Accessibility permission not working

1. Go to **System Settings → Privacy & Security → Accessibility**
2. Remove Bzz if listed
3. Restart Bzz — it will re-request permission
4. Grant access and restart

### Corrections not happening

- Check the tray icon (is it ⚡ or 💤)?
- Verify Accessibility permission is granted
- Check system logs: `log stream | grep Bzz`
- Ensure your keyboard layout is set to Russian (Cmd+Space to switch)

### Performance or crashes

- Check `~/Library/Application Support/Bzz/config.yaml` for typos
- Try resetting config: Delete the file and restart (defaults will be recreated)
- Report issues with system details at [GitHub Issues](https://github.com/zlopixatel/bzz/issues)

## License

MIT License — see [LICENSE](LICENSE) for details.

Copyright © 2026 Roman Kovalev

## Competitors

- **Caramba Switcher**: Closed-source, subscription model ($29.99/year)
- **Punto Switcher**: Legacy Windows-first design, limited macOS support
- **Bzz**: Free, open-source, macOS-native

---

## Русский / Russian

### О проекте

**Bzz** — автоматический переключатель раскладки клавиатуры для macOS. Типите "ghbdtn" вместо "привет"? Bzz исправит это прямо при вводе.

### Отличия этого форка

Этот форк ([scorpionishe/bzz](https://github.com/scorpionishe/bzz)) делает Bzz **нейтральным к раскладке** и укрепляет ручной хоткей:

- **Не переключает системную раскладку.** В апстриме после каждой коррекции активный язык ввода щёлкался «на следующий», из-за чего раскладка «гуляла» по смешанной фразе и ломалась при >2 источниках (например ABC + Russian + Character Viewer) — попадала не туда и портила каждое второе слово. Здесь Bzz правит только *текст* на месте (и в авто-коррекции, и в ручном `Cmd+Shift+X`), а раскладку не трогает — как классический Punto. Печатаешь в одной раскладке, Bzz просто чинит слова.
- **Укреплён `Cmd+Shift+X`.** Сбрасывает залипшие модификаторы до и после конвертации: *синтетический* хоткей (например переназначенный с Caps Lock через Karabiner) больше не «протекает» `Shift`'ом во внутренний `Cmd+C` (ошибка «no selection detected») и не оставляет зажатым `Cmd` (из-за чего следующий пробел превращался в `Cmd+Space`/Spotlight). Плюс очищает буфер авто-коррекции при срабатывании, чтобы пробел после не сработал по устаревшим буквам и не давал двойную конвертацию (`привет` → `привета`).

Коммиты `fcd0851` и `da0f2d1`. Сборка подписана ad-hoc (без нотаризации) — шаг с Gatekeeper/карантином см. в установке.

### Возможности

- **Автоматическое исправление**: Обнаруживает ошибку раскладки и исправляет на лету
- **Словарь из 98K слов**: С лемматизацией Snowball для точного обнаружения
- **Нечёткий поиск**: Находит опечатки в расстоянии Левенштейна до 1
- **Отмена за 5 секунд**: Cmd+Z для отката последней коррекции
- **Контекстная обработка**: Понимает русский/английский контекст
- **Без конфигурации**: Работает сразу из коробки
- **Иконка в трее**: Пауза/возобновление одним кликом
- **Автозапуск**: Запускается при входе в систему

### Установка

#### Готовый бинарник

1. Скачайте последний `.dmg` с [Releases](https://github.com/zlopixatel/bzz/releases)
2. Откройте DMG и перетащите **Bzz** в Applications
3. Запустите Bzz
4. Системе потребуется разрешение доступа в **System Settings → Privacy & Security → Accessibility**

#### Сборка из исходников

```bash
git clone https://github.com/zlopixatel/bzz.git
cd bzz
make app      # Создаёт Bzz.app
make install  # Копирует в ~/Applications/
```

### Использование

#### Автоматическое исправление

Просто пишите. Bzz следит за ошибками раскладки:

```
Печатаете: ghbdtn [Space]     → Исправляет на: привет
Печатаете: Lfvecz, [Enter]    → Исправляет на: Привет, [отправляет]
```

Исправляет когда:
- Нажимаете Space, Enter или пунктуацию (. , ! ? ; : ' ")
- Слово есть в русском словаре
- Или в одной опечатке от русского слова (6+ букв)

#### Отмена

**Cmd+Z** в течение **5 секунд** после исправления:

```
ghbdtn [Space] → привет
[Cmd+Z]        → ghbdtn (отменено)
```

#### Пауза/возобновление

Клик на иконку в трее:
- **⚡ Активно** (работает)
- **💤 Пауза** (отключено)

### Конфигурация

Отредактируйте `~/Library/Application Support/Bzz/config.yaml`:

```yaml
enabled: true              # Включено/отключено
primary_language: ru       # Основной язык (ru или en)
min_word_length: 2         # Минимальная длина слова
excluded_apps:             # Приложения, где отключено
  - idea                   # Пример: JetBrains IDEs
```

### Проблемы и решения

**"Bzz cannot be opened":**

```bash
xattr -d com.apple.quarantine /Applications/Bzz.app
```

**Нет разрешения на доступ:**

1. **System Settings → Privacy & Security → Accessibility**
2. Удалите Bzz из списка
3. Перезапустите Bzz и снова разрешите доступ

**Исправления не работают:**

- Проверьте иконку в трее (⚡ или 💤?)
- Убедитесь, что раскладка установлена на Русский
- Посмотрите логи: `log stream | grep Bzz`

### Лицензия

MIT — см. [LICENSE](LICENSE)

© 2026 Roman Kovalev
