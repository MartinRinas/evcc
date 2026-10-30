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

Collected 15min energy profile over the last 4 weeks, averaged per slot. A per-slot percentile (e.g. 50 = median, robust against a few heavy days) can be configured as "Expected consumption" in the optimizer settings or via `/api/profilepercentile` instead. See [Forecast uncertainty](#forecast-uncertainty) for what it does and does not do.

### Measured value blending

The base load profile is anchored to the current situation using the last completed
15min metrics slot: the measured home consumption replaces the first slot and decays
linearly into the profile over 4 slots. The solar forecast is not blended.

### Forecast uncertainty

The optimizer is deterministic: it plans as if solar and consumption forecasts were exact.
When reality is worse than the forecast, energy that could have been stored cheaply at
night is imported later at a higher price (#35518). Two mechanisms address this.

#### Solar: automatic margin (`core/site_optimizer_margin.go`)

The solar forecast is reduced by a margin derived from data the installation already has.
There is no setting.

- **Forecast error.** The daily produced/forecasted ratios of the last 30 days are already
  stored and used for the "solar adjusted" scale (median, min 14 days, days below 0.5 kWh
  skipped). Their spread is the installation's own forecast error.
- **Confidence from cost.** How cautious to be follows from the prices of this request,
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
- **Fade-in.** Like measured blending, slot 0 is unchanged and the full factor applies
  from slot 4: the near-term forecast is reliable and current-slot decisions such as the
  PV surplus check are not affected.

Example with cheap 20 ct, expensive 35 ct, feed-in 8 ct, `rt = 0.81`:

| Situation                          | under | over | q    | Solar planned at |
| ---------------------------------- | ----- | ---- | ---- | ---------------- |
| Winter, solar does not fill        | 10.3  | 4.7  | 0.69 | P31              |
| Summer, solar fills the battery    | 10.3  | 16.7 | 0.38 | median           |
| Flat tariff                        | 0     | 7    | 0    | median           |

This is why one rule works in summer and winter without a seasonal switch: insurance is
cheap when energy carries over and expensive when it would displace solar. Without a home
battery, solar forecast or 14 days of history there is no margin.

#### Consumption: expected consumption setting

Consumption uncertainty is not handled automatically yet: evcc does not store what it
expected the home to consume, so there is no forecast error history like for solar. Until
then the existing per-slot percentile of the 4 week profile is offered in the optimizer
settings as "Expected consumption": average (default), cautious (P60), more cautious (P75),
very cautious (P90).

- It raises the planned load in slots that vary a lot (evenings, cooking), so more energy
  is kept in the battery for them.
- It is a percentile per slot, not a confidence level for the day. P90 in every slot sums
  to far more than a P90 day and P50 can be below the average since load is skewed.
- It also applies to the heating demand profiles.

#### Alternatives considered

- **Higher battery minSoc**: permanently withholds energy, also when the forecast is right,
  and at the bound the suggestion becomes `normal`.
- **Fixed or user-selected margin**: the right size depends on the tariff spread and the
  installation's forecast quality, which change daily and are known to evcc. A single knob
  for solar and consumption would mix two errors with different data.
- **Global default margin**: wrong for flat tariffs (pure loss) and for summer (displaces
  solar).

#### Limitations and future work

- The stored forecast is the latest one before each slot, closer to reality than the
  day-ahead forecast used for night charging, so the margin is somewhat too small.
- In full automatic mode the margin also makes vehicle planning more cautious.
- A cautious consumption setting and the solar margin stack.
- Future: store the expected daily consumption, derive one combined net demand error
  (consumption minus solar) and apply the same `q`. The consumption setting then becomes
  an expert override.

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
