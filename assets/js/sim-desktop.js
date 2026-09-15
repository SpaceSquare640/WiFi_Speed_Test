/*
 * The simulated run for the desktop preview.
 *
 * Same phases and same invented data as the other shells, presented the way a
 * window with room to spare can afford: the measurement on the left, what it
 * means on the right, and the history the tool keeps locally underneath.
 */

(() => {
  const PHASES = [
    { key: 'phasePing', metric: 'latency', ms: 900 },
    { key: 'phaseDownload', metric: 'download', ms: 1700 },
    { key: 'phaseUpload', metric: 'upload', ms: 1400 },
    { key: 'phaseLayers', metric: null, ms: 1000 },
  ];

  const ARC = 2 * Math.PI * 100 * 0.75;
  const HISTORY_LIMIT = 8;

  const el = {
    start: document.getElementById('start'),
    arc: document.getElementById('dial-arc'),
    value: document.getElementById('dial-value'),
    unit: document.getElementById('dial-unit'),
    phase: document.getElementById('dial-phase'),
    download: document.getElementById('sum-download'),
    upload: document.getElementById('sum-upload'),
    latency: document.getElementById('sum-latency'),
    grade: document.getElementById('sum-grade'),
    layers: document.getElementById('layers'),
    verdict: document.getElementById('verdict'),
    verdictTitle: document.getElementById('verdict-title'),
    verdictBody: document.getElementById('verdict-body'),
    results: document.getElementById('results'),
    historyBody: document.getElementById('history-body'),
    historyEmpty: document.getElementById('history-empty'),
    watch: document.getElementById('watch'),
    scenarios: document.querySelectorAll('.scenario-chip'),
    live: document.getElementById('live-region'),
  };

  const reduceMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;

  let running = false;
  // The phase currently on screen, so that switching language mid-run can
  // repaint it instead of leaving one label in the previous language until the
  // next phase happens to overwrite it.
  let phaseKey = 'idle';
  let forced = null;
  let last = null;
  let history = [];
  let watchTimer = null;

  /* ---------- dial ---------- */

  function setArc(fraction, variant) {
    el.arc.style.strokeDasharray = `${ARC} ${ARC}`;
    el.arc.style.strokeDashoffset = String(ARC * (1 - Math.max(0, Math.min(1, fraction))));
    el.arc.className.baseVal = `dial__arc${variant ? ` dial__arc--${variant}` : ''}`;
  }

  function setReadout(value, unit) {
    el.value.textContent = value;
    el.unit.textContent = unit;
  }

  // As in the app preview: the animation frame gives the sweep, the timer
  // guarantees the phase ends even when the tab is hidden and frames stop.
  function animate(phase, target, unit, scale) {
    return new Promise((resolve) => {
      phaseKey = phase.key;
      el.phase.textContent = I18n.t(phaseKey);

      let settled = false;
      const finish = () => {
        if (settled) return;
        settled = true;
        setReadout(target.toFixed(unit === 'ms' ? 0 : 1), unit);
        setArc(target / scale, phase.metric);
        resolve();
      };

      if (reduceMotion) return void finish();

      const guard = setTimeout(finish, phase.ms + 400);
      const start = performance.now();

      const tick = (now) => {
        if (settled) return;
        const progress = Math.min(1, (now - start) / phase.ms);
        const shown = target * (1 - (1 - progress) ** 3);
        setReadout(shown.toFixed(unit === 'ms' ? 0 : 1), unit);
        setArc(shown / scale, phase.metric);
        if (progress < 1) requestAnimationFrame(tick);
        else {
          clearTimeout(guard);
          finish();
        }
      };
      requestAnimationFrame(tick);
    });
  }

  const wait = (ms) => new Promise((r) => setTimeout(r, reduceMotion ? 0 : ms));

  /* ---------- rendering ---------- */

  const LAYER_KEYS = {
    gateway: 'layerGateway',
    regional: 'layerRegional',
    international: 'layerInternational',
  };

  function renderLayers(report) {
    el.layers.replaceChildren();

    report.layers.forEach((layer) => {
      const row = document.createElement('li');
      row.className = `layer layer--${layer.status}`;

      const name = document.createElement('span');
      name.className = 'layer__name';
      name.textContent = I18n.t(LAYER_KEYS[layer.kind]);

      const target = document.createElement('span');
      target.className = 'layer__target numeric';
      target.textContent = layer.target;

      const value = document.createElement('span');
      value.className = 'layer__value numeric';
      value.textContent = `${layer.latency.toFixed(2)} ms`;

      const detail = document.createElement('span');
      detail.className = 'layer__detail';
      detail.textContent =
        `${I18n.t('jitter')} ${layer.jitter.toFixed(2)} ms · ${I18n.t('loss')} ${layer.loss}%`;

      row.append(name, target, value, detail);
      el.layers.append(row);
    });
  }

  function renderHistory() {
    el.historyBody.replaceChildren();
    el.historyEmpty.hidden = history.length > 0;

    // Newest first: the run just finished is what the eye goes to.
    [...history].reverse().forEach((entry, index) => {
      const row = document.createElement('tr');
      if (index === 0) row.dataset.latest = 'true';

      const cells = [
        [entry.time, ''],
        [entry.download.toFixed(1), 'down numeric'],
        [entry.upload.toFixed(1), 'up numeric'],
        [entry.latency.toFixed(0), 'ping numeric'],
      ];
      cells.forEach(([text, cls]) => {
        const td = document.createElement('td');
        td.className = cls;
        td.textContent = text;
        row.append(td);
      });
      el.historyBody.append(row);
    });
  }

  function render(report) {
    el.download.textContent = report.download.toFixed(1);
    el.upload.textContent = report.upload.toFixed(1);
    el.latency.textContent = report.layers[2].latency.toFixed(0);

    const grade = FakeData.grade(report.download);
    el.grade.textContent = I18n.t(`grade${grade[0].toUpperCase()}${grade.slice(1)}`);

    renderLayers(report);
    el.verdict.className = `verdict verdict--${report.severity}`;
    el.verdictTitle.textContent = I18n.t(report.verdictKey);
    el.verdictBody.textContent = I18n.t(`${report.verdictKey}Body`);
    el.results.hidden = false;

    el.live.textContent = `${I18n.t(report.verdictKey)}. ${I18n.t('download')} ${report.download.toFixed(1)} Mbps.`;
  }

  function record(report) {
    const now = new Date();
    const pad = (n) => String(n).padStart(2, '0');
    history.push({
      time: `${pad(now.getHours())}:${pad(now.getMinutes())}:${pad(now.getSeconds())}`,
      download: report.download,
      upload: report.upload,
      latency: report.layers[2].latency,
    });
    // The real tool trims its history file; the preview trims the table.
    if (history.length > HISTORY_LIMIT) history = history.slice(-HISTORY_LIMIT);
    renderHistory();
  }

  /* ---------- run ---------- */

  async function run() {
    if (running) return;
    running = true;

    el.start.disabled = true;
    el.start.textContent = I18n.t('running');
    el.results.hidden = true;

    const report = FakeData.report(forced);
    last = report;

    await animate(PHASES[0], report.layers[2].latency, 'ms', 300);
    await wait(150);
    await animate(PHASES[1], report.download, 'Mbps', 150);
    await wait(150);
    await animate(PHASES[2], report.upload, 'Mbps', 60);
    await wait(150);

    phaseKey = PHASES[3].key;
    el.phase.textContent = I18n.t(phaseKey);
    await wait(PHASES[3].ms);

    setReadout(report.download.toFixed(1), 'Mbps');
    setArc(report.download / 150, 'download');
    phaseKey = 'phaseDone';
    el.phase.textContent = I18n.t(phaseKey);

    render(report);
    record(report);

    el.start.disabled = false;
    el.start.textContent = I18n.t('again');
    running = false;

    // Continuous monitoring: the mode the previous generation of this tool ran
    // in, kept because a connection that fails intermittently is invisible to a
    // single pass.
    if (el.watch.checked) {
      clearTimeout(watchTimer);
      watchTimer = setTimeout(run, reduceMotion ? 1500 : 6000);
    }
  }

  el.start.addEventListener('click', run);

  el.watch.addEventListener('change', () => {
    if (!el.watch.checked) {
      clearTimeout(watchTimer);
      watchTimer = null;
    } else if (!running) {
      watchTimer = setTimeout(run, 1200);
    }
  });

  el.scenarios.forEach((chip) => {
    chip.addEventListener('click', () => {
      forced = chip.dataset.scenario === 'random' ? null : chip.dataset.scenario;
      el.scenarios.forEach((other) => other.setAttribute('aria-pressed', String(other === chip)));
    });
  });

  document.addEventListener('languagechange', () => {
    if (!running) el.start.textContent = I18n.t(last ? 'again' : 'start');
    el.phase.textContent = I18n.t(phaseKey);
    if (last) render(last);
  });

  setArc(0, null);
  setReadout('--', '');
  el.phase.textContent = I18n.t('idle');
  renderHistory();
})();
