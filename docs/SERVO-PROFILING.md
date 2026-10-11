# Профилирование Servo на handheld

## Включение и сохранение

Создать пустой файл `zimlite/servo/profile.enabled` рядом с установленным
приложением либо установить `ZIMLITE_PROFILE=1` для PortMaster launcher.
Запустить Zimlite обычным способом и открыть проблемную статью.

Основные журналы записываются в `/tmp/zimlite-profile.XXXXXX/` (tmpfs).
Launcher сразу создаёт `zimlite/profiles/UTC-ДАТА-PID/` на карте. Отдельный
процесс каждые 2 секунды копирует туда последние 256 КиБ каждого журнала
под именами `events.jsonl.live`, `app.log.live`, `system.log.live`,
`kernel.log.live`. При обычном выходе сохраняются полные журналы.
`run-state.txt` содержит `RUNNING` и путь RAM-журналов; при обычном выходе
состояние меняется на `COMPLETE`.

| Файл | Содержимое |
| --- | --- |
| `app.log` | Журнал приложения и stderr helper: этапы EGL, ServoBuilder, WebView и снимка кадра, память helper, причина остановки |
| `events.jsonl` | Открытие статьи, preflight, размер/MIME/время чтения ресурсов, PID helper, первый кадр, память каждые 250 мс, ошибки, остановка, освобождение кешей и этапы фоллбека; Go heap/GC/стек/goroutines/cgo каждую секунду |
| `events.previous.jsonl` | Предыдущая часть событий после ротации |
| `system.log` | RAM/swap/Slab/CMA/Dirty, CPU ticks, page faults/reclaim, PSI при наличии, RSS/HWM/Swap/Threads и ожидание процессов каждую секунду |
| `system.previous.log` | Предыдущая часть системного журнала |
| `kernel.log` | Снимок dmesg каждые 5 секунд: сообщения OOM и драйверов |

Каждые 5 секунд сборщик читает PSS/private memory из `smaps_rollup`, если
ядро предоставляет его, и `wchan` потоков. Сборщик и копирование на карту
работают независимо от UI и друг от друга. Одновременно работает только
один процесс копирования: заблокированная запись на карту не создаёт новые
процессы и не блокирует сборщик.

Синхронные `fsync`/`sync` не используются: на исследуемом KNULLI `/userdata`
использует FUSE, и синхронная запись журналов ранее сопровождалась зависанием.
Сохранение `.live` выполняется через временный файл и переименование, но
сбой питания/ядра/карты может потерять последние записи. Для наиболее полного
журнала сохранять RAM-папку по SSH/ADB до перезагрузки.

События ротируются на 4 МиБ, системный журнал — примерно на 2 МиБ;
сохраняются две части каждого. `.live` занимает до 1 МиБ на запуск (плюс
один временный снимок). `app.log`, объём dmesg и число папок запусков отдельно
не ограничены. RAM-журналы влияют на расход памяти. Тексты статей, токены
HTTP и окружение процессов не записываются; пути/названия статей присутствуют.

## Скорость прокрутки

При включённом профилировании helper пишет в `app.log`:

- `servo.profile.frames`: FPS и число кадров за интервал, обработанные и
  объединённые команды движения, среднее время `paint_avg_us`,
  `readback_avg_us`, `ipc_avg_us`, суммарное `spin_total_us` и максимальный
  интервал между кадрами `max_gap_ms`. Времена стадий — микросекунды.
- `servo.profile.raster`: переход между полным растром (`scale=1`) и растром
  движения (`scale=0.5`). Ширина/высота в сообщении — исходный viewport.
- `servo.profile.raster.ready`: первый завершённый кадр нового растра,
  его физические размеры и время перехода `elapsed_ms`.

Вертикальная прокрутка использует compositor API Servo, без JavaScript на
каждый шаг. Последовательные команды движения объединяются до отрисовки
(не более 16 за один проход); остальные команды сохраняют порядок.
Обновление состояния страницы при движении ограничено одним запросом в
200 мс. Таблица прокручивается в собственном контейнере.

В software runtime при движении по умолчанию используется половина
разрешения по каждой оси. Патч `servo-viewport-scale.patch` меняет HiDPI и
размер framebuffer одной операцией, отправляя в layout только окончательный
CSS viewport. Отдельные вызовы resize/HiDPI временно меняли CSS-ширину вдвое
и нарушали раскладку. Во время перехода helper не выдаёт промежуточные кадры:
снимок Servo ждёт готовности новой сцены, а SDL сохраняет предыдущий кадр.
Команды движения ждут готовности новых scroll/hit-test деревьев. Переход
ограничен 5 секундами; ошибка или таймаут запускают обычный fallback.
Сохраняются CSS-ширина страницы и размер текста. SDL масштабирует полученный
кадр. После 1 секунды без новых команд движения helper возвращает полный
растр; получение чёткого кадра дополнительно требует времени отрисовки.
Сглаживание текста в software режиме — обычное, без субпиксельных проходов.

`ZIMLITE_SERVO_SCROLL_SCALE=1` отключает снижение разрешения;
поддерживаются только `0.5` и `1`. Прямой запуск desktop helper по умолчанию
использует `1`. Software launcher выбирает до четырёх потоков llvmpipe;
число можно переопределить через `LP_NUM_THREADS`.
Для растра движения требуется совместное обновление приложения и helper:
старое приложение ожидает только кадры полного размера.

Для замера без SDL-окна используется `servo-probe --scroll-duration 12s`
с обычными аргументами `--source`, `--article`, `--helper`, `--width`,
`--height`. Профилировщик сохраняет обычные watchdog и бюджеты ресурсов,
отправляет движение каждые 33 мс со сменой направления каждые 60 команд,
затем ещё 3 секунды принимает кадры после остановки. Длительность движения
ограничена 30 секундами. `--scroll-pause 800ms` добавляет паузу после каждых
60 команд для измерения повторных переходов между растрами; допустимо
от 0 до 2 секунд. Итоговая строка `Servo scroll profile` содержит FPS,
число команд, максимальную паузу и последнюю позицию.
`--scroll-frames-dir /tmp/frames` дополнительно удерживает не более пяти кадров
начала/смены разрешения (до 18 МиБ для максимально допустимого viewport).
PNG записываются после остановки helper, чтобы кодирование и запись не
влияли на скорость прокрутки. Снимки нужны для проверки раскладки:
одни показатели FPS не обнаруживают визуальных ошибок перехода.
Не запускать probe
одновременно с приложением на устройстве с 1 ГБ RAM. Такой замер исключает
затраты SDL/UI; его FPS не является замером всего приложения.

## Защита памяти и остановка

По запросу пользователя движок может использовать всю доступную физическую
память сверх резерва 50 МиБ. Используется Linux `MemAvailable`, а не `MemFree`:
возвращаемые страницы файлового кеша входят в доступную память.

- Проверка каждые 50 мс, независимо от SDL и первого кадра.
- Постоянный системный резерв 50 МиБ для завершения helper и фоллбека.
- При запуске: `MemAvailable − 50 МиБ`. При доступном бюджете менее 192 МиБ
  Servo не запускается. Swap не увеличивает бюджет физической памяти.
- Во время сессии: `MemAvailable + RSS helper − 50 МиБ`, сумма ограничена
  физической RAM. Фиксированные пределы 384 МиБ / 1 ГиБ и коэффициент 2/3
  удалены. RSS уже занял память, поэтому добавляется обратно при оценке общей
  ёмкости. Бюджет меняется вместе с потреблением остальных процессов.
- Остановка при достижении системного резерва или прогноза его пересечения
  за 200 мс. Прогноз применяется при MemAvailable ниже 100 МиБ.
- Swap helper свыше 16 МиБ вызывает остановку только при MemAvailable ниже
  100 МиБ. Для самого zimlite учитывается рост swap от 4 МиБ между близкими
  замерами (до 250 мс), также только ниже 100 МиБ доступной RAM.
  Стабильный swap неактивных страниц при достаточной RAM не вызывает фоллбек.
- Без первого кадра за 20 секунд или при превышении бюджетов ресурсов
  загрузка прекращается. Ограничения размеров HTML, изображений и IPC сохраняются.
- Профиль включает RSS/swap самого приложения, CMA и SUnreclaim. Некорректный
  CmaFree больше CmaTotal помечается `cma_valid=false`; эти показатели не
  используются для вычисления дополнительного запретительного лимита.

Это более свободная политика, чем прежний резерв ~195 МиБ. На старом ядре
TrimUI показатели CMA противоречивы; резерв 50 МиБ и MemAvailable не гарантируют,
что драйвер сможет выполнить любую физически непрерывную аллокацию.

При ошибке HTTP-соединения закрываются, helper получает принудительное
завершение всей группы процессов. UI ждёт его остановку максимум 500 мс, затем закрытия ресурсов до 100 мс;
зависшее чтение libzim завершается отдельно, после чего закрывается архив.
Перед возвратом из обычного Close освобождается владение предыдущей сессией,
поэтому следующий запуск не уходит в фоллбек из-за краткой гонки очистки.
При действительно заблокированном чтении новый helper остаётся запрещён до
его завершения. На Linux
helper получает `oom_score_adj=1000`, если это разрешено, чтобы ядро выбрало
его первым при OOM; это дополнительная мера, а не основной механизм.

Затем UI удаляет кеш документов и текстуру полного движка, возвращает
неиспользуемые страницы Go heap системе и строит простой документ из уже
сохранённого HTML. Повторное чтение/распаковка статьи для фоллбека не нужны.
Положение и выбранная ссылка восстанавливаются. Автоматического возврата
на Servo в текущей статье нет.

У простого рендерера также ограничены размер ресурса (16 МиБ), SVG (2 МиБ),
bitmap (1 мегапиксель), число кешированных текстур (8), анимаций (2) и память
одной GIF-анимации (16 МиБ). GIF проверяется до декодирования всех кадров;
при слишком большой анимации используется первый кадр. Размер SVG bitmap
проверяется до выделения памяти в LunaSVG.

Это защита по измерениям, а не жёсткая гарантия расхода RAM: выделение между
отсчётами, нативная распаковка libzim, ядро и графический драйвер остаются
за пределами полного контроля. На исследуемом KNULLI нет memory cgroup.
`RLIMIT_AS` не используется: Servo резервирует большое виртуальное адресное
пространство, которое не соответствует физическому расходу RAM. Если ядро
уже зависло, watchdog не сможет исполняться и освободить память.

## Как читать после зависания

После перезагрузки найти последнюю папку `zimlite/profiles/` и читать `.live`.
`RUNNING` означает, что обычное архивирование не завершилось; свежий `dmesg`
после перезагрузки обычно не содержит сообщения предыдущего сбоя.

- `servo.watchdog`: резерв, динамическое разрешение на RSS, пороги давления swap и интервал проверки.
- `servo.memory_sample`: доступная RAM, RSS/Swap/HWM helper, RSS/Swap zimlite, CMA/SUnreclaim, наличие кадра,
  текущий `helper_budget_kib` и исходный `startup_helper_budget_kib`.
- `servo.memory_pressure`: измерения и конкретная причина остановки.
- `servo.error`, `servo.helper.exited`, `servo.resources.closed`: ошибка,
  подтверждённый выход процесса, завершение чтений и закрытие архива.
- `servo.close.deferred`: UI перестал ждать зависшую остановку; это не
  подтверждение завершения helper. Сопоставлять с `servo.helper.exited`.
- `article.memory.released`: RAM до/после очистки Go-кешей.
- `article.fallback.cached_html`, `article.simple.parse.begin/end`,
  `article.fallback.ready`: повторное использование HTML, разбор и готовность
  простого рендерера.
- `resource.begin` без `resource.end`: незавершённое чтение или ожидание libzim.
- Рост Go heap сопоставлять с RSS приложения; остальные выделения могут
  принадлежать libzim, SDL и отображённым файлам. Сумма RSS повторно учитывает
  общие страницы, поэтому PSS точнее при наличии.
- Низкий MemAvailable, рост SUnreclaim/reclaim/swap и сообщения ядра помогают
  отличить давление памяти от ожидания накопителя/драйвера. RSS/PSS не
  охватывает все выделения GPU; отсутствие OOM само по себе ничего не исключает.

Для отключения удалить `servo/profile.enabled` и убрать переменные
`ZIMLITE_PROFILE`/`KIWIX_PROFILE_DIR`, если они были заданы вручную.

## Speed and Quality modes

Settings → Full renderer → Render mode selects a persisted `render_mode` in
`config.json` (`speed` or `quality`). Translations cover all 13 UI languages.
The PortMaster launcher sets `KIWIX_DEFAULT_RENDER_MODE=speed` for configurations
without a saved valid mode; desktop defaults to `quality`. A saved choice wins.
The mode applies when an article is opened; entering settings stops the current
article helper, so returning to it starts the newly selected mode.

- **Speed:** fit the article viewport within 640×480, preserve the content area's
  aspect ratio, and never upscale the helper viewport beyond the screen size.
  Layout uses this small viewport with HiDPI scale 1. SDL enlarges its frames.
  For a 1280×720 window with an 18-pixel status bar this is 640×351. The helper
  receives `ZIMLITE_SERVO_SCROLL_SCALE=1`, disabling motion/idle raster changes
  and their frame-readiness wait. Text and responsive tables reflow for the
  smaller width, rather than keeping the full screen's CSS layout.
- **Quality:** retain the existing bounded screen viewport (at most 1280 per
  dimension and 1280×720 total pixels) and use motion scale 0.5. Atomic half/full
  transitions retain layout and wait for a valid frame; the initial motion
  pause remains possible in this mode.

`servo.start.begin` profiles the actual viewport and `motion_scale`. Input and
mouse coordinates are converted to article pixels; fractional scroll deltas
are retained when a low-resolution image is enlarged. System memory monitoring
and fallback remain active in both modes. No FPS improvement is claimed until
measured with the new small-width layout on the target device.

## Scroll diagnostics

With profiling enabled, `servo.state` records `scroll_y`, `scroll_height`,
`viewport_height`, selected link and horizontal table position/range at most
four times per second. Native state sampling also runs when no new frame is
produced, to reveal commands that fail to move the page. `servo.frame` records
received frame sequence and dimensions at the same bounded rate.

The native one-second `servo.profile.frames` summaries include applied
`scroll_commands`/`scroll_delta` and `table_commands`/`table_delta` for relative
pixel motion. Deltas include the merged commands actually applied to Servo.
Compare these with `servo.state` and `servo.frame` timestamps: input without
scrollY change indicates a scroll target/extent issue; scrollY change without
frames indicates rendering/readback/IPC delay. No article scripts, credentials
or authentication URLs are added to these diagnostics.

Horizontal stick input targets a visible wide table: the selected link's table
has priority if visible, otherwise the table nearest the viewport centre is
used. Selection is optional, including for tables without links. Target
resolution is shared by helper input and state diagnostics. The SDL adapter
forwards motion without gating it on potentially stale table state; when no
wide table is visible, the helper leaves the article still.
