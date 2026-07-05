Критические проблемы
1. errcheck отключен в .golangci.yml
Файл: .golangci.yml:5
Весь проект без проверки необработанных ошибок. 40+ мест где err игнорируется (особенно defer, Close(), sdl.PushEvent)
2. Config — глобальный mutable singleton
internal/config/config.go:17 — var currentConfig Config
Нет мьютекса, нет thread-safety. Фоновая goroutine (checkInternetAsync) читает/пишет.
3. Гонка в DocumentLoader.internetAvailable
loader.go:31 — bool читается/пишется из разных goroutine без синхронизации.
4. CGO memory management risk
bridge.cpp:104 — malloc(sz) без проверки sz == 0
wrapper.go:24 — C.lunasvg_render — потенциальный segfault при пустых данных
zim.go:115 — unsafe.Add с uintptr(i)*size — корректно, но хрупко
5. renderTables(false) в draw.go
Метод renderTables всегда вызывается с visible=false — таблицы никогда не рендерятся.
Баг или dead code.
6. StatusBar без текста при переключении темы
draw.go:209-211 — statusOverride очищается только при новом вызове.
Если statusOverride="", статус-бар пуст (нет подсказок по умолчанию).
7. Скачивание без HTTPS проверки
storage.go:69 — http.Client{} без таймаута (исправлено только в loader.go). Разные клиенты в проекте:
- fetchFeed — 5s timeout
- Download — timeout не установлен
- ResourceLoader — 3s timeout
- Нет единообразия.
🟡 Средние проблемы
8. todo.md:8 — многоязычный интерфейс
9. Поиск по статьям (todo.md:13)
10. Texture cache никогда не очищается
renderer.go:195 — ClearCache() вызывается только при relayout() и Zoom().
Нет лимита кэша — при длительной работе с большим числом разных документов память растёт.
11. Дублирование formatSize
menu.go:151 и library.go:65 — идентичная функция.
12. font.SetStyle на каждый рендер
draw.go:124-126 — стиль шрифта меняется на каждый glyph. Дорого.
Лучше: разные шрифты для bold/italic.
13. Вложенные списки — потеря структуры
markdown/parser.go:296-319 — flattenList превращает nested lists в плоский список. Потеря иерархии.
14. unsafe в image_manager
image_manager.go:150 — unsafe.Pointer(&anim.Frames[currentFrame].Pix[0]) — если слайс пуст, это UB.
image_manager.go:244 — то же.
15. Магические числа
image_manager.go:68-73 — scrollStep = 40
layout.go:106 — maxCodeW := s.maxW - 24
input.go:196-198 — -val/16000 — что такое 16000?
16. unsafe.Slice в SVG wrapper
wrapper.go:39 — unsafe.Slice в Go 1.17+. Проект на Go 1.25, OK. Но если gh * stride больше len(pixels) — panic.
17. Отсутствие graceful shutdown
ui.go:240 — defer app.shutdown() нормально, но если Run() внутри горутины — shutdown не вызовется.
Ticker goroutine (ui.go:245-257) никогда не дожидается завершения.
🟢 Мелкие улучшения
18. goconst — много повторяющихся строк:
"virtual:menu" (10+ мест)
"text/html" prefix check (3 места)
virtual:, zim:, virtual:tree
19. No forward в SimpleNavigator
navigation.go:77 — Forward() реализован, но в UI не используется.
20. SDLEvent vs UserEvent
sdl.PushEvent(&sdl.UserEvent{Type: sdl.USEREVENT}) — 6 мест, дублирование. Вынести в helper.
21. debugEvent всегда дебаг
input.go:30 — debugEvent("KEY", int(sc), 0) — даже без KIWIX_DEBUG считается. Можно обернуть в conditional.
22. test_comprehensive.md — тестовые данные в корне
23. Отсутствие CI — нет .github/workflows (не проверял, но в списке файлов нет).
24. Ошибка в library.go:32
Language string `xml:"http://purl.org/dc/terms/ language"`
Пробел перед language — вероятно опечатка, должно быть "http://purl.org/dc/terms/language".
Итог
Категория	Количество
🔴 Критические	7
🟡 Средние	10
🟢 Мелкие	7
Всего	24
Главное: починить renderTables (dead code), добавить errcheck, синхронизировать shared state (config, internetAvailable), унифицировать HTTP клиенты, защитить CGO границу от пустых данных.