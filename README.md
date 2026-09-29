# plugin-wl

Wayland desktop automation for OpenCharly — the `wl:` check verb.

The `wl:` verb is the unified desktop-automation verb for wlroots compositors
(sway, labwc), **Hyprland**, and **KWin (KDE Plasma)**. It provides screenshots,
input (click, type, key combos, scroll, drag), window management, clipboard,
resolution control, accessibility introspection (AT-SPI2), and window geometry
queries. It is served **out-of-process** — charly's loader fetches this repo,
host-builds the provider binary, and serves it over go-plugin gRPC, so the
Wayland/sway driver lives here, out of charly's core check surface. It is
EXEC-based: the host attaches its live `DeployExecutor` over the reverse channel
and the plugin dials back through the SDK to run the venue's compositor tools.

It is **compositor-aware** (`detectCompositor` picks the backend per method):

- On **wlroots** (sway/labwc): `wlrctl` (pointer + toplevel), `wlr-randr`
  (resolution), `grim`/`pixelflux` (screenshot).
- On **KWin**: window management via `kdotool` (KWin scripting) + screenshot via
  `pixelflux`; `status` reports `compositor: kwin`. The wlroots protocols KWin
  does not implement (keyboard, clipboard, pointer, resolution) fail fast with a
  clear "unsupported on KWin" error instead of hanging.
- On **Hyprland**: window management and resolution route through `hyprctl`
  (`wl: status` reports `window=hyprctl resolution=hyprctl`); pointer, keyboard
  and clipboard keep their wlroots backends.

The `hypr-*` family covers Hyprland's IPC the way `sway-*` covers sway's;
`hypr-layers` correlates layer-shell surfaces against monitor geometry and emits
a tab-separated record per surface ending in `onscreen`/`offscreen`/`unknown`.

## What it provides

| Capability | Surface |
|---|---|
| `verb:wl` | the `wl:` check verb — screenshots, input, window management, clipboard, resolution, AT-SPI2, geometry, the nested `sway-*` / `hypr-*` / `overlay-*` methods |

## How to use it

Compose the plugin candy in a desktop bed (e.g. `sway-browser-vnc`,
`selkies-desktop`, `selkies-kde-desktop`):

```yaml
- '@github.com/opencharly/plugin-wl/candy/plugin-wl:<tag>'
```

Then author the verb in a plan (`context: [deploy]`):

```yaml
- check: a non-empty desktop screenshot is captured
  context: [deploy]
  wl:
    method: screenshot
    artifact: /tmp/desktop.png
    artifact_min_bytes: 10000
```

`wl: screenshot` writes a PNG to `artifact:`; the artifact validators
(`artifact_min_bytes:`, `artifact_min_dimensions:`, `artifact_not_uniform:`,
`artifact_contains_text:` for OCR, `artifact_min_cast_events:`) turn the file into
an assertion. `artifact_contains_text:` needs `tesseract` + a language pack **on
the host**, not in the venue.

## Layout

- `candy/plugin-wl/` — the plugin module: `methods.go` (the method surface),
  `compositor.go` (per-compositor routing), `hyprland_layers.go`, `atspi.go`,
  `provider.go` / `plugin.go`, `schema/wl.cue` (the self-contained `#WlInput`),
  `params/cue_types_gen.go`, `cmd/serve/main.go`.
- `charly.yml` — the root project manifest (`discover: candy` only); the
  `wl-skill` and `wl-overlay-skill` `skill:` entities live in the candy manifest
  `candy/plugin-wl/charly.yml`.
- `.github/workflows/tag-on-merge.yml` — CalVer tag + `CHANGELOG/` on merge.

## Related

- Owning skill: `/charly-check:wl` — the `wl:` Wayland desktop-automation verb,
  authored in this candy's `skill:` entity.
- `/charly-check:wl-overlay` — the `wl: overlay-*` methods, authored in this
  candy's second `skill:` entity.
- `/charly-check:vnc` / `/charly-check:cdp` — sibling verbs for remote / DOM-level
  interaction.
- `/charly-internals:plugin` — the out-of-process plugin model.
