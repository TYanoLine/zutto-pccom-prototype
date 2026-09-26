# 1996 NTT dial-call tariff research

Status: source-backed rate table with prototype geographic routing metadata.

This document records the historical facts used by the web client's atmospheric
telephone-charge display. It does **not** claim that generated fictional phone
numbers are fully geocoded to historical NTT Message Areas (MAs).

## Confirmed historical facts

Primary/current archival NTT sources:

- NTT West, `DATA BOOK / 電話料金 / ダイヤル通話料の推移`:
  https://www.ntt-west.co.jp/info/databook/17/
  - direct tariff-history PDF:
    https://www.ntt-west.co.jp/info/databook/pdf/045-048_dialtsuwaryosuii.pdf
- NTT West, `MA（単位料金区域）`:
  https://www.ntt-west.co.jp/info/databook/pdf/052-053_MA.pdf
- NTT East, historical telephone chronology:
  https://www.ntt-east.co.jp/databook/history-telecom.html

The tariff-history table explicitly labels post-April-1989 monetary values as
tax-exclusive. For the March 1996 row the dial-call unit is 10 yen and the table
expresses the number of seconds covered by each 10-yen unit.

March 1996 NTT dial-call pulse seconds implemented by the client:

| Distance class | Weekday 08:00-19:00 | Weekday 19:00-23:00 | 23:00-08:00 | Sat/Sun/holiday 08:00-23:00 |
| --- | ---: | ---: | ---: | ---: |
| Same MA | 180 | 180 | 240 | 180 |
| Adjacent MA / <=20 km | 90 | 90 | 120 | 90 |
| >20-30 km | 45 | 45 | 60 | 45 |
| >30-60 km | 36 | 36 | 60 | 36 |
| >60-100 km | 22.5 | 30 | 45 | 30 |
| >100-160 km | 13 | 22.5 | 30 | 22.5 |
| >160 km | 13 | 18 | 22.5 | 18 |

The March 1996 change specifically reduced the weekday daytime >160 km rate;
NTT's chronology records the remote-distance reduction on 19 March 1996. The
three-minute maximum-distance weekday daytime charge therefore became 140 yen.

The table uses pulse accounting: a pulse or fraction thereof consumes one
10-yen unit. Thus a 28-second same-MA weekday daytime call is 10 yen, while a
28-second >160 km weekday daytime call spans three 13-second units and is 30 yen.

## Prototype geographic model

The NTT source table is expressed in MA/distance terms. For the current client,
the caller's configured area code is the geographic origin abstraction used to
pick the local-rate bucket. This intentionally keeps the user-facing model at
the same granularity as the dialed BBS directory.

The prototype caller preset is currently:

- label: 福岡
- configured area code: 092
- all `092...` destinations: local-rate bucket
- other destinations: distance-band resolution when geographic metadata exists;
  otherwise the documented >160 km fallback

This is a client simulation rule, not a claim that an NTT area code and MA are
historically interchangeable in every numbering plan. Future center geography
may refine non-local distance bands without changing the user's simple origin
setting.

## Telehodai

Historical references confirm:

- Telehodai began in 1995.
- The discounted window is 23:00 through 08:00.
- Up to two selected telephone numbers are registered.
- Telehodai 1800 applies to same-MA selected numbers.
- Telehodai 3600 extends selection to adjacent MA / <=20 km destinations.

The client therefore does not waive a far-distance call merely because its
number appears in the configured Telehodai number list.

## Calendar handling

World time is represented by `Japan1996WorldClock` as Japanese wall-clock
fields encoded in UTC. Tariff code intentionally uses `getUTC*` accessors.

Saturday, Sunday and Japanese public holidays use the NTT weekend/holiday
daytime tariff. The 1996 holiday set follows the National Astronomical
Observatory calendar rules, including substitute holidays and the first Marine
Day in 1996.

## Location configuration direction

Caller origin is persisted separately from modem communication settings under
`zutto.callerLocation.v1`. The default is 福岡 / 092 as requested for the
prototype.

Future UI should allow selecting a caller location/area code without changing
terminal or BBS state. The world/center catalog should eventually expose
destination geography or a tariff-distance class so non-local calls can resolve
intermediate historical distance bands instead of falling back to >160 km.
