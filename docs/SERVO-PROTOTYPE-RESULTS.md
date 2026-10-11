# Servo: состояние реализации и замеры

Дата: 2026-10-10. [План реализации](SERVO-IMPLEMENTATION-PLAN.md).

## Реализовано

- `lib/servo` — Git submodule, SHA `43f6f9d2486c53342c92157af4fa0b0936303964`.
- Rust embedder `native/servo-host`, Rust 1.97.1, собственный Cargo.lock.
  Servo: `default-features=false`, только `bundled`; JIT, WebGL/WebGPU/WebXR,
  GStreamer и внутренний multiprocess не включены.
- Выбор **SDL / Servo** в настройках; SDL остаётся значением по умолчанию.
  Markdown, меню и дерево используют существующий renderer.
- HTML/ZIM открываются через исходный HTML, CSS и ресурсы. Servo работает
  в отдельном процессе с одним WebView. SDL принимает только ограниченные
  viewport RGBA, рисует текстуру и прежнюю статусную строку.
- Постоянный сеанс: resize, прокрутка, якоря, выбор/открытие ссылок,
  масштаб, уведомление темы, восстановление scroll/selection после Back.
  Между статьями helper пересоздаётся для освобождения памяти Servo.
- Ошибка helper возвращает текущую статью в SDL и показывает сообщение.
- Источники файлов используют `os.Root`; ZIM имеет отдельный reader,
  ограничение item до копирования, канонические redirects без предварительного
  копирования тела и сериализованное чтение libzim.
- Loopback server: случайный token/cookie, проверка Host, только GET/HEAD;
  внешние ресурсы отменяются delegate. CSP запрещает скрипты контента,
  frames, media и inline data images. Собственная JS glue обслуживает
  управление viewer; произвольные JS команды через IPC не принимаются.
- EGL/GLES pbuffer загружает GPU библиотеки прошивки; PowerVR core symbols
  ищутся также непосредственно в libGLESv2. Для Mesa предусмотрен переход
  с неработающего default display на surfaceless platform.
- Software context доступен отдельно через `ZIMLITE_SERVO_CONTEXT=software`.

## Ограничения памяти

HTML 8 MiB; ресурс 16 MiB; передача за сеанс 32 MiB; два активных ответа,
очередь 16; raster до 4 мегапикселей; viewport до 921600 пикселей и 1280 по оси;
IPC команда 8 KiB, событие 64 KiB; последний кадр/состояние заменяют предыдущие.
Очередь команд 16; helper допускает одну незавершённую команду JS и один запрос
состояния. Лимит ссылок 2048. История приложения 32 записи, cache документов 4,
сохранённые locations 64; дерево не строится для архивов более 100000 статей.

Go watchdog следит за RSS+swap PID helper раз в 100 ms: максимум 500 MiB,
дополнительно ограниченный MemAvailable минус 96 MiB. Сумма RSS+swap
Go приложения и helper ограничена 600 MiB. При доступном бюджете
меньше 160 MiB запуск прекращается. Первый кадр ожидается до 35 s;
незавершённая команда helper — до 5 s.

Это **не доказательство** общего расхода приложения менее 600/700 MiB:
GPU, DOM, libzim cluster decompression, SVG и число кадров анимации этими
лимитами не ограничены. Watchdog может допустить краткий overshoot.

## Подтверждённые результаты

| Среда и документ | Viewport | VmHWM helper | Первый полный кадр |
| --- | --- | --- | --- |
| Desktop amd64, software, небольшая HTML fixture | 640×456 | 177 MiB | 0,12–0,15 s после прогрева |
| Desktop amd64, software, статья Hindi из ZIM | 640×456 | 220 MiB | 0,525 s total |
| Desktop amd64, NVIDIA GLES, HTML fixture | 640×456 | 217 MiB | 0,496 s после создания WebView |
| Desktop amd64, автоматический переход на private Mesa llvmpipe | 640×456 | 224 MiB | 0,743 s total |
| **TrimUI Smart Pro, KNULLI, PowerVR GLES, HTML fixture** | **1280×696** | **79,7 MiB** | **3,089 s total** |
| TrimUI Smart Pro, KNULLI, PowerVR GLES, статья Hindi из ZIM | 1280×696 | 92,6 MiB | 5,102 s total |

На TrimUI probe целиком, включая loopback server и запись PNG, занял 3,852 s.
Кадр проверен визуально: русский текст, внешний CSS, SVG, таблица и якорь;
скрипт контента не исполнился. Это одна небольшая статья и одна прошивка.
Устройство: aarch64, Linux 4.9.191, glibc 2.40, 974 MiB RAM, около 619 MiB
MemAvailable при повторном подключении; zram около 487 MiB. EGL 1.4,
OpenGL ES 3.2, PowerVR Rogue GE8300.

Аппаратный ZIM probe занял 5,996 s целиком. Получен PNG на устройстве;
визуальная проверка этого кадра пока не выполнена. Эти аппаратные замеры
использовали первоначальный bookworm helper; итоговый portable runtime
и постоянный сеанс на handheld ещё предстоит проверить.

На desktop подтверждены persistent resize 640×456 → 400×300, выбранная
ссылка и scroll к якорю (195 CSS px), закрытие процесса. Диагностический
SDL event loop прошёл выбор ссылки, anchor и Back в меню; получен кадр
именно SDL compositor со статусной строкой. Проверено отображение Hindi:
FreeSerif/FreeSans и patch fallback Unifont устранили квадраты вместо букв.
Автоматический software fallback проверен с намеренно недоступным EGL:
после exec в том же PID выбран именно Mesa llvmpipe (LLVM 19), а не GPU.
GLVND vendor ограничен private `egl_vendor.json`.

Последняя Go сборка и lint прошли (0 issues). Ранее `make test` и race для
article/servo прошли до добавления UI integration; это не результат повторного
прогона после всех изменений. Новые unit tests на этой стадии не запускались.

## ABI и упаковка

`Dockerfile.servo` собирает desktop и аппаратный ARM64 probe с GCC 12 /
Debian bookworm. ARM helper требует символы glibc до **2.35**, GLIBCXX до
**3.4.30**; он работает на предоставленной KNULLI 2.40, но **не является**
общим PortMaster runtime для glibc 2.30/2.33.

`Dockerfile.servo-abi30` собирает ARM64 с GCC 10 и старым sysroot.
Сборка завершилась. ELF аудит: helper и 10 CPU библиотек, а также 38 ELF
software runtime требуют GLIBC не выше **2.30**. Сам helper требует GLIBC
2.30 и GLIBCXX 3.4.21. Упаковщик рекурсивно собирает CPU runtime библиотеки
и отвергает любую зависимость с GLIBC >2.30. Это проверка символов, а не
аппаратное подтверждение совместимости со всеми прошивками.
GPU библиотеки и libc прошивки не заменяются. Отдельный private software
Mesa runtime включается только в helper, если firmware GLES context недоступен. Runtime layout:
`servo/run.sh`, `servo/aarch64/bin`, `servo/aarch64/lib`, fonts и licenses.
Launcher делает exec, чтобы watchdog следил именно за PID helper.

Получен `dist/zimlite-servo.zip`: 95,1 MiB в архиве; Servo runtime
занимает около 241 MiB на диске (это не расход RAM). Собранное ARM64
приложение требует GLIBC не выше 2.29. В архиве изменено имя `port.json`
на `zimlite-servo.zip`; основной `portmaster/port.json` сохранён.
Выбор SDL остаётся по умолчанию. Итоговый пакет установлен по SSH на TrimUI
Smart Pro/KNULLI в `/userdata/roms/ports/zimlite`; launcher —
`/userdata/roms/ports/Zimlite.sh`. SHA-256 архива и установленных app/helper
совпали с локальными файлами. `ldd` разрешил все зависимости app и helper.
Добавлена небольшая ZIM-книга Hindi из аппаратного probe. Меню KNULLI
обновлено через локальный `reloadgames`. Приложение после установки не
запускалось; пользователь самостоятельно проверяет итоговый пакет.

Матрица [SERVO-DEVICE-MATRIX.json](SERVO-DEVICE-MATRIX.json): 68 пар
устройство/прошивка с RAM=1024, все aarch64; glibc 2.30/2.33/2.38/2.40.
Полный охват этой матрицы пока не подтверждён. Визуальная проверка ZIM кадра,
управление в полном приложении на handheld, малый MemAvailable и другие
EGL/driver семейства ещё требуют проверки.

## Диагностика после пользовательской проверки, 2026-10-10

Снимок framebuffer полного приложения подтвердил: CSS, рамки и таблицы
рисуются, но текст статьи отсутствует; строка состояния SDL отображается.
Helper создавал кадры за 1084–2686 мс без ошибок или падений.
Комплектные шрифты присутствуют, но `fc-list` с комплектной библиотекой
Fontconfig 2.13.1 и исходным `<dir prefix="relative">fonts</dir>` возвращал
пустой список. Замена каталога на абсолютный путь на том же устройстве
возвращает DejaVu, FreeSans и Unifont. Системная библиотека с исходным
конфигом также видит шрифты. Следовательно, переносимый launcher теперь
создаёт конфигурацию с абсолютным XML-экранированным путём в `servo/cache`.
Изменение применяется при следующем запуске helper; текущий процесс
пользовательского приложения не прерывается. Исправленный полный кадр
ещё необходимо проверить после повторного открытия статьи.

## Повторное зависание и профилирование

После повторной пользовательской проверки устройство зависло целиком при
`Loading article`. Причина (OOM или блокировка драйвера) пока не установлена:
после перезагрузки предыдущий обычный лог оказался пустым, SSH восстановился
с задержкой. После перезагрузки MemAvailable составлял примерно 645 МиБ,
swap не использовался; это не измерение во время сбоя.

Установлена и включена профилирующая сборка с
[отдельным сборщиком и JSON-событиями](SERVO-PROFILING.md). Проверены
контрольные суммы установленных app/helper и запись системного/kernel лога
на устройстве без запуска приложения. Резервная копия до обновления:
`/userdata/system/zimlite-profile-update/backup-20261010T113557Z`.
Пользователь должен воспроизвести сбой для анализа последней папки `profiles`.
Сборки app/ARM64 Servo, `cargo fmt`, shell syntax, `make lint` (0 issues)
успешны. Итоговый ZIM-рендер с профилированием пока не проверялся.

## Анализ профиля зависания, 2026-10-10

Профиль `20261010T124109Z-5583` сохранён с устройства. Последний системный
замер перед обрывом (12:41:13 UTC): MemAvailable=639352 КиБ (624 МиБ),
RSS Zimlite=35880 КиБ (35 МиБ), занято swap системы всего 84 КиБ.
Go heap в предыдущем замере — 3356264 байт. Preflight `index` успешно
прочитал 7459 байт HTML; helper PID 5711 создан, watchdog запущен.
После этого прекращаются все три журнала; первой отметки `process.start`
helper в сохранённом app.log нет. В сохранённом dmesg нет OOM/panic.
Это не подтверждает исчерпание RAM; момент сбоя между замерами не записан.

Тот же установленный helper отдельно запускается без EGL и возвращает usage.
Программный Mesa 20.3.5/LLVM 11 создал кадр `index` 1280×702 за 7819 мс;
HWM=203860 КиБ (199 МиБ), RSS перед отправкой кадра=200540 КиБ, VmSwap=0.
Устройство осталось доступным. Три отсутствующих ресурса заставили старый
probe отказаться от сохранения PNG, поэтому добавлен диагностический флаг
`--allow-missing-resources` (строгая проверка по умолчанию сохранена).

Launcher теперь использует программный Mesa по умолчанию, также установлен
на устройстве. Аппаратный `auto`/`gles` остаётся явной настройкой окружения.
Это экспериментальный обход предполагаемого сбоя аппаратного пути, а не
доказательство конкретной ошибки PowerVR. При повторном software probe с
новой утилитой (включены JSON-события и сохранение неполного PNG) SSH снова
стал недоступен до получения результата. Новый профиль находится в
`/userdata/system/zimlite-software-diagnostic/png-check/`; его ещё требуется
прочитать после восстановления доступа. Устойчивость обхода не подтверждена.

## Исправление активных записей на FUSE и диагностика SDL

Второй профиль содержит только `app.start`, ещё до старта Servo;
MemAvailable=662116 КиБ (646 МиБ), swap=0. На устройстве `/userdata` —
`fuseblk`, `/tmp` — tmpfs. Новый probe с тем же JSON-профилированием в
`/tmp` завершил рендер `index` за 13,34 с, helper HWM=217828 КиБ (213 МиБ).
Зависимость от места записи требует учёта FUSE/синхронного I/O, а не только
GPU; конкретная блокировка внутри FUSE пока не доказана.

Убраны fsync каждого JSON-события и фоновый sync во время работы. Launcher
пишет активные логи в `/tmp`, архивирует после нормального выхода; после
перезагрузки RAM-логи исчезают. Fontconfig и Mesa используют частный кэш
в `/tmp` (Mesa ограничен 8 МиБ; заданы старые и новые имена переменных).
После архивирования временные логи удаляются. Исправленная сборка установлена.

Временный диагностический harness использовал настоящий SDL renderer и
`ui.App.Run`, ZIM-статью `1998-99_एन_बी_ए_सीज़न`, события прокрутки и масштаба.
Он нормально завершился: helper HWM=220204 КиБ (215 МиБ), первый кадр
18,675 с на холодном запуске; min MemAvailable=461024 КиБ (450 МиБ).
Scroll state изменился на 315 px, ошибок fallback нет. SDL ReadPixels
захватил фон меню EmulationStation, которое при SSH-запуске продолжает
рисовать экран. Поэтому визуальное подтверждение настоящего PortMaster
запуска остаётся необходимым. Выбор ссылки ещё не подтверждён (URL пуст).
Однокадровый рендер той же статьи после прогрева завершился за 4,30 с,
helper HWM=218800 КиБ (214 МиБ), без зависания.

## Проверка через frontend KNULLI после исправлений

2026-10-10 повторный диагностический запуск выполнен через HTTP `/launch`
EmulationStation с существующим PortMaster launcher. Временный harness
использовал установленный ARM64 helper и тот же `ui.App.Run`/SDL renderer;
после запуска обычный launcher восстановлен. Статья NBA отобразилась на
физическом экране: framebuffer и SDL-снимок содержат текст, таблицу,
прокрутку и выделенную ссылку. Первый кадр после прогрева — 2240 мс,
helper HWM=219032 КиБ (214 МиБ), min MemAvailable=502548 КиБ (491 МиБ).
Прокрутка изменилась с 0 до 315 px; после масштаба и выбора ссылки возвращён
непустой URL, на снимке видно выделение. Диагностика завершилась нормально
без перезагрузки, падения или fallback. Переход по ссылке этим запуском
не проверялся. Прежний пустой URL не воспроизвёлся.

Логи и снимки сохранены локально в
`dist/servo-diagnostics/zimlite-frontend-evidence/`; на устройстве — в
`/userdata/roms/ports/zimlite/profiles/` (запуск 13:31 UTC).
Это подтверждает программный путь на данном TrimUI/KNULLI и данной статье;
остальные модели/прошивки и большие ZIM ещё требуют замеров. Источник
прежних зависаний внутри ядра/драйвера точно не установлен.

## Повтор обычного бинарника с `/userdata`

После успешной frontend-диагностики запущен обычный установленный Go
бинарник с аргументом ZIM. В RAM app.log записаны инициализация SDL и
`Starting application event loop`, затем доступ снова прекратился. В этот
момент восстановление launcher через `cp` также перестало продвигаться;
Ctrl-C сначала вернул SSH prompt, но последующие чтения остановились.
Диагностический бинарник выполнялся с tmpfs, обычный — с FUSE `/userdata`.
Это новая существенная разница; успешный harness не подтверждает
устойчивость обычной установки. Последний профиль ещё не удалось забрать.

Подготовлен launcher, который при наличии Servo копирует только Go
бинарник (~22 МиБ) в уникальную папку `/tmp` перед стартом SDL и удаляет
копию после выхода. `KIWIX_APP_DIR` сохраняет исходное расположение
конфигурации, кеша каталога и helper; книги остаются в исходной папке.
Завершены ARM64 сборка, shell syntax и lint (0 issues). Версия установлена
после перезагрузки; SHA256 Go binary:
`f61e9a8a182751b574ead0193f4d2f726303b2ca0318fbac6d9af477ba65ad5f`.
Повтор через frontend с главной страницей ZIM снова перестал отвечать:
через 8 с SSH-команда чтения RAM app.log не вернула результат.
Таким образом перенос одного Go бинарника в tmpfs проблему не устранил.
Нужна проверка остальных отличий: `index` против NBA-статьи, начальное
SDL-окно 640×480 против 1280×720, helper/библиотеки и ZIM по-прежнему на SD.
Конкретная причина блокировки FUSE/ядра пока не доказана.

Пользователь предположил неисправность SD. В сохранённом kernel.log карта
определена как mmc0/mmcblk0 (SD64G); ошибки mmc1 относятся к SDIO и сами
по себе не подтверждают повреждение карты. Есть предупреждение dirty FAT
на `/boot` (mmcblk0p4) после аварийных выключений. В текущем dmesg перед
последним запуском подходящих сообщений mmc0 не было; журнал уже был
заполнен повторными событиями USB, поэтому это не полноценная история
загрузки. Контрольные суммы app/helper с карты совпали с локальными.
Аппаратный дефект SD и проблема FUSE/exFAT пока остаются гипотезами.


## Формулы MediaWiki SVG на desktop, 2026-10-11

Пользователь показал пропавшие формулы в `正态分布` из
`wikipedia_zh_mathematics_maxi_2026-06.zim`. Воспроизведено текущим desktop
helper. Сервер с очередью 16 отклонял 82 из 134 запросов ресурсов; успешные
чтения ошибок не имели. Очередь поднята до 512 ожидающих запросов: до чтения
они не держат ресурс в памяти, параллельных слотов по-прежнему 2, доступ к
libzim последовательный, HTML/resource/session лимиты сохранены.
Первое исправление убрало HTTP-ошибки, но формулы оставались сломанными.

SVG формул имеют MIME `image/svg+xml; charset=utf-8; profile="https://www.mediawiki.org/wiki/Specs/SVG/1.0.0"`.
Та же формула через FileSource с `image/svg+xml` отображалась. После
нормализации только SVG HTTP-заголовка к `image/svg+xml` формулы появились
и в статье. HTML/CSS charset не менялся. Повторный probe: 134 ответа,
из них 113 SVG, 0 resource failures, первый кадр 410 мс, helper HWM
351332 КиБ (343 МиБ). Снимки и события — `dist/servo-diagnostics/svg-fix/`.
Linux app пересобран в `dist/zimlite-linux-pc`, обновлён desktop архив;
lint — 0 issues. Исправлен также встроенный `<svg>`: Servo сериализует его во внутренний
`data:image/svg+xml;base64,` URL, ранее отменявшийся проверкой origin.
Helper разрешает только такие подресурсы размером URL до 8 МиБ;
главные документы и внешние сетевые origin не разрешены. CSP неизменён,
авторские скрипты отключены. После пересборки desktop helper внешний
SVG и встроенный зелёный прямоугольник/жёлтый круг отобразились в fixture.
Persistent-сессия китайской статьи (тёмная тема, scroll=260) отобразила
формулу плотности и обозначения; helper HWM=351840 КиБ (344 МиБ),
первый кадр 398 мс, получены 3 кадра. Обновлены Linux бинарник, helper
и ZIP. `rustfmt --check` прошёл на локальном stable toolchain; образ
cross-toolchain не содержит cargo-fmt.
Проверка handheld с этим исправлением не выполнена.

## Desktop fallback по общему бюджету, 2026-10-11

На последующем пользовательском снимке видна строка `Servo unavailable;
using SDL: application and Servo exceeded combined memory budget`.
Измеренная сумма — 682504 КиБ RSS+swap, выше прежнего общего лимита
614400 КиБ. Поэтому изображение показывает native SDL fallback:
таблица разложена линейно, CSS-инверсия SVG формул не применяется.
Диагностический снимок был из небольшой Go-программы с Servo Session,
которая не держала память полного SDL/UI приложения.

Watchdog теперь выбирает общий лимит по Linux MemTotal: при >1536 МиБ
RAM — 1024 МиБ, иначе прежние 600 МиБ. Максимум самого helper — 500 МиБ,
порог доступной памяти и остальные ограничения не менялись. При ошибке
чтения MemTotal остаётся меньший лимит. Linux бинарник и ZIP обновлены,
lint — 0 issues. Устойчивость полного SDL-приложения после изменения ещё
требует пользовательской проверки; это не подтверждение исправления
handheld зависаний или уменьшения потребления RAM.

## Запуск desktop

```sh
make servo-builder
make servo-probe
# Диагностический один кадр:
dist/servo-probe --helper scripts/servo-probe-helper.sh \
  --source native/servo-host/fixtures/index.html --output /tmp/servo-probe.png
# Пользовательское приложение: helper должен быть доступен локально,
# Docker wrapper предназначен для probe, а не watchdog приложения.
KIWIX_SERVO_HELPER=/absolute/path/zimlite-servo-host ./zimlite article.html
```

Выбрать Servo в Settings. Не копировать desktop Mesa/GPU библиотеки на handheld.


## 2026-10-11: SDL formulas and dynamic memory reserve

Native SDL now identifies MathJax SVG formulas by their root title label and
inverts RGB in the dark theme, preserving alpha. Theme changes invalidate image
textures; light and sepia retain original colors. Ordinary SVG pictures are not
inverted. This applies to formulas carrying MathJax SVG metadata.

The current watchdog supersedes the earlier fixed helper and combined RSS caps.
It samples Linux MemAvailable every 100 ms and terminates Servo below 50 MiB,
letting the existing SDL fallback restore the current article and position.
Preflight checks the same reserve before starting the helper. Availability is
profiled once a second and memory pressure is logged explicitly. This is a sampled
system memory guard, not a kernel-enforced allocation limit.

Linux main binary and desktop archive rebuilt successfully; `make lint` reports
0 issues. Runtime theme switching and memory-pressure fallback have not been
exercised in this revision, and it has not been deployed to the handheld.


## 2026-10-11: remove article-count restriction from tree browsing

The 100,000-article gate was an application restriction added to avoid the
complete C++ and Go title/path lists, not a libzim or renderer limitation. It has
been removed. Real ZIM readers now expose ArticleAt via iterByTitle().offset(i,1).
The UI uses an indexed tree with at most 32 title ranges per expansion and 128
article leaves in a final group. Ranges display their first and last title and
article count, covering the full title index without truncation. Expanding a
sibling collapses the previous indexed branch and releases its descendants.
This changes prefix-letter grouping into title-range grouping for real ZIM files;
existing navigation keys and opening leaf articles remain the same. No article
bodies are read while building the tree. Alternate readers without random title
access retain the existing in-memory prefix tree.

Errors while reading the index are surfaced with localized messages and can be
retried. The obsolete count-limit warning was removed. Linux main and desktop
archive rebuilt; make lint reports 0 issues. No runtime measurements on an
archive with millions of entries were performed for this revision.
