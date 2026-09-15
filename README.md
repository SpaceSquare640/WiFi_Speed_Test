# WiFi Speed Test — preview

A preview of the [WiFi Speed Test](https://github.com/SpaceSquare640/WiFi_Speed_Test)
interface. It is a static site that measures nothing.

> **Everything on this site is simulated.** No measurement is taken, no request
> leaves the page, and the addresses shown are drawn from the ranges the IETF
> reserves for documentation (RFC 5737), so none of them routes anywhere.

## What it shows

The tool does not only report how fast a connection is; it reports which segment
of the path is responsible when something is wrong. Latency, jitter and loss are
measured at three points:

| Layer | What a fault there means |
| --- | --- |
| Local gateway | The problem is inside your own network or Wi-Fi |
| Regional egress | The problem is inside your internet provider |
| International | The problem is on the long-haul link |

The preview lets you force each of those situations and see what the interface
says about them, which is the part a single speed figure cannot convey.

## Pages

| Page | Shell |
| --- | --- |
| `index.html` | Overview |
| `terminal.html` | The command line tool. Layout, columns and wording reproduce the real output exactly; only the figures are invented |
| `mobile.html` | The Android app |

More shells are added to this same folder as they are designed. Every page
shares one set of design tokens and one simulated-data generator, which is what
keeps them recognisable as one product.

## Running it

No build step, no dependencies. Open `index.html` in a browser, or serve the
folder:

```
python -m http.server 8000
```

Then visit `http://localhost:8000`.

## Language

English by default, with a Traditional Chinese toggle in the header. Every
string lives in `assets/js/i18n.js`.

## Licence

MIT. Copyright (c) 2026 SpaceSquare.
