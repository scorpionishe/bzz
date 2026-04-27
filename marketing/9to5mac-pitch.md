# 9to5Mac Indie App Spotlight Pitch

**Send to:** michaelb@9to5mac.com
**Subject:** Indie App Spotlight pitch — Bzz (open-source keyboard layout switcher)

---

Hi Michael,

Pitching Bzz for the Indie App Spotlight series.

**The hook:** Every Russian Mac user knows the pain of typing "ghbdtn" instead of "привет" because they forgot to switch layouts. The canonical fix — Punto Switcher — was abandoned by Yandex in 2017 and the only working alternative (Caramba Switcher) charges a yearly subscription. I built Bzz as an open-source replacement: type a wrong-layout word, it gets fixed on the fly, before you hit Enter.

**One-liner:** Open-source automatic keyboard layout switcher for macOS. MIT licensed.

**What's interesting from a Mac dev perspective:**

- Active CGEventTap (not listen-only) so it can intercept Enter *before* the host app submits — works in Slack, Telegram, Notion where the message would otherwise fly out unedited
- Fuzzy matching with Levenshtein edit distance 1 — if you mistyped "gjljk;bv" instead of "ghjljk;bv" (missed a key), it still finds "продолжим" in the 98K dictionary
- Cmd+Z undo within a 5-second window, and crucially: a Cmd+Z explicitly teaches the app that this word should never be auto-corrected in this specific app. Self-tuning per-app exclusion list
- 4.5 MB Go binary, no telemetry, no auto-update tracking

**Why now:**
- Punto Switcher for Mac is dead since 2017
- Caramba is a 449₽/year subscription with a closed source binary that has full keyboard access — there's renewed concern in the community about that combo

**Links:**
- Site: https://bzz.app (TBA — currently github.io)
- GitHub: https://github.com/zlopixatel/bzz
- Demo GIF: https://github.com/zlopixatel/bzz/blob/main/docs/demo.gif
- Latest release: https://github.com/zlopixatel/bzz/releases/latest

Happy to ship a higher-quality video, screenshots, or answer questions. Already have notarization in progress (so the user-facing setup is just a drag-and-drop).

Thanks,
Roman
zlopixatel12@gmail.com

---

## Notes for follow-up
- If Michael is interested but wants more polish — point him at notarization (when done) and add a custom domain bzz.app (currently github.io)
- Have screenshots ready: tray icon ⚡/💤, demo of fuzzy matching, FAQ about open source security
- If feature happens — schedule social posts (Twitter, Mastodon, RU Telegram) to coincide
