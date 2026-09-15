/*
 * The simulated test run for the Android app preview.
 *
 * It animates through the same phases the real engine performs, in the same
 * order, so that a visitor learns what the product does rather than watching an
 * arbitrary loading bar. None of it measures anything.
 */

(() => {
  const PHASES = [
    { key: 'phasePing', metric: 'latency', ms: 900 },
    { key: 'phaseDownload', metric: 'download', ms: 1600 },
    { key: 'phaseUpload', metric: 'upload', ms: 1300 },
    { key: 'phaseLayers', metric: 'layers', ms: 1100 },
  ];

  // The dial sweeps 270 degrees, leaving a gap at the bottom so the ends of the
  // arc are distinguishable from each other.
  const ARC_LENGTH = 2 * Math.PI * 100 * 0.75;

  const el = {
    start: document.getElementById('start'),
    arc: document.getElementById('gauge-arc'),
    value: document.getElementById('gauge-value'),
    unit: document.getElementById('gauge-unit'),
    phase: document.getElementById('gauge-phase'),
    grade: document.getElementById('gauge-grade'),
    download: document.getElementById('metric-download'),
    upload: document.getElementById('metric-upload'),
    latency: document.getElementById('metric-latency'),
    layers: document.getElementById('layers'),
    verdict: document.getElementById('verdict'),
    verdictTitle: document.getElementById('verdict-title'),
    verdictBody: document.getElementById('verdict-body'),
    results: document.getElementById('results'),
    scenarios: document.querySelectorAll('.scenario-chip'),
    live: document.getElementById('live-region'),
  };

  let running = false;
  // The phase currently on screen, so that switching language mid-run can
  // repaint it rather than leaving one label in the previous language.
  let phaseKey = 'idle';
  let forcedScenario = null;
  let lastReport = null;

  const reduceMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;

  function setArc(fraction, variant) {
    const clamped = Math.max(0, Math.min(1, fraction));
    el.arc.style.strokeDasharray = `${ARC_LENGTH} ${ARC_LENGTH}`;
    el.arc.style.strokeDashoffset = String(ARC_LENGTH * (1 - clamped));
    el.arc.className.baseVal = `gauge__arc${variant ? ` gauge__arc--${variant}` : ''}`;
  }

  function setReadout(value, unit) {
    el.value.textContent = value;
    el.unit.textContent = unit;
  }

  /**
   * Runs one phase, easing the displayed figure up to its final value.
   *
   * Two mechanisms drive it. requestAnimationFrame gives a smooth sweep, but a
   * browser suspends it entirely while the tab is hidden: a visitor who
   * switches away mid-test would return to a frozen dial and a button locked
   * forever. So a timer guarantees the phase finishes regardless, and whichever
   * arrives first settles it.
   */
  function animatePhase(phase, target, unit, scale) {
    return new Promise((resolve) => {
      phaseKey = phase.key;
      el.phase.textContent = I18n.t(phaseKey);

      let settled = false;
      const finish = () => {
        if (settled) return;
        settled = true;
        setReadout(target.toFixed(unit === 'ms' ? 0 : 1), unit);
        setArc(Math.min(1, target / scale), phase.metric === 'layers' ? null : phase.metric);
        resolve();
      };

      if (reduceMotion) {
        finish();
        return;
      }

      const guard = setTimeout(finish, phase.ms + 400);

      const start = performance.now();
      const tick = (now) => {
        if (settled) return;
        const progress = Math.min(1, (now - start) / phase.ms);
        // Ease out: the figure races up and settles, the way a real
        // measurement converges once enough samples are in.
        const eased = 1 - (1 - progress) ** 3;
        const shown = target * eased;

        setReadout(shown.toFixed(unit === 'ms' ? 0 : 1), unit);
        setArc(Math.min(1, shown / scale), phase.metric === 'layers' ? null : phase.metric);

        if (progress < 1) {
          requestAnimationFrame(tick);
        } else {
          clearTimeout(guard);
          finish();
        }
      };
      requestAnimationFrame(tick);
    });
  }

  function wait(ms) {
    return new Promise((resolve) => setTimeout(resolve, reduceMotion ? 0 : ms));
  }

  function renderLayers(report) {
    el.layers.innerHTML = '';

    report.layers.forEach((layer) => {
      const row = document.createElement('li');
      row.className = `layer layer--${layer.status}`;

      const nameKey = {
        gateway: 'layerGateway',
        regional: 'layerRegional',
        international: 'layerInternational',
      }[layer.kind];

      const name = document.createElement('span');
      name.className = 'layer__name';
      name.dataset.i18n = nameKey;
      name.textContent = I18n.t(nameKey);

      const target = document.createElement('span');
      target.className = 'layer__target numeric';
      target.textContent = layer.target;

      const value = document.createElement('span');
      value.className = 'layer__value numeric';
      value.textContent = `${layer.latency.toFixed(2)} ms`;

      const detail = document.createElement('span');
      detail.className = 'layer__detail';
      // Status is never carried by colour alone; the words repeat it.
      detail.textContent =
        `${I18n.t('jitter')} ${layer.jitter.toFixed(2)} ms · ` +
        `${I18n.t('loss')} ${layer.loss}%`;

      row.append(name, target, value, detail);
      el.layers.append(row);
    });
  }

  function renderVerdict(report) {
    el.verdict.className = `verdict verdict--${report.severity}`;
    el.verdictTitle.textContent = I18n.t(report.verdictKey);
    el.verdictBody.textContent = I18n.t(`${report.verdictKey}Body`);
  }

  function renderResults(report) {
    el.download.textContent = report.download.toFixed(1);
    el.upload.textContent = report.upload.toFixed(1);
    el.latency.textContent = report.layers[2].latency.toFixed(0);

    const gradeKey = `grade${FakeData.grade(report.download).replace(/^./, (c) => c.toUpperCase())}`;
    el.grade.textContent = `${I18n.t('grade')}: ${I18n.t(gradeKey)}`;

    renderLayers(report);
    renderVerdict(report);
    el.results.hidden = false;

    // Announce the outcome for anyone who cannot see the dial.
    el.live.textContent = `${I18n.t(report.verdictKey)}. ${I18n.t('download')} ${report.download.toFixed(1)} Mbps.`;
  }

  async function run() {
    if (running) return;
    running = true;

    el.start.disabled = true;
    el.start.textContent = I18n.t('running');
    el.results.hidden = true;
    el.grade.textContent = '';

    const report = FakeData.report(forcedScenario);
    lastReport = report;

    await animatePhase(PHASES[0], report.layers[2].latency, 'ms', 300);
    await wait(180);
    await animatePhase(PHASES[1], report.download, 'Mbps', 150);
    await wait(180);
    await animatePhase(PHASES[2], report.upload, 'Mbps', 60);
    await wait(180);

    phaseKey = PHASES[3].key;
    el.phase.textContent = I18n.t(phaseKey);
    await wait(PHASES[3].ms);

    // The dial ends on the download figure, which is the number people mean
    // when they ask how fast their connection is.
    setReadout(report.download.toFixed(1), 'Mbps');
    setArc(Math.min(1, report.download / 150), 'download');
    phaseKey = 'phaseDone';
    el.phase.textContent = I18n.t(phaseKey);

    renderResults(report);

    el.start.disabled = false;
    el.start.textContent = I18n.t('again');
    running = false;
  }

  el.start.addEventListener('click', run);

  el.scenarios.forEach((chip) => {
    chip.addEventListener('click', () => {
      const id = chip.dataset.scenario;
      forcedScenario = id === 'random' ? null : id;
      el.scenarios.forEach((other) => {
        other.setAttribute('aria-pressed', String(other === chip));
      });
    });
  });

  // Re-render the last result in the new language rather than leaving a mix of
  // both on screen.
  document.addEventListener('languagechange', () => {
    if (!running) el.start.textContent = I18n.t(lastReport ? 'again' : 'start');
    el.phase.textContent = I18n.t(phaseKey);
    if (lastReport) renderResults(lastReport);
  });

  // Idle state.
  setArc(0, null);
  setReadout('--', '');
  el.phase.textContent = I18n.t('idle');
})();
