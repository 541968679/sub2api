# Frontend pin wiring

Long-term exemption (`pinned`) is a sixth writable admission state. It is not `resumed` (豁免期, 15/30m).

## GET

- Show 长期豁免 only when `pinned: true`, or when `admission` / `state` is `pinned`.
- Do not invent pin from an expired 豁免期 (`resume_users` / `resume_watching_users` in the past, or `qualityHint === 'resumed'`).
- Account hard-close still wins: chip can be `stopped` while the switcher checkmarks 长期豁免 (`:pinned="true"`).

## Write

- Switcher sends `state: 'pinned'` explicitly.
- Omitted `state` stays 豁免期 (`resumed`). `resumeSmartSchedule` default is unchanged.
- Pin does not write the 15/30m resume grace windows.

## Occupancy

- Pinned uses the full member cap, same as selectable.
- Never use `probe_cap` for pinned.
- Uncapped pinned still displays 999, same as selectable today.

## Priority (`resolvePoolAdmission`)

`stopped` → `paused` → `pinned` → `cooling` → `probing` → `pair_full` → quality hints → `selectable`.

Pin stays visible at pair-full and ignores leftover cooldown / probe marks.

## Switcher

暂停 / 冷却 / 考察 / 调度 / 豁免期 / 长期豁免

Hint: 仅手选；满额可调度；不因配对质量冷却；直到再手选下一态。账号硬关闭仍可能整号停。
