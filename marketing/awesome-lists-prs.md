# PR-тексты для awesome-mac списков

## 1. jaywcjlove/awesome-mac

**Repo:** https://github.com/jaywcjlove/awesome-mac
**Раздел:** README.md → "Productivity" → "Productivity Tools" или "Utilities"

### PR title
Add Bzz — open-source keyboard layout switcher

### Markdown line

```md
- [Bzz](https://github.com/zlopixatel/bzz) - Automatic keyboard layout switcher for macOS. Open-source replacement for Punto Switcher / Caramba Switcher. Detects mistyped words ("ghbdtn" → "привет") and fixes them on the fly. Fuzzy matching catches typos. Cmd+Z undo with per-app learning. MIT. ![Open-Source Software][OSS Icon]
```

### PR description

> Adds Bzz — an open-source automatic keyboard layout switcher for macOS, written in Go.
>
> Punto Switcher (the canonical Russian keyboard switcher) was abandoned for macOS in 2017 and Caramba Switcher (the proprietary alternative) charges a yearly subscription. Bzz fills the gap as a free, open-source option.
>
> - Built with CGEventTap (active mode — can intercept Enter before submit)
> - 98K Russian dictionary + Snowball stemmer + Levenshtein fuzzy matching (catches typos)
> - Cmd+Z undo within 5s, with per-app exception learning
> - Tray icon, LaunchAgent auto-start
> - 4.5 MB binary, MIT license

---

## 2. iCHAIT/awesome-macOS

**Repo:** https://github.com/iCHAIT/awesome-macOS
**Раздел:** "Productivity" or "Utilities"

### Markdown line
```md
- [Bzz](https://github.com/zlopixatel/bzz) - Open-source automatic keyboard layout switcher for macOS. Mistyping "ghbdtn" instead of "привет"? Bzz instantly fixes it.
```

---

## 3. serhii-londar/open-source-mac-os-apps (КРИТИЧЕСКИ ВАЖНО)

**Repo:** https://github.com/serhii-londar/open-source-mac-os-apps
**Раздел:** "Useful tools" or "Utilities"
**Note:** автор русскоязычный, есть Telegram канал @opensourcemacosapps — после мерджа PR попадает в канал автоматически

### Markdown line
```md
- [Bzz](https://github.com/zlopixatel/bzz) - Automatic keyboard layout switcher (RU↔EN). Detects mistyped words and fixes them on the fly. Replaces the abandoned Punto Switcher and avoids Caramba's subscription model. Built with Go + CGEventTap. ![swift][swift_icon] [![build][build-badge-here]]()
```

(используйте формат как соседи в этом списке — посмотри на 2-3 соседних entry, чтобы попасть в их стиль)

---

## 4. abordage/awesome-mac

**Repo:** https://github.com/abordage/awesome-mac
**Note:** автообновляемый список, нужно добавить в YAML или через PR в data файл

---

## Чек-лист перед PR

- [ ] README у нас полный, с описанием и demo GIF
- [ ] Repo public
- [ ] Есть хотя бы один tagged release (v0.1.0 ✓)
- [ ] LICENSE.txt — MIT (есть)
- [ ] Description в репозитории GitHub заполнено
- [ ] Topics в репо: `macos`, `keyboard-layout`, `switcher`, `golang`, `productivity`, `punto-switcher`, `russian`

## Как делать PR (механика)

1. Form repo → клон
2. Найти соответствующий раздел, добавить строку в алфавитном порядке
3. Commit с message: `Add Bzz`
4. Push, открыть PR через `gh pr create`
5. В body PR — короткое описание (см. выше)
6. Не отвечать на code review до 24h: maintainer'ы awesome-list просматривают пачкой

## Ожидаемый таймлайн merge

- iCHAIT/awesome-macOS — 3-7 дней
- jaywcjlove/awesome-mac — 1-2 недели (огромный поток PR)
- serhii-londar — 2-4 дня (активный maintainer)
- abordage — автообновление, как только попадёт в data
