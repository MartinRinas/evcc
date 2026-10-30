# Optimizer

Optimizer uses mixed integer linear programming (MILP) to minize a cost function. The optimizer model implementation honors:

- energy consumption/ feed-in costs
- solar forecast
- base load (aka home) energy demand
- end of forecast commercial value
- strategy- either "charge before export" (charge loads as soon as possible) or attenuating grid peaks on the demand side, the feed-in side or both
- home battery or loadpoint/vehicle...
  - capacity, soc and charge goals
  - charge/discharge power limits and efficiency

Optimization spans N slots with N being minimum of available forecast data.

## Parameters

### Energy consumption/ feed-in costs/ Solar forecast

Grid/ feed-in/ solar tariff data.

TODO

- [ ] make feed-in optional
- [ ] make solar optional

### Base load energy demand

Collected 15min energy profile over the last 4 weeks, averaged per slot. A per-slot percentile (e.g. 50 = median, robust against a few heavy days) can be configured via `/api/profilepercentile` instead. Exposing and scoping that setting is a separate change from this PV prototype.

### Measured value blending

The base load profile is anchored to the current situation using the last completed
15min metrics slot: the measured home consumption replaces the first slot and decays
linearly into the profile over 4 slots. The solar forecast is not blended.

### PV forecast uncertainty: experimental proposal

This branch contains a prototype, not an agreed default risk policy. The original proposal
combined a consumption setting with an automatic PV margin. Review separated those changes
and challenged whether a second PV adjustment is needed at all.

#### Existing correction versus a risk preference

The existing "solar adjusted" option already corrects installation-specific bias using the
median of daily produced/forecasted ratios. For consistently 20% optimistic forecasts,
the scale is 0.8. Another flat discount would duplicate that correction.

Bias correction does not eliminate variation around the median. Ratios of 0.6, 1.0 and
1.4 have a median of 1.0, yet the low-production day can still leave the battery empty.
Planning below the corrected central forecast buys insurance against such days, at the
cost of losses or displaced PV. That is a deliberate economic risk preference, not
necessarily a more accurate forecast.

An extra margin is therefore not universally required. Establish whether remaining
shortfalls come from PV variability after correction, household consumption, timing or
constraints before adding one. A percentile of household load likewise selects a planning
preference; both should be explicit rather than one silently becoming automatic.

Recommendation: retain the existing default and, if live validation supports an additional
PV margin, make it an explicit optimizer opt-in separate from bias correction. The current
prototype does not yet have that opt-in and should not be merged as a default behavior change.

#### Prototype: cost-derived margin (`core/site_optimizer_margin.go`)

The solar forecast is reduced by a margin derived from data the installation already has.
There is currently no setting; this is one of the unresolved design points.

- **Forecast error.** The daily produced/forecasted ratios of the last 30 days are already
  stored and used for the "solar adjusted" scale (median, min 14 days, days below 0.5 kWh
  skipped). Their spread is the installation's own forecast error.
- **Confidence from cost.** The prototype estimates how cautious to be from this request,
  the newsvendor quantile `q = under / (under + over)` over the next 24h:
  - `under`, cost per Wh of storing too little because solar was overestimated: the shortfall is imported in the
    expensive half of the day instead of being stored at the cheapest price,
    `later - cheap/rt`.
  - `over`, cost per Wh of storing too much because solar was underestimated: the insurance energy carries over,
    losing only the round trip, `cheap * (1/rt - 1)`. If the solar surplus fills the
    battery anyway (surplus over home demand ≥ usable capacity) it displaces solar that
    is exported, `cheap/rt - feedIn`.
- **Margin.** Solar is planned at the `(1 - q)` percentile of the ratios, relative to
  their median: `factor = min(1, P(1-q) / P50)`. Relative to the median the margin only
  covers the spread, the bias stays with the "solar adjusted" toggle, so both are
  independent. `q ≤ 0.5` means no margin, the margin never increases solar. `q` is capped
  at 0.9 since 30 days cannot resolve rarer days.
- **Fade-in.** Slot 0 is unchanged and the full factor applies from slot 4. This assumes
  lower near-term uncertainty without measuring forecast error by lead time. Current-slot
  decisions such as the PV surplus check are not affected.

Example with cheap 20 ct, expensive 35 ct, feed-in 8 ct, `rt = 0.81`:

| Situation                          | under | over | q    | Solar planned at |
| ---------------------------------- | ----- | ---- | ---- | ---------------- |
| Winter, solar does not fill        | 10.3  | 4.7  | 0.69 | P31              |
| Summer, solar fills the battery    | 10.3  | 16.7 | 0.38 | median           |
| Flat tariff                        | 0     | 7    | 0    | median           |

The intended seasonal behavior is to value insurance more cheaply when energy carries
over than when it displaces solar. This is a heuristic, not a proven optimal risk policy.
Without a home battery, solar forecast or 14 days of history there is no margin.

#### Alternatives considered

- **Higher battery minSoc**: permanently withholds energy, also when the forecast is right,
  and at the bound the suggestion becomes `normal`.
- **Explicit PV risk setting**: a simpler, maintainable policy boundary and the preferred
  starting point if further PV conservatism is justified. The price-derived `q` can remain
  an experiment rather than prescribing every installation's risk preference.
- **Global default margin**: wrong for flat tariffs (pure loss) and for summer (displaces
  solar).

#### Limitations and future work

- The stored forecast is the latest one before each slot, not a retained day-ahead
  planning snapshot. These ratios cannot establish calibrated day-ahead confidence;
  they may understate the error relevant to night charging. Days below the energy
  threshold also exclude some severe shortfalls.
- `q` uses the minimum import price and the average of the most expensive half of
  slots, ignoring chronology, charging limits and demand weights. The fill check sums
  positive surplus against full usable capacity, ignoring current SoC, intervening
  demand and charging losses. The margin also lacks a grid-charge capability guard.
- In full automatic mode the margin also makes vehicle planning more cautious.
- A separately configured consumption percentile can stack with the PV margin.
- Validate baseline and margin plans from identical input snapshots against actual
  demand and PV, accounting for final battery energy. A single observed shortfall
  cannot demonstrate that the margin improves net cost.
- Future: retain forecasts at planning time to measure lead-time-specific PV errors.
  Any combined consumption/PV error model is a separate design, not required for the
  independent household consumption setting.

### End of forecast commercial value

Use minimum of energy consumption cost.

### Home Battery

### Loadpoint and Vehicles

- home battery or loadpoint/vehicle...
  - capacity, soc and charge goals
  - charge/discharge power limits and efficiency

Without vehicle capacity or soc a configured session energy limit is modelled instead:
state is the session's charged energy, goal is the limit. Loadpoints with neither are
not modelled at all- their power is added to the base load.
