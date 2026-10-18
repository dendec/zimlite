# Handheld profile — 2026-10-11

Device: TrimUI Smart Pro, KNULLI/Linux 4.9.191, physical RAM 997816 KiB.
Application build: `a79fbde`, software rendering through Mesa llvmpipe.
Source: device run `profiles/20261011T054737Z-13698`, with full RAM logs from
`/tmp/zimlite-profile.sbx3Am`, copied locally before updating the application.

## Observed fallback

Both memory-triggered fallbacks opened the main page `index` of
`wikipedia_ru_chemistry_maxi_2026-07.zim`. The original HTML was 26715 bytes.

| UTC | Helper RSS / budget, KiB | Available RAM at guard / after cleanup, KiB | Helper exit | Simple renderer ready |
| --- | --- | --- | --- | --- |
| 05:47:49.873 | 304460 / 302059 | 441440 / 679792 | +91 ms | +367 ms |
| 05:48:34.099 | 283004 / 280630 | 407972 / 625292 | +134 ms | +370 ms |

`servo.memory_pressure` identified the RSS+swap helper budget as the cause.
Helper swap was zero at both thresholds. `servo.helper.exited` confirmed
`signal: killed`, followed by `servo.resources.closed`. Fallback used
`article.fallback.cached_html` instead of reopening the article. HTML parsing
finished in 56 and 37 ms. Both runs reached `article.fallback.ready`.

An intervening article successfully produced a Servo frame in 9561 ms,
with helper peak RSS 232652 KiB. Logs confirm that the helper budget can
stop the main page while other articles remain renderable.

The captured kernel log contains no OOM-killer event. The saved session does
not contain `app.stop` and its archive marker remains `RUNNING`; these logs
do not establish how the application was subsequently terminated. They do
confirm two completed early fallbacks with hundreds of MiB still available.

## Input change following the profile

Document stick events now retain the mapped X/Y values until release and do
not also generate a discrete vertical scroll action. The frame loop integrates
both axes using one bounded analog-hold calculation: linear deflection strength
outside the dead zone, fractional pixel accumulation, continuous exponential
hold acceleration after 200 ms, maximum multiplier 16, and at most 100 ms
catch-up after a stall. Neutral input or direction reversal resets the hold.

Vertical movement scrolls the article. Horizontal movement sends signed pixel
deltas through `table_scroll_by` to the selected link's table container;
article text stays in place. The helper retains its older `table_scroll`
operation for compatibility with older application builds. The updated
application requires the updated helper for pixel-based table scrolling.

## Scroll performance investigation

The subsequent GUI run `profiles/20261011T062212Z-19119` also completed an
RSS-budget fallback for the Hindi archive's main page; the next article
`भौतिक_शास्त्र` remained in Servo. This article was used for all scroll probes
below, in `wikipedia_hi_mathematics_maxi_2026-07.zim`, at a 1280 × 702 viewport.
The probe preserves the production memory watchdog and resource budgets;
no SDL window/application was running concurrently. Commands were sent every
33 ms, ±24 CSS pixels, with direction changes every 60 commands, for 12 seconds.

| Configuration | Motion frames / FPS | Maximum frame gap |
| --- | --- | --- |
| Previous helper, JS scrolling, 2 llvmpipe workers | 11 / 0.92 | 1800 ms |
| Compositor scrolling + bounded command merging, 2 workers | 24 / 2.00 | 790 ms |
| Same, 4 workers | 35 / 2.92 | 535 ms |
| Final: half-resolution motion, SDL scaling, grayscale AA, 4 workers | 78 / 6.50 | 343 ms |

Logs: `/tmp/zimlite-scroll-before`, `/tmp/zimlite-scroll-after`,
`/tmp/zimlite-scroll-threads4`, `/tmp/zimlite-scroll-final` on the device.
Their `probe.log` and `events.jsonl` were copied locally after completion.

The initial bottleneck was both serial JavaScript commands and software paint:
with compositor scrolling but two workers, paint took 270–630 ms per frame,
readback about 30 ms, and IPC about 15 ms. A trial that scaled reduced images
back to viewport size inside the helper added approximately 500 ms of CPU
resize time, so that implementation was removed. Scaling now happens in SDL.

In the final run, steady paint took approximately 76–110 ms, readback 7–8 ms,
and IPC 4 ms. Interval FPS ranged around 6–8. The helper first frame arrived
at full resolution; there was one transition to `scale=0.5` during motion and
one return to `scale=1` after release. Matching HiDPI and framebuffer scale
retains the CSS viewport; this changes temporary raster quality, not the
article's logical font size or width. A coalesced zero delta still counts as
input activity, preventing spurious full-resolution transitions on reversal.

The final watchdog reserve was still 199563 KiB. No memory fallback occurred
during these short scroll probes. These measurements cover helper throughput,
not total GUI FPS or physical stick latency, and do not demonstrate the same
performance for every article. Software painting remains the dominant cost.

The paired ARM64 app/helper and software launcher were installed through ADB.
Previous app and helper are retained as `.before-fast-scroll` backups. The
PC build and bundled helper were updated from the same sources. Changes in
this follow-up are not committed.

## Premature fallback during scroll: dynamic budget correction

GUI run `profiles/20261011T074558Z-1203` (RAM logs from
`/tmp/zimlite-profile.sgZrqe`, copied to the workstation) showed the reported
Markdown transition on `Натрий` in the Russian chemistry archive. At
07:46:49 UTC, helper RSS was 270404 KiB against a startup-fixed allowance of
270155 KiB, with 398912 KiB system memory available and no helper swap.
The helper exited and the simple renderer was ready about one second later.
Other articles also hit shrinking startup allowances of 200–275 MiB.
These were resident-budget fallbacks; they were not system-reserve breaches.

The watcher now recalculates its allowance each sample from estimated capacity
`MemAvailable + helper RSS`, using the existing reserve, two-thirds fraction,
and 384 MiB handheld cap. This counts the running helper when estimating the
allowance rather than freezing a low limit at startup. Both increases and
reductions are allowed. Reserve, rapid-loss forecast, swap limit, resource
limits, and first-frame timeout are unchanged. Samples log both the current
and startup allowances. At the recorded sodium failure the recalculated
allowance would have been 313168 KiB, above its measured RSS; this comparison
is an offline calculation, not a replay of the GUI session.

A subsequent guarded headless run used the same sodium article, 1280 × 702,
30 seconds of commands with 800 ms pauses after each 60 commands. The pauses
exercise repeated full/half raster transitions. Startup allowance was
259515 KiB; the dynamic allowance reached 299411 KiB. Peak sampled RSS was
262236 KiB, which exceeded the old fixed startup allowance. Minimum sampled
MemAvailable was 378788 KiB (about 370 MiB); helper swap was zero. There were
no `servo.memory_pressure` or `servo.error` events. The run completed 660
commands and 171 frames, averaging 5.70 FPS including the pauses. Logs were
copied from `/tmp/zimlite-scroll-dynamic-budget` before installing the new app.

This demonstrates completion of the specific 30-second helper sequence. The
full GUI and longer scroll sessions still require device use; a real reserve,
process-cap, swap, timeout, or resource error still causes the intended fallback.
The paired helper did not change in this correction. The new ARM64 app was
installed with its predecessor retained as `zimlite.before-dynamic-budget`;
PC app artifacts were updated too.

## Visual glitches during raster transitions

The user's 17-second recording of sodium showed lateral jumps, doubled/halved
text and transient blank pages. The previous half/full transition called
`resize` and `set_hidpi_scale_factor` separately. This exposed intermediate CSS
viewport widths of 640 or 2560 instead of 1280, and readback could publish the
old compositor scene at the new physical size. The throughput-only probes above
did not inspect the transition images and therefore missed this defect.

`patches/servo-viewport-scale.patch` adds an atomic physical-size/HiDPI update
through WebView, Paint and the rendering context's Painter. Each affected
renderer sends only the final window-size notification. The helper suppresses
intermediate readbacks, waits for Servo's screenshot readiness (pipeline epochs
and completed WebRender frames), and publishes that completed screenshot. Input
is retained and merged while the transition is pending, then applied to the
new scroll/hit-test trees. The 5-second transition deadline and production
memory/resource watchdogs remain active. The client retains its previous frame.

Initial corrected device captures in `/tmp/zimlite-scroll-raster-fix` showed
consistent layout across five full/half transition images. These were copied
to the workstation and visually inspected. However, the transitions themselves
took 779–999 ms. With the old 300-ms idle threshold and 800-ms input pauses,
frequent transitions reduced throughput to 3.08 FPS over 12 seconds.

The idle threshold is now one second, so brief releases keep the reduced raster.
The final paired device helper is SHA-256
`61f828bf985bbab10cc80e1b58ec2ab9f3ca3c7fa0454677941ce229aaad85bf`;
it requires at most GLIBC 2.30. In `/tmp/zimlite-scroll-raster-final`, the same
12-second/800-ms pause sequence produced 66 frames (5.50 FPS including pauses),
267 commands, peak sampled helper RSS 256180 KiB, minimum MemAvailable 372952
KiB, zero helper swap and no memory-pressure/error events. There was one motion
transition, taking 1033 ms, and one idle restoration request; short pauses no
longer caused repeated full/half changes. The diagnostic settle period was
extended from two to three seconds because two seconds could expire before the
one-second idle delay plus full-frame completion. A subsequent eight-second
run (`/tmp/zimlite-scroll-raster-release`) captured all three initial/motion/idle
frames on the final helper. The completed full frame arrived 1009 ms after its
restoration request. The captured images were visually inspected; no scale or
lateral layout jumps appeared. Its motion sequence produced 36 frames (4.50 FPS
including pauses), demonstrating that timing varies between runs.

The desktop Linux helper was rebuilt with the same patch and threshold. Its
separate eight-second software probe used the local Russian top/mini archive,
not the chemistry/maxi archive installed on the device. Three initial/motion/
idle images were visually inspected and retained their layout; its performance
is not comparable with handheld software rendering or physical stick latency.
The ARM64 helper and source/patch license files were installed over ADB, with
its predecessor kept as `.before-raster-fix`. Desktop helper/package artifacts
were updated too. Go lint reports zero issues; no automated tests were run.

These captures cover the helper's transition images and guarded synthetic input.
They do not establish total GUI FPS, smoothness for every article, or the absence
of every glitch during physical stick use. The first motion transition still
holds the previous frame for about one second on this device.

## Persistent Speed / Quality setting

Implemented a saved `render_mode` setting with translations in all 13 UI
languages. Speed uses a fixed article viewport bounded by 640×480, with layout
at that viewport width and motion scale 1; Quality retains adaptive half/full
raster transitions with motion scale 0.5. Screen and viewport aspect ratios
remain matched. Fractional vertical/table scrolling survives SDL enlargement.

PortMaster defaults to Speed through the launcher; desktop defaults to Quality.
Explicit saved choices override the default. The existing device config had no
render mode, so the updated launcher selects Speed on its next normal launch.
The helper binary already supports both motion scales and did not need rebuilding.

ARM64 application built and deployed over ADB, with matching local/device SHA256:
`b4ba9d5bec3fc1f7870c271de192a364f9c6c286ec3de2daab47f78baea1f5a0`.
Previous application and launcher preserved as `.before-render-mode` backups.
Desktop binary and its Servo ZIP refreshed. Go lint reported zero issues;
`git diff --check` and Bash syntax checks passed. No runtime scroll measurements
or automated tests were performed for this setting change; responsiveness still
needs manual comparison of Speed and Quality on the handheld.
