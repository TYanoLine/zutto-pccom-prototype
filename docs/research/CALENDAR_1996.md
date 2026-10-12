# 1996 world calendar research

Status: source-backed holiday table plus labeled customary periods. Used by
`apps/server/internal/worldcalendar` (spec 015) to vary resident connection
likelihood by day. The *effects* of these days on connections
(`worldcalendar/data/effects.yaml`) are fictional tuning values, not historical
statistics; this document covers only the calendar facts.

## National holidays — Confirmed

Source: Cabinet Office (内閣府) list of national holidays and substitute
holidays, 1955 onward: https://www8.cao.go.jp/chosei/shukujitsu/syukujitsu.csv
(linked from https://www8.cao.go.jp/chosei/shukujitsu/gaiyou.html). Read on
2026-10-12. The CSV lists the following 19 days for 1996; substitute holidays and
the 5/4 day between two holidays appear as `休日`.

| Date | Weekday | Name |
| --- | --- | --- |
| 1996-01-01 | Mon | 元日 |
| 1996-01-15 | Mon | 成人の日 |
| 1996-02-11 | Sun | 建国記念の日 |
| 1996-02-12 | Mon | 休日（振替） |
| 1996-03-20 | Wed | 春分の日 |
| 1996-04-29 | Mon | みどりの日 |
| 1996-05-03 | Fri | 憲法記念日 |
| 1996-05-04 | Sat | 休日（祝日に挟まれた日） |
| 1996-05-05 | Sun | こどもの日 |
| 1996-05-06 | Mon | 休日（振替） |
| 1996-07-20 | Sat | 海の日（この年が最初） |
| 1996-09-15 | Sun | 敬老の日 |
| 1996-09-16 | Mon | 休日（振替） |
| 1996-09-23 | Mon | 秋分の日 |
| 1996-10-10 | Thu | 体育の日 |
| 1996-11-03 | Sun | 文化の日 |
| 1996-11-04 | Mon | 休日（振替） |
| 1996-11-23 | Sat | 勤労感謝の日 |
| 1996-12-23 | Mon | 天皇誕生日 |

This is the same set as `JAPAN_1996_HOLIDAYS` in
`apps/web/src/billing/pseudoTariffs.ts` (which cites the National Astronomical
Observatory calendar notice); a server test checks that the two lists agree.

The same CSV also lists 1997. 1997 is outside spec 015 and is not loaded yet.

## Days — Confirmed (fixed calendar dates)

| Key | Date | Note |
| --- | --- | --- |
| `christmas_eve` | 1996-12-24 (Tue) | Calendar date. |
| `new_years_eve` | 1996-12-31 (Tue) | Calendar date. |

## Periods — Likely / inferred

These are customary or institution-specific periods. Their exact span varied by
employer, school board and region, so they are recorded as **Likely** and used
only as probability modifiers.

| Key | Span used | Basis |
| --- | --- | --- |
| `year_end_new_year` | 1996-12-29 – 1997-01-03 | National government offices close 12/29–1/3 under 行政機関の休日に関する法律 第1条 (https://laws.e-gov.go.jp/law/363AC0000000091). Private employers commonly used similar spans; not every resident is off. |
| `obon` | 1996-08-13 – 1996-08-16 | Customary お盆 period (new-calendar August). Not a public holiday; time off varied by employer and region. |
| `school_summer` | 1996-07-20 – 1996-08-31 | Typical public-school summer break; start/end differ by prefecture and school. |
| `school_winter` | 1996-12-25 – 1997-01-07 | Typical winter break; varies by school. |
| `school_spring` | 1996-03-25 – 1996-04-07 | Typical spring break; varies by school. |

## Events — not loaded

`events` is empty for now. Candidate events that might change connection
patterns (for example the Atlanta Olympics in July–August 1996, or the NHK 紅白歌合戦
on New Year's Eve) need a primary source for the exact dates and broadcast times
before they are added. None was confirmed while writing this.
