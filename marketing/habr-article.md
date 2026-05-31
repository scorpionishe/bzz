# Хабр статья: Bzz — open-source замена Punto Switcher для Mac

**Когда публиковать:** будний день, утро (10:00-12:00 МСК)
**Хаб:** Open source, macOS, Go, Программирование, Тестирование IT-систем
**Теги:** punto-switcher, caramba-switcher, macos, golang, cgeventtap, open-source, переключатель раскладки, accessibility-api

**Title (выбрать один):**

A. **Как я написал замену Punto Switcher для Mac на Go (и что вылезло из CGEventTap)**
B. **Punto Switcher для Mac мёртв с 2017. Я написал ему open source замену**
C. **Активный CGEventTap, Levenshtein и race conditions в трёх потоках: история одной утилиты**

Рекомендую **A** — связывает узнаваемое имя с техническим хуком.

---

# Как я написал замену Punto Switcher для Mac на Go (и что вылезло из CGEventTap)

![Bzz demo](https://github.com/zlopixatel/bzz/raw/main/docs/demo.gif)

Punto Switcher на Mac не выпускается с 2017 года. Caramba Switcher — закрытый код и подписка 449 ₽/год. Я хотел что-то простое, бесплатное и с открытыми исходниками — и за пару выходных сделал свою утилиту. Но три "выходных" растянулись в две недели отладки, потому что macOS event tap живёт не так, как написано в документации.

В статье — про активный CGEventTap, перехват Enter в Slack/Telegram до отправки сообщения, fuzzy matching через Levenshtein, и три race condition'а, которые проявлялись только при быстром вводе.

Код целиком на GitHub под MIT: [github.com/zlopixatel/bzz](https://github.com/zlopixatel/bzz).

## Зачем вообще такая утилита

Все, кто пишет на двух языках попеременно, через раз ловят себя на наборе на неправильной раскладке: вместо "привет" получается "ghbdtn", вместо "спасибо" — "cgfcb,j". На Windows эту проблему с 2001 года решает Punto Switcher: смотрит набранное слово, видит, что оно — мусор на текущей раскладке, но осмысленное на другой, и автоматически переключает.

На Mac:

- **Punto Switcher для Mac** — последний релиз сентябрь 2017. На Apple Silicon не запускается, на macOS Sonoma/Sequoia хук клавиатуры мёртв. Yandex проект формально не закрыл, но и не поддерживает.
- **Caramba Switcher** — единственный живой коммерческий вариант. Закрытый код. Подписка 449 ₽ в год. У утилиты-переключателя нет облачной инфраструктуры, нет серверов — подписка тут чисто как способ извлечь больше денег. И в App Store у Caramba 4.2★, но из 2 033 оценок 1 408 — единица. Главные жалобы: подписка, нет настроек, грубые ответы разработчика на вопросы.
- **InputSourcePro / Mahou / Kawa** — переключают input source по приложению, но не правят уже набранный текст.

Окно для open source решения было пустое. Я его и занял.

## Что в итоге получилось

```
Type: ghbdtn [Space]   → "привет "
Type: rfr ltkf? [End]  → "как дела?" (Enter перехвачен и пересылается после замены)
Type: gjljk;bv [Space] → "продолжим " (fuzzy: пропущена клавиша "h")
Cmd+Z (в течение 5 сек) → откат + запоминание исключения для этого приложения
```

Stack:

- Ядро на Go, бинарник 4.5 MB
- CGEventTap через CGo (Objective-C bridges) для перехвата клавиатуры
- 98K русских слов + Snowball stemmer + suffix stripping для морфологии
- Levenshtein edit distance 1 для опечаток
- Тестов 227 cases в 23 функциях, проходят с `-race`
- LaunchAgent для автозапуска, NSStatusBar для иконки в menubar

Дальше — про куски, которые показались интересными.

## Хук клавиатуры на macOS: listen-only vs active

Первая итерация была на `kCGEventTapOptionListenOnly`:

```objc
CFMachPortRef tap = CGEventTapCreate(
    kCGSessionEventTap,
    kCGHeadInsertEventTap,
    kCGEventTapOptionListenOnly,   // ← только наблюдение
    (1 << kCGEventKeyDown),
    eventCallback,
    NULL
);
```

Listen-only tap получает события для анализа, но не может их модифицировать или подавлять. События как пришли, так и улетят в приложение. Для самой замены этого достаточно — мы накапливаем символы в буфере, при пробеле смотрим слово, шлём backspace + новый текст через `CGEventPost`.

Проблема обнаружилась, когда я начал тестировать в Slack и Telegram: пользователь набирает "ghbdtn?" и нажимает Enter, чтобы отправить вопрос. До того, как goroutine с заменой успевает запуститься, Enter уже улетел в приложение, и сообщение ушло как "ghbdtn?". Замена потом доходит, но в уже **отправленное** сообщение её не вставишь.

Чтобы исправить, перешёл на `kCGEventTapOptionDefault` (active tap):

```objc
int suppress = goKeyCallback(keycode, ch, flags);
if (suppress) return NULL;   // ← возвращаем NULL → событие подавлено
return event;                 // ← обычный pass-through
```

Когда мы видим Enter и в буфере есть слово, требующее замены: подавляем Enter (возвращаем `NULL`), синхронно запускаем замену, потом инжектим Enter заново через `CGEventPost`. Слово исправлено до отправки.

Тонкость: `CGEventCreateKeyboardEvent` для Enter без подавления уже отправленного события вызывает повторение. Нужно именно `return NULL` в callback'е, а не "поглотить и переотправить".

## NSWorkspace и поток, на котором он не работает

Чтобы знать, в каком приложении сейчас юзер, я хотел получать bundle ID frontmost приложения. Решение в лоб:

```objc
NSRunningApplication *app = [[NSWorkspace sharedWorkspace] frontmostApplication];
NSString *bid = app.bundleIdentifier;
```

Вызывал прямо из event tap callback. Работало. Месяц. Потом начали сыпаться спорадические краши на пользовательских машинах с разными macOS-версиями.

Apple явно говорит: NSWorkspace API должны вызываться из main thread. Event tap callback живёт в отдельном CFRunLoop потоке. На практике вызов из background часто проходит, но не гарантированно. Поведение зависит от состояния системы, количества запущенных приложений, и фазы луны.

Решение — реактивное. Регистрируемся на `NSWorkspaceDidActivateApplicationNotification` (ловится строго на main thread), кэшируем bundle ID в C-атомике, читатели из любого потока берут готовое значение:

```c
static _Atomic(const char*) cachedBundleID = NULL;

static void onAppActivated(NSNotification* note) {
    NSRunningApplication* app = note.userInfo[NSWorkspaceApplicationKey];
    const char* newStr = app ? strdup([app.bundleIdentifier UTF8String]) : NULL;
    const char* old = atomic_exchange(&cachedBundleID, newStr);
    if (old) free((void*)old);
}

// thread-safe read из любого потока
static const char* frontmostBundleID(void) {
    return atomic_load(&cachedBundleID);
}
```

После этого FrontmostAppID() стал O(1) и безопасным. Краши прекратились.

## Race conditions, которые ловились только под нагрузкой

В Go-коде у меня было три гонки. Все мирно лежали и ждали быстрого ввода или одновременных событий, чтобы выстрелить.

### 1. RWMutex с записью под RLock

```go
func (s *ExceptionStore) IsException(app, word string) bool {
    s.mu.RLock()
    defer s.mu.RUnlock()
    if e, ok := s.index[makeKey(app, word)]; ok {
        e.HitCount++   // ← запись под RLock!
        return true
    }
    return false
}
```

`RLock` разрешает несколько одновременных читателей. Если два потока одновременно вызовут `IsException("Slack", "the")`, оба войдут в `if`, и оба сделают `HitCount++` — классическая гонка с потерей инкрементов. Хуже того, race detector это видит как UB.

Фикс — заменить `RLock`/`RUnlock` на `Lock`/`Unlock`. Производительность не страдает, потому что инкрементируется редкий happy path.

### 2. Goroutine-ливень из event tap

Изначальная версия буфера эмиттила слово через `go b.onWord(word)`:

```go
if isWordBoundary(r) {
    word := emitAndClear()
    if b.onWord != nil {
        go b.onWord(word)   // ← новая горутина на каждое слово
    }
}
```

Снаружи это казалось разумным: callback может быть медленным (детектор + replace через CGEventPost занимает 50-100 ms), и блокировать event tap callback нельзя — придёт следующее событие, мы его не обработаем.

Но детектор — не stateless. У него `lastLangRu`, `initialized`, `trailingPunct`, и они мутируются при каждом `Check()`. Параллельные горутины эти поля затирают друг другу.

Плюс был более тонкий dead-lock: `b.onWord(word)` вызывается под `b.mu.Lock()`. Внутри callback зовётся `replaceText`, который зовёт `buf.Clear()`, который пытается взять тот же самый mutex.

Фикс — перейти на синхронный вызов после освобождения mutex:

```go
func (b *Buffer) Add(r rune) {
    b.mu.Lock()
    var emit string
    if isWordBoundary(r) {
        emit = b.flush()
    } else {
        b.chars = append(b.chars, r)
    }
    b.mu.Unlock()

    if emit != "" && b.onWord != nil {
        b.onWord(emit)   // синхронно, без mutex
    }
}
```

CGEventTap callback при этом не блокируется надолго — детектор укладывается в ~5 µs на слово, а сам replace запускается через goroutine уже **внутри** колбэка с `replacing` атомиком, чтобы не ловить свои же события обратно.

### 3. Зомби-процесс из go test

Это не race, а weirdness, но потеряло мне час дебага. Замены начали дублироваться — каждый символ вставлялся два раза. Откатывал изменения, гонял `git bisect` — баг не находился ни на каком коммите. В итоге `ps aux | grep` показал:

```
romankovalev  37352  /Users/romankovalev/Applications/Bzz
romankovalev  52196  /var/folders/.../go-build.../bzz
```

Процесс из `/var/folders/.../go-build/...` — тестовый бинарник от прошлого `go test`, который не был убит и держал свой собственный event tap. Каждый keystroke ловили оба, замены тоже шли двумя. Killed PID 52196 — баг исчез.

Мораль: на macOS любой нативный keyboard-hook процесс должен убиваться внятно при выходе из тестов. Поставил `t.Cleanup` в setup-функциях.

## Fuzzy matching: Levenshtein без BK-tree

Базовая логика тривиальна:

```go
func (d *Detector) Check(text string) (wrong bool, corrected string) {
    converted := QWERTYToRussian(text)
    if d.ruDict.Has(converted) {
        return true, converted
    }
    return false, ""
}
```

Но юзеры ошибаются. Печатают "gjljk;bv" имея в виду "продолжим" (правильно было бы "ghjljk;bv" — пропущена "h"). Конвертация даёт "подолжим" — такого слова нет.

Опечатка может быть deletion (потеряли клавишу), insertion (лишний символ), substitution (попали в соседнюю). На каждое слово в словаре прогонять Levenshtein слишком медленно — словарь 98K. Я взял другой подход: вместо поиска ближайшего слова из словаря, генерирую все слова на расстоянии 1 от моего, и ищу их через хеш-лукап:

```go
func (d *Dict) FuzzyFind(word string) (string, bool) {
    runes := []rune(word)
    alphabet := []rune("абвгдеёжзийклмнопрстуфхцчшщъыьэюя")

    // Insertions: добавим букву в каждую позицию
    for i := 0; i <= len(runes); i++ {
        for _, ch := range alphabet {
            candidate := string(runes[:i]) + string(ch) + string(runes[i:])
            if d.Has(candidate) {
                return candidate, true
            }
        }
    }

    // Substitutions
    for i := 0; i < len(runes); i++ {
        for _, ch := range alphabet {
            if ch == runes[i] {
                continue
            }
            // ... та же логика
        }
    }

    // Deletions
    for i := 0; i < len(runes); i++ {
        candidate := string(runes[:i]) + string(runes[i+1:])
        if d.Has(candidate) {
            return candidate, true
        }
    }

    return "", false
}
```

Для слова из 10 символов это ~10×33 + 10×33 + 10 = 670 кандидатов. С хеш-лукапом — около 5 ms суммарно, что приемлемо для одной интерактивной операции. Без BK-tree, без specialized структур, без оптимизации, и работает.

Чтобы не ловить false positives на коротких словах, fuzzy включается только для слов 6+ символов. Иначе любое 3-буквенное "the" попадёт в матч с "тне" или "тhe".

## Active tap — это не тестировать, а пользоваться нельзя

Самый дорогой урок: active tap отлично работает в `go run`, в Терминале, в TextEdit. Я тестировал именно там. На юзере утилита уходила в краш с непонятной диагностикой.

Причина оказалась глупой. macOS делает строгий permission check на разные tap-режимы. Listen-only можно ставить с обычным Accessibility разрешением. Active tap (с возможностью suppress events) требует более строгой проверки кода — в частности, корректной подписи бинарника. Без подписи на свежем бинарнике диалог Accessibility появляется, но event tap молча не активируется.

Решение — нормальная подпись. Без Apple Developer аккаунта (99 $/год) пользователю приходится через "Open Anyway" в System Settings, и инструкция в README выглядит как восемь шагов: скачать → распаковать → попытаться открыть → получить блок Gatekeeper → пойти в Settings → Open Anyway → дать Accessibility → перезапустить. Каждый шаг отваливает 30-50% юзеров.

С подписью этих шагов — два. С Notarization — один. Это сейчас главная боль.

## Open source: про телеметрию и доверие

Утилита, которая видит каждое нажатие клавиши, технически имеет доступ к паролям, перепискам, всему. У Punto Switcher на Windows в 2018 году обнаружили [передачу набираемых данных на серверы Yandex](https://habr.com/ru/articles/353976/). Caramba официально заявляет, что не передаёт ничего, но проверить нельзя. Вы доверяете разработчику.

Open source ядра тут не идеологическая прихоть, а практическое требование. В коде Bzz нет ни одного http-клиента. Нет аналитики, телеметрии, auto-update проверок. Можно скомпилировать самому, можно нанять аудитора, можно проверить .ipa-дампы (если такая паранойя).

Хранится только локально:
- `~/Library/Application Support/Bzz/config.yaml` — настройки
- `~/Library/Application Support/Bzz/exceptions.json` — что ты отменил через Cmd+Z

Никаких облачных синков, никаких аккаунтов.

## Open core и монетизация

Ядро бесплатное и MIT, навсегда. Pro-фичи (платные) — то, чем готовы платить пользователи, но что не блокирует базовый use case:

- **Custom dictionary** — добавь свои термины, имена клиентов, бренды, чтобы Bzz их не ломал
- Per-app правила с UI вместо config.yaml
- Расширенный fuzzy (edit distance 2)
- Дополнительные раскладки: украинская, казахская, белорусская, немецкая

Цена 490 ₽ разово. Не подписка. У переключателя раскладки нет облака — нет смысла платить ежегодно.

Это сознательное анти-Caramba позиционирование: открытый код, разовый платёж, нет подписки. За два года Caramba берёт 898 ₽, Bzz — 490 ₽ один раз навсегда.

## Что дальше

- Notarization (как только наберу на Apple Developer)
- Поддержка дополнительных раскладок: UK, KZ, BY на очереди — архитектура любую пару QWERTY↔non-Latin поддерживает
- Полноценный Windows-бэкенд (сейчас компилируется и есть базовый функционал, но без tray и без перехвата Enter)
- License-сервер для Pro-ключей через Telegram-бот

Если кто-то хочет добавить свой словарь, поправить мелочь или прислать issue — pull request'ы открыты.

## Полезные ссылки

- GitHub: [github.com/zlopixatel/bzz](https://github.com/zlopixatel/bzz)
- Скачать v0.2.0: [releases/latest](https://github.com/zlopixatel/bzz/releases/latest)
- Сравнение с конкурентами: [Bzz vs Caramba](https://zlopixatel.github.io/bzz/caramba-alternative.html)
- Документация Apple: [CGEventTap Reference](https://developer.apple.com/documentation/coregraphics/cgeventtap)
- Snowball stemmer: [snowballstem.org](https://snowballstem.org/)

Спасибо за прочтение.

---

**P.S. для модерации Хабра:** статья от автора проекта (не реклама), open source с MIT лицензией, конкретные технические детали с code snippets, без призывов "купите". Пара ссылок на сравнительные страницы и репозиторий в конце для тех, кто заинтересуется.
