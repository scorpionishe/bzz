# Show HN draft

## Когда постить
- Вторник или четверг (среда тоже работает)
- 8:00–10:00 PT (19:00–21:00 МСК / 18:00–20:00 UTC)
- НЕ: понедельник до 7:00 PT, пятница после 14:00 PT, выходные

## Title (примеры, выбрать один)

**A. Прямой:**
> Show HN: Bzz – Open-source keyboard layout switcher for macOS

**B. С контекстом проблемы (предпочтительнее):**
> Show HN: Bzz – Auto-fix mistyped layouts on macOS (open-source Punto Switcher replacement)

**C. С техническим хуком:**
> Show HN: Bzz – Active CGEventTap intercepts Enter to fix wrong-layout words before submit

Рекомендую **B** — конкретика, понятная не-русскоговорящим. "Punto Switcher replacement" гуглится — старые экспаты узнают.

## URL
https://github.com/zlopixatel/bzz

## Body (первый комментарий, обязательно от автора)

```
Author here.

Russian speakers on macOS know the pain: you forget to switch layouts and type "ghbdtn" when you meant "привет". Punto Switcher (Yandex's tool that auto-fixed this on Windows) was abandoned for Mac in 2017 — the installer doesn't even run on Apple Silicon. The closed-source alternative (Caramba Switcher) charges a yearly subscription. So I built Bzz.

It's a Go binary (4.5 MB) using CGEventTap in active mode. The interesting part is intercepting Enter before the host app submits — without that, in Slack/Telegram/Notion the message would fly out unedited. The tap suppresses Enter, runs the dictionary check synchronously, replaces the word if needed, then re-injects Enter.

Other bits I'm proud of:

- Fuzzy matching with Levenshtein edit distance 1 — if you missed a key ("gjljk;bv" instead of "ghjljk;bv"), it still finds "продолжим" in the 98K-word dictionary
- Cmd+Z undo within 5 seconds, and Cmd+Z teaches the app to never touch that word in that specific app again — self-tuning per-app exclusion list
- Frontmost-app detection via NSWorkspace notifications instead of polling NSWorkspace from the event tap thread (which Apple doesn't guarantee thread-safe)
- 215 tests, runs clean with -race

Tech stack: Go for the core, Objective-C bridges via CGo for CGEventTap and the tray icon. NSApp event loop locked to OS main thread (had a bunch of fun crashes before figuring out runtime.LockOSThread() was needed).

Code: https://github.com/zlopixatel/bzz
Demo GIF in the README.

MIT licensed. Pre-built DMG in releases. Happy to answer questions about the event tap architecture or the dictionary tuning.
```

## Подготовка перед постом

- [ ] Repo public ✓
- [ ] README актуальный, с demo GIF ✓
- [ ] Latest release собран ✓
- [ ] Topics на GitHub ✓
- [ ] Description ✓
- [ ] Issues открыты для багрепортов

## После поста

**Первые 2 часа критичны** — ранжирование зависит от:
1. Скорости первых апвоутов (быть готовым шерить ссылку в личку 5-10 знакомых dev-друзей с просьбой прочитать и upvote если понравится — НЕ просить просто upvote, это банится)
2. Качество комментариев (на каждый комментарий отвечать в течение 30 минут первые 2 часа)
3. Engagement в треде

**Типичные вопросы которые зададут (готовы ответы):**

Q: Why not Karabiner-Elements?
A: Karabiner is a key remapper, not a layout switcher. Bzz detects mistyped *words* (after the fact) and fixes them. Different problem.

Q: Why not just use the system input source switcher (Caps Lock)?
A: That's manual. The whole point is detecting that you forgot to switch.

Q: Open source security concerns about keyboard hooks?
A: That's the whole reason it's MIT — you can read the code, build it yourself, audit it. Closed-source keyboard hooks are a leap of faith. The Punto Switcher 2018 incident (Habr article) where the Windows version was caught sending typing data to Yandex servers is part of why this exists.

Q: Performance?
A: Synchronous detection takes ~2µs per word for exact matches, ~5ms worst case for fuzzy. CGEventTap callback runs on its own dispatch queue.

Q: Does this work for [other languages]?
A: Currently RU↔EN. Architecture supports any layout pair — needs a dictionary file and a QWERTY mapping. UA, KZ, BY are on the roadmap. PRs welcome.

Q: Why Go and not Swift?
A: Cross-platform — same code compiles for Windows. Also, I just like Go.

## Дополнительные форумы для повтора

После Show HN:
- Lobsters (если есть инвайт): меньше hype, лучше signal
- r/golang — фокус на технической реализации
- r/macapps — focus на user-facing
- r/macprogramming — техническая часть про CGEventTap
