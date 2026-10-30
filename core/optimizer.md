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

Collected 15min household base load over the last 4 weeks, averaged per slot by default.
The optimizer settings expose the existing `/api/profilepercentile` setting as
"Expected consumption": Average (default), Cautious (P60), More cautious (P75) and Very
cautious (P90). Other API-configured values remain selectable.

The percentile is calculated separately for each slot, not for the total day.
A higher percentile favors higher consumption within each slot's history and can lead to
more advance grid charging, but is not a percentage uplift or a daily confidence level.
Even P60 can be below the average when a few large loads skew the history.

Only the household base load collector uses this setting. Heating loadpoint profiles,
including daily, weekday and temperature-binned profiles, continue to use averages.
PV forecasts, vehicle goals and measured-value blending are unchanged. Heating or other
appliances not separately measured as loadpoints remain part of the household base load;
the setting cannot distinguish them within that aggregate measurement.

This is deliberately independent of PV forecast adjustment and battery minimum SoC.
The original uncertainty proposal combined both changes, but a household preference can
be reviewed and used without adding another PV correction or an automatic risk policy.
Consumption forecast error calibration would require retaining the forecasts made at
planning time; the existing history only stores actual consumption.

### Measured value blending

The solar forecast and the base load profile are anchored to the current situation
using the last completed 15min metrics slot, decaying linearly over 4 slots:

- base load: the measured home consumption replaces the first slot and decays into the profile
- solar: the scale factor measured production/forecasted production is applied to the first slot and decays towards 1

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
