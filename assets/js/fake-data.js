/*
 * Invented measurements for the preview.
 *
 * Nothing here is measured and nothing here is real. No address in this file
 * routes anywhere: the three layer targets are drawn from the ranges the IETF
 * reserves for documentation (RFC 5737), precisely so that an example cannot be
 * mistaken for a live host. The page never reads the visitor's own network, so
 * it has no real address to leak even by accident.
 *
 * One rule governs every number below: the three layers must grow further away
 * in that order. A gateway that answered slower than an overseas host would be
 * physically absurd, and a reader who notices stops believing the rest of the
 * page.
 */

const FakeData = (() => {
  const TARGETS = {
    gateway: '192.0.2.1', // RFC 5737 TEST-NET-1
    regional: '198.51.100.53', // RFC 5737 TEST-NET-2
    international: '203.0.113.53', // RFC 5737 TEST-NET-3
  };

  /** Uniform random within a range, rounded to the given precision. */
  function between(min, max, decimals = 2) {
    const value = min + Math.random() * (max - min);
    const factor = 10 ** decimals;
    return Math.round(value * factor) / factor;
  }

  /*
   * The three situations the tool exists to tell apart. A preview that only
   * ever shows a healthy connection demonstrates nothing: the point of layered
   * diagnostics is what it says when something is wrong.
   */
  const SCENARIOS = {
    healthy: {
      id: 'healthy',
      weight: 0.5,
      verdict: 'verdictHealthy',
      severity: 'ok',
      layers: {
        gateway: { latency: [1, 3], jitter: [0.1, 0.6], loss: 0, status: 'ok' },
        regional: { latency: [15, 45], jitter: [0.2, 2], loss: 0, status: 'ok' },
        international: { latency: [45, 90], jitter: [0.3, 3], loss: 0, status: 'ok' },
      },
      throughput: { download: [80, 120], upload: [20, 40] },
    },

    localFault: {
      id: 'localFault',
      weight: 0.25,
      verdict: 'verdictLocal',
      severity: 'bad',
      // The fault is inside the home: the first hop already stumbles, and
      // everything beyond it inherits the damage.
      layers: {
        gateway: { latency: [45, 120], jitter: [12, 40], loss: [4, 18], status: 'bad' },
        regional: { latency: [90, 180], jitter: [14, 45], loss: [4, 20], status: 'warn' },
        international: { latency: [140, 260], jitter: [16, 50], loss: [5, 22], status: 'warn' },
        },
      throughput: { download: [4, 22], upload: [1, 6] },
    },

    internationalFault: {
      id: 'internationalFault',
      weight: 0.25,
      verdict: 'verdictInternational',
      severity: 'warn',
      // Home and ISP are healthy; only the long haul is congested. This is the
      // case a single speed figure can never explain.
      layers: {
        gateway: { latency: [1, 3], jitter: [0.1, 0.6], loss: 0, status: 'ok' },
        regional: { latency: [18, 40], jitter: [0.3, 2], loss: 0, status: 'ok' },
        international: { latency: [240, 430], jitter: [18, 60], loss: [2, 9], status: 'bad' },
      },
      throughput: { download: [30, 70], upload: [10, 25] },
    },
  };

  function resolveRange(value, decimals) {
    if (Array.isArray(value)) return between(value[0], value[1], decimals);
    return value;
  }

  function buildLayer(kind, spec, target) {
    return {
      kind,
      target,
      latency: resolveRange(spec.latency, 2),
      jitter: resolveRange(spec.jitter, 2),
      loss: resolveRange(spec.loss, 0),
      status: spec.status,
    };
  }

  /** Picks a scenario, honouring the weights. */
  function pickScenario() {
    const roll = Math.random();
    let cumulative = 0;
    for (const scenario of Object.values(SCENARIOS)) {
      cumulative += scenario.weight;
      if (roll <= cumulative) return scenario;
    }
    return SCENARIOS.healthy;
  }

  /**
   * Produces one invented report.
   * @param {string} [scenarioId] force a scenario; omit to pick at random.
   */
  function report(scenarioId) {
    const scenario = SCENARIOS[scenarioId] || pickScenario();
    const layers = [
      buildLayer('gateway', scenario.layers.gateway, TARGETS.gateway),
      buildLayer('regional', scenario.layers.regional, TARGETS.regional),
      buildLayer('international', scenario.layers.international, TARGETS.international),
    ];

    // Enforce the ordering rather than trusting the ranges to produce it: the
    // ranges overlap at their edges, and one unlucky draw would print a
    // physically impossible result.
    for (let i = 1; i < layers.length; i += 1) {
      if (layers[i].latency <= layers[i - 1].latency) {
        layers[i].latency = Math.round((layers[i - 1].latency + between(1, 8)) * 100) / 100;
      }
    }

    return {
      scenario: scenario.id,
      severity: scenario.severity,
      verdictKey: scenario.verdict,
      layers,
      download: between(scenario.throughput.download[0], scenario.throughput.download[1]),
      upload: between(scenario.throughput.upload[0], scenario.throughput.upload[1]),
      latency: layers[2].latency,
      jitter: layers[2].jitter,
      dns: between(4, 40),
    };
  }

  /** Grade bands, matching the command line tool's thresholds exactly. */
  function grade(mbps) {
    if (mbps >= 500) return 'excellent';
    if (mbps >= 200) return 'veryGood';
    if (mbps >= 100) return 'good';
    if (mbps >= 50) return 'fair';
    return 'slow';
  }

  return { report, grade, between, SCENARIOS, TARGETS };
})();
