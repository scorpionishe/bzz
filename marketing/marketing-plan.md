# Bzz Marketing Plan (по фреймворкам Dunford + Lenny + YC Alstromer)

## TL;DR — три источника, одна стратегия

**Топ-3 серии лекций изучены:**
1. **April Dunford** — "Obviously Awesome" (positioning) — главный фреймворк
2. **Lenny Rachitsky** — "First 1000 users" — channel selection
3. **YC / Gustaf Alstromer + Kevin Hale** — retention, PMF, channel-product fit

**Финальное позиционирование (Dunford 5-component framework):**

> Bzz — **open-source MIT-альтернатива Caramba Switcher** для macOS: 98 000 русских слов, морфология, lifetime 490₽.

Не "ещё один переключатель раскладки", а **переопределение категории** через противопоставление Caramba.

## ICP (по Lenny — узко определённый "Дима-разработчик")

- 28-35, Mac, IT/digital, RU
- Читает HN ежедневно, Habr еженедельно
- Пишет 50/50 RU/EN
- Удалил Caramba из-за подписки/закрытого кода
- Готов заплатить 490₽ один раз за **доверенный** инструмент
- Триггерится на "open-source + не подписка"

## Channels (Reforge: 70% из ОДНОГО канала)

| # | Канал | Когда | Frame |
|---|---|---|---|
| 1 | **Habr** | Неделя 3 | Lenny: где живёт ICP |
| 2 | **Hacker News (Show HN)** | Неделя 4 | Markepear playbook |
| 3 | **Reddit r/macapps + r/programming** | Параллельно | YC Alstromer |
| 4 | **GitHub Awesome-lists (4 уже отправлены)** | Compounding | ScrollLaunch indie |
| 5 | **Press: 9to5Mac, MacRumors, vc.ru** | Усилитель после волны | Lenny strategy 6 |

## 3 hook'а для тестирования (Harry Dry: visualize/falsifiable/unique)

**A. HN (англ):**
> Show HN: Bzz – an open-source (MIT) auto layout switcher for macOS, drop-in replacement for Caramba (449 ₽/year) and the dead Punto Switcher. 98 000-word dictionary, $5 lifetime.

**B. Habr (русск):**
> Caramba просит 449₽/год, Punto Switcher умер в 2017. Я написал свой open-source автопереключатель: MIT, 98K слов, 490₽ один раз. Разбираю CGEventTap, Snowball-стемминг и почему 70% оценок Caramba в App Store — 1 звезда.

**C. Reddit r/macapps (англ):**
> I open-sourced my replacement for Caramba Switcher. MIT license, $5 lifetime. 98K-word morphological dictionary, fuzzy typo matching. Code on GitHub if you want to audit it.

## Метрики (YC Alstromer)

| Метрика | Цель |
|---|---|
| W1 retention | >50% |
| **W4 retention** (PMF threshold) | **>30%** |
| Activation rate | >70% |
| GitHub stars/install | ~10% |
| Free → Pro | 3-5% |
| Channel concentration | >70% from one |
| Refund rate | <5% |

## План по неделям

### Неделя 1 — Foundations
- Лендинг переписать по Harry Dry (visualize/falsifiable/unique)
- Дунфорд-карточка вверху README
- Сравнительная таблица **Bzz vs Caramba vs Punto**
- Закрыть P0: автозапуск, .app bundle, **notarization**
- Без notarization W1 retention < 30% — без смысла лить трафик

### Неделя 2 — Soft launch
- Manual onboarding 20 знакомых разработчиков (Алстромер: "talk to users")
- Получить 5 testimonial цитат
- Запостить в 2-3 Mac-Telegram каналах **спросить feedback**, не продавать

### Неделя 3 — Habr launch (русский HN)
- Технический deep-dive: CGEventTap + Snowball + dictionary cleanup
- В первые 5 минут — авторский комментарий-расширение
- Параллельно r/macapps + r/programming

### Неделя 4 — Show HN
- Только когда Habr-волна стабилизировалась и баги исправлены
- В первые 5 минут — длинный авторский коммент по Markepear шаблону
- Доступным быть на тред 24-48ч, отвечать ВСЕМ
- НЕ просить друзей апвоутить (бан)

### Месяц 2 — Compounding
- 6-8 awesome-lists PR (расширить с 4)
- 1 пост/неделя на Habr (deep-dive по технике)
- Email/Telegram changelog еженедельно
- Pitch 9to5Mac, MacRumors, ixbt, vc.ru

### Квартал 1 — целевые метрики
- 3-5K GitHub stars
- 5-10K installs
- 200-500 paid (3-5%)
- 100-250K₽ revenue
- W4 > 30% → можно scale через ads
- W4 < 20% → стоп, фиксить продукт

## ЧТО ИЗБЕГАТЬ

1. ❌ "Ещё одна тулза"-позиционирование — нарушает Дунфорд
2. ❌ Дробить силы на 5 каналов одновременно — Reforge принцип нарушен
3. ❌ Запускать на HN до того как починены P0 баги — один шанс
4. ❌ Trial вместо free forever — ломает open-core positioning
5. ❌ Слова "best", "fastest", "innovative" — Harry Dry: failes falsifiability
6. ❌ Покупать ads до 1000 органических юзеров — данных для CAC мало
7. ❌ HN и Habr одновременно — раздробишь внимание автора
8. ❌ Платить инфлюенсерам в Phase 1 — для dev-tools не работает
9. ❌ Pre-launch waitlist — для утилитарных продуктов не работает
10. ❌ Строить community до запуска — нет аудитории

## Источники

- [April Dunford — Obviously Awesome](https://www.aprildunford.com/post/a-quickstart-guide-to-positioning)
- [Lenny — First 1000 users](https://www.lennysnewsletter.com/p/how-the-biggest-consumer-apps-got)
- [YC Gustaf Alstromer — Growth for Startups](https://www.ycombinator.com/library/6k-growth-for-startups)
- [Markepear — Dev tool HN launch playbook](https://www.markepear.dev/blog/dev-tool-hacker-news-launch)
- [Reforge — Four Fits Framework](https://www.reforge.com/blog/four-fits-growth-framework)
- [Harry Dry — Marketing Examples copywriting](https://growthexperts.co/copywriting-tips-from-harry-dry-three-rules-to-help-you-level-up/)
- [ScrollLaunch — Indie playbook](https://www.scrolllaunch.com/blog/indie-hacker-growth-first-1000-users)
