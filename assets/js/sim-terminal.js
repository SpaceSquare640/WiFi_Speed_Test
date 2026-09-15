/*
 * The terminal preview.
 *
 * The layout below is not an impression of a command line tool; it reproduces
 * what `wifitest` actually prints, down to the column widths and the wording.
 * A visitor who installs the tool after seeing this page should recognise the
 * screen in front of them. Only the figures are invented.
 *
 * Mirrored from internal/cli/output/human.go and json.go.
 */

(() => {
  const LABEL_WIDTH = 12; // human.go: labelWidth
  const PROMPT = '$ ';

  const el = {
    body: document.getElementById('terminal-body'),
    chips: document.querySelectorAll('.command-chip'),
    skip: document.getElementById('skip'),
    replay: document.getElementById('replay'),
    exit: document.getElementById('exit-line'),
  };

  const reduceMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;

  let timer = null;
  let segments = [];
  let totalChars = 0;
  let shown = 0;
  let currentCommand = 'wifitest';

  /* ---------- helpers ---------- */

  const seg = (text, cls) => ({ text, cls: cls || '' });
  const pad = (text, width) => text + ' '.repeat(Math.max(0, width - text.length));

  function field(label, valueSegments) {
    return [seg('  ' + pad(label + ':', LABEL_WIDTH) + ' '), ...valueSegments, seg('\n')];
  }

  function timestamp() {
    const d = new Date();
    const p = (n) => String(n).padStart(2, '0');
    return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ` +
      `${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`;
  }

  const LAYER_NAMES = {
    gateway: 'Local gateway',
    regional: 'Regional egress',
    international: 'International',
  };

  const PORTS = { gateway: '80', regional: '53', international: '53' };

  /* ---------- output builders ---------- */

  /**
   * The human-readable report.
   *
   * Column widths are measured from the content and padded, exactly as the Go
   * renderer does, rather than being hard-coded here. Hard-coding them would
   * drift the moment a value grew a digit.
   */
  function humanReport(report) {
    const out = [
      seg('WiFi Speed Test', 't-bold'),
      seg('  ' + timestamp(), 't-dim'),
      seg('\n'),
      seg('host: preview-host (linux/amd64)', 't-dim'),
      seg('\n\n'),
    ];

    out.push(...field('Download', [seg(report.download.toFixed(2) + ' Mbps', 't-download')]));
    out.push(...field('Upload', [seg(report.upload.toFixed(2) + ' Mbps', 't-upload')]));
    out.push(...field('Latency', [seg(report.latency.toFixed(1) + ' ms', 't-latency')]));

    const gradeId = FakeData.grade(report.download);
    const gradeText = { excellent: 'excellent', veryGood: 'very good', good: 'good', fair: 'fair', slow: 'slow' }[gradeId];
    const gradeClass = gradeId === 'slow' ? 't-bad' : gradeId === 'fair' || gradeId === 'good' ? 't-latency' : 't-download';
    out.push(...field('Grade', [seg(gradeText, gradeClass)]));
    out.push(seg('\n'));

    out.push(seg('  Layered diagnostics', 't-bold'), seg('\n'));

    const rows = [['LAYER', 'TARGET', 'PORT', 'LATENCY', 'JITTER', 'LOSS']];
    report.layers.forEach((layer) => {
      rows.push([
        LAYER_NAMES[layer.kind],
        layer.target,
        PORTS[layer.kind],
        layer.latency.toFixed(2) + ' ms',
        layer.jitter.toFixed(2) + ' ms',
        layer.loss.toFixed(0) + ' %',
      ]);
    });

    const widths = rows[0].map((_, i) => Math.max(...rows.map((r) => r[i].length)));

    rows.forEach((row, rowIndex) => {
      out.push(seg('  '));
      row.forEach((cell, i) => {
        const gap = i < row.length - 1 ? ' '.repeat(widths[i] - cell.length + 2) : '';
        if (rowIndex === 0) {
          out.push(seg(cell + gap, 't-dim'));
          return;
        }
        // Latency is amber and a non-zero loss is red, matching the tool.
        let cls = '';
        if (i === 3) cls = 't-latency';
        if (i === 5 && parseFloat(cell) > 0) cls = 't-bad';
        out.push(seg(cell, cls), seg(gap));
      });
      out.push(seg('\n'));
    });
    out.push(seg('\n'));

    out.push(...field('DNS', [
      seg(report.dns.toFixed(2) + ' ms '),
      seg('(example.com)', 't-dim'),
    ]));
    out.push(seg('\n'));

    return out;
  }

  /** The machine-readable report, as the JSON writer emits it. */
  function jsonReport(report) {
    const wire = {
      schema_version: 1,
      timestamp: new Date().toISOString(),
      host: { name: 'preview-host', os: 'linux', arch: 'amd64' },
      dns: { host: 'example.com', ok: true, resolve_ms: +report.dns.toFixed(3) },
      layers: report.layers.map((layer) => ({
        kind: layer.kind === 'regional' ? 'regional_egress' : layer.kind,
        target: layer.target,
        user_defined: false,
        ok: true,
        latency_ms: +layer.latency.toFixed(3),
        jitter_ms: +layer.jitter.toFixed(3),
        loss_pct: +layer.loss.toFixed(3),
        probe_port: Number(PORTS[layer.kind]),
      })),
      download: { ok: true, mbps: +report.download.toFixed(3) },
      upload: { ok: true, mbps: +report.upload.toFixed(3) },
      latency: { ok: true, latency_ms: +report.latency.toFixed(3), jitter_ms: +report.jitter.toFixed(3), loss_pct: 0 },
      grade: FakeData.grade(report.download).replace('veryGood', 'very_good'),
      resolver_is_public: false,
    };

    // Coloured the way a JSON viewer would: keys, strings and numbers apart.
    // The grade and the layer kinds are emitted as identifiers rather than as
    // display text, which is the property downstream parsers depend on.
    const text = JSON.stringify(wire, null, 2);
    const out = [];
    text.split('\n').forEach((line) => {
      const match = line.match(/^(\s*)"([^"]+)":\s?(.*)$/);
      if (!match) {
        out.push(seg(line + '\n'));
        return;
      }
      const [, indent, key, rest] = match;
      out.push(seg(indent), seg(`"${key}"`, 't-key'), seg(': '));
      if (rest.startsWith('"')) out.push(seg(rest, 't-string'));
      else if (/^[\d-]/.test(rest)) out.push(seg(rest, 't-number'));
      else out.push(seg(rest));
      out.push(seg('\n'));
    });
    return out;
  }

  /** The Chinese interface. Column widths differ because CJK takes two cells. */
  function humanReportChinese(report) {
    const width = (s) => [...s].reduce((w, c) => w + (c.charCodeAt(0) > 0x2e7f ? 2 : 1), 0);
    const padWide = (s, w) => s + ' '.repeat(Math.max(0, w - width(s)));

    const names = { gateway: '本地網關', regional: '區域出口', international: '國際節點' };
    const out = [
      seg('WiFi 測速工具', 't-bold'),
      seg('  ' + timestamp(), 't-dim'),
      seg('\n'),
      seg('主機: preview-host (linux/amd64)', 't-dim'),
      seg('\n\n'),
    ];

    const fieldW = (label, value, cls) => {
      out.push(seg('  ' + padWide(label + ':', LABEL_WIDTH) + ' '), seg(value, cls), seg('\n'));
    };
    fieldW('下載', report.download.toFixed(2) + ' Mbps', 't-download');
    fieldW('上傳', report.upload.toFixed(2) + ' Mbps', 't-upload');
    fieldW('延遲', report.latency.toFixed(1) + ' ms', 't-latency');
    fieldW('評級', '良好', 't-latency');
    out.push(seg('\n'));

    out.push(seg('  分層診斷', 't-bold'), seg('\n'));
    const rows = [['層級', '目標', '埠', '延遲', '抖動', '遺失']];
    report.layers.forEach((layer) => {
      rows.push([
        names[layer.kind],
        layer.target,
        PORTS[layer.kind],
        layer.latency.toFixed(2) + ' ms',
        layer.jitter.toFixed(2) + ' ms',
        layer.loss.toFixed(0) + ' %',
      ]);
    });
    const widths = rows[0].map((_, i) => Math.max(...rows.map((r) => width(r[i]))));
    rows.forEach((row, rowIndex) => {
      out.push(seg('  '));
      row.forEach((cell, i) => {
        const gap = i < row.length - 1 ? ' '.repeat(widths[i] - width(cell) + 2) : '';
        if (rowIndex === 0) out.push(seg(cell + gap, 't-dim'));
        else out.push(seg(cell, i === 3 ? 't-latency' : ''), seg(gap));
      });
      out.push(seg('\n'));
    });
    out.push(seg('\n'));
    out.push(seg('  ' + padWide('DNS 解析:', LABEL_WIDTH) + ' ' + report.dns.toFixed(2) + ' ms '), seg('(example.com)', 't-dim'), seg('\n\n'));
    return out;
  }

  const HELP = `wifitest 0.1.0-dev - network diagnostics that say which segment is at fault

Usage:
  wifitest [flags]

Modes:
  --watch                 repeat until stopped
  --interval <duration>   delay between passes (default 30s, watch only)
  --count <n>             stop after n passes (watch only)

Output:
  --json                  machine-readable output, one object per pass
  --no-color              disable colour
  --lang <en|zh-TW>       interface language

Scope:
  --quick                 sample a single endpoint
  --no-download           skip the download measurement
  --no-upload             skip the upload measurement
  --no-layers             skip the layered diagnostics

Targets:
  --endpoint <url>        throughput endpoint; repeat to supply several
                          (replaces the built-in list rather than adding to it)
  --layer1 <host>         local gateway target
  --layer2 <host>         regional egress target
  --layer3 <host>         international target

Measurement:
  --servers <n>           endpoints contributing to the mean
  --timeout <duration>    per-measurement timeout
  --retries <n>           retry attempts per measurement
  --icmp                  probe with ICMP rather than TCP (may need elevation)

Files:
  --config <path>         configuration file
  --history <path>        history file
  --no-history            do not record this run
  --webhook <url>         webhook to notify; prefer WIFITEST_WEBHOOK_URL,
                          since a command line lands in shell history

Exit codes:
  0 success   1 partial   2 no network   3 no endpoint   4 config   130 interrupted
`;

  /* ---------- command table ---------- */

  const COMMANDS = {
    wifitest: { exit: 0, build: () => humanReport(FakeData.report('healthy')) },

    'wifitest --json': { exit: 0, build: () => jsonReport(FakeData.report('healthy')) },

    'wifitest --lang zh-TW': { exit: 0, build: () => humanReportChinese(FakeData.report('healthy')) },

    'wifitest --watch --interval 30s': {
      exit: 130,
      build: () => {
        const out = [];
        out.push(...humanReport(FakeData.report('healthy')));
        out.push(...humanReport(FakeData.report('healthy')));
        out.push(seg('^C', 't-dim'), seg('\n'));
        return out;
      },
    },

    // The one run that shows the tool doing its actual job: saying which
    // segment broke rather than only that something did.
    'wifitest --layer1 192.0.2.1': {
      exit: 1,
      build: () => {
        const report = FakeData.report('localFault');
        const out = humanReport(report);
        out.push(seg('  Problems', 't-bold'), seg('\n'));
        out.push(seg('  - gateway: high jitter and packet loss on the first hop', 't-bad'), seg('\n\n'));
        return out;
      },
    },

    'wifitest --help': { exit: 0, build: () => [seg(HELP)] },
  };

  /* ---------- typing ---------- */

  function render(count) {
    const frag = document.createDocumentFragment();
    let remaining = count;

    for (const s of segments) {
      if (remaining <= 0) break;
      const slice = s.text.slice(0, remaining);
      remaining -= slice.length;

      if (s.cls) {
        const span = document.createElement('span');
        span.className = s.cls;
        span.textContent = slice;
        frag.append(span);
      } else {
        frag.append(document.createTextNode(slice));
      }
    }

    el.body.replaceChildren(frag);
    if (count < totalChars) {
      const caret = document.createElement('span');
      caret.className = 'caret';
      el.body.append(caret);
    }
    el.body.scrollTop = el.body.scrollHeight;
  }

  function stop() {
    if (timer) {
      clearInterval(timer);
      timer = null;
    }
  }

  function finish() {
    stop();
    shown = totalChars;
    render(shown);
    el.skip.disabled = true;
    el.exit.hidden = false;
  }

  function play(command) {
    stop();
    currentCommand = command;
    const spec = COMMANDS[command];

    segments = [seg(PROMPT, 't-prompt'), seg(command, 't-command'), seg('\n\n'), ...spec.build()];
    totalChars = segments.reduce((n, s) => n + s.text.length, 0);
    shown = 0;

    el.exit.hidden = true;
    el.exit.innerHTML = '';
    const code = spec.exit;
    const label = document.createElement('span');
    label.textContent = '$ echo $? → ';
    const value = document.createElement('strong');
    value.textContent = String(code);
    value.className = code === 0 ? 't-download' : 't-latency';
    el.exit.append(label, value);

    if (reduceMotion) {
      finish();
      return;
    }

    el.skip.disabled = false;
    // Characters per tick rather than one at a time: a full report is over two
    // thousand characters, and a visitor will not wait that long to read it.
    timer = setInterval(() => {
      shown = Math.min(totalChars, shown + 14);
      render(shown);
      if (shown >= totalChars) finish();
    }, 16);
  }

  /* ---------- wiring ---------- */

  el.chips.forEach((chip) => {
    chip.addEventListener('click', () => {
      el.chips.forEach((other) => other.setAttribute('aria-pressed', String(other === chip)));
      play(chip.dataset.command);
    });
  });

  el.skip.addEventListener('click', finish);
  el.replay.addEventListener('click', () => play(currentCommand));

  play('wifitest');
})();
