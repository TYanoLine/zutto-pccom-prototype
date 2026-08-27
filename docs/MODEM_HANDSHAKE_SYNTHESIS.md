# Modem Handshake Synthesis Design

This document records the reference design for modem startup audio in **zutto-pccom-prototype**.

The goal is not to make a generic "dial-up-like" sound effect. The goal is to reproduce the **structure and perceptual character of real modem startup signalling** closely enough that someone familiar with 1990s modems hears the expected sequence.

## Core rule

Do **not** return to the old `noiseBurst()` approach that approximates training with dozens of random-frequency sine waves.

The startup sound should be generated from protocol-shaped signals:

1. deterministic signalling / training sequence
2. FSK / PSK / QAM modulation as appropriate
3. pulse shaping where appropriate
4. calling- and answering-modem signals mixed in the time domain
5. telephone-channel band limiting and mild analogue coloration

A useful conceptual pipeline is:

```text
protocol state / bit or symbol sequence
    -> modulation
    -> pulse shaping
    -> calling + answering paths mixed
    -> telephone channel filter
    -> mild saturation / line noise / level variation
```

Recorded WAV references may be used to tune **timing, level, spectral balance, overlaps, and perceived duration**, but should not be copied or sliced into the generated result.

---

## General DSP primitives

The implementation should provide reusable primitives roughly equivalent to:

```ts
tone(freq, duration)
phaseReversalTone(freq, reversalPeriod, duration)
cpfsk(mark, space, baud, bits)
dbpsk(carrier, baud, bits)
qam(carrier, symbolRate, constellation, symbols, pulseShape)
multitone(freqs, phases, levels, duration)
telephoneChannel(lowCut, highCut, saturation, noise)
```

For QAM-like phases, generate actual complex symbols and use root-raised-cosine or equivalent pulse shaping before upconversion. Do not emulate them with static random sine sums.

The telephone-channel stage should usually be in approximately the 300-3400/3800 Hz range, with only light analogue-like saturation. Avoid hard clipping.

---

# 2400 bps — V.22bis direction

The current preferred direction is a **two-band 600-symbol/s QAM reconstruction**, rather than broad pseudo-noise.

Perceptually important structure from the reference:

```text
~2100 Hz answer tone with phase reversals
    -> narrow negotiation / carrier tones
    -> short low/high channel markers
    -> simultaneous low-band and high-band QAM-like training
```

The final training region should be generated as two independent modulated channels, roughly centred around the classic low/high V.22bis voice-band regions, then mixed.

Important: preserve the clearly separated two-band spectral character. A single wide noise burst is wrong.

---

# 9600 bps — V.32 reference design (V3 baseline)

This is the current accepted baseline for the 9600 startup sound.

## Perceptual sequence

The reference has a distinctive transition:

```text
PIIIII...
    -> BEEEEEN
    -> BIN / resonant multi-tone transition
    -> structured training bursts
```

The important detail is that the 1800 Hz tone begins **before the 2100 Hz answer tone has fully ended**.

## Timing / signalling model

Use a deliberately non-rushed startup; the active audio should be around six seconds rather than ending after only 3-4 seconds.

Reference V3 structure:

```text
0.00 s   2100 Hz ANS begins
         phase reversal about every 450 ms

~1.18 s  1800 Hz AA/CC-like tone begins
         it overlaps the still-present 2100 Hz tone

~1.55 s  1800 Hz remains dominant
~1.62 s  weak 600 Hz and 3000 Hz components are added

         approximate spectral relationship:
         600 : 1800 : 3000 ~= 0.3 : 1.0 : 0.3

~2.08 s  first 2400-symbol/s QAM training block
~3.11 s  short structured S/S'-like sync block
~3.31 s  opposite-side training block
~4.29 s  short full-duplex sync cue
~4.47 s  second long training block
~5.59 s  scrambled/data-like tail
~6.2 s   transition to CONNECT
```

The exact wall-clock values are tuning targets, not normative ITU requirements; perceptual pacing is important.

## Modulation

Use approximately:

```text
carrier:     1800 Hz
symbol rate: 2400 symbols/s
```

Training blocks should be **deterministic and different from one another**. For example:

- periodic constellation traversal
- corner-point sequence
- PRBS/scrambled symbol sequence
- short four-phase structured sequence

This creates the characteristic machine-like segmentation instead of one unchanging noise bed.

---

# 28800 bps — V.34 reference design (V3 baseline)

This is the most important section. Earlier versions were too short and sounded like a collection of generic modem effects rather than a V.34 startup.

The V3 design deliberately follows the high-level V.34 startup progression and gives each phase enough audible time.

Target active duration: roughly **9 seconds** before CONNECT.

## Phase 1 — V.8 negotiation

### ANSam

Generate the answer signal as approximately:

```text
carrier:             2100 Hz
phase reversals:     ~450 ms interval
amplitude modulation: ~15 Hz, modest depth
```

Do not stop this tone as soon as the calling-side negotiation starts. The characteristic sound comes partly from **other signalling appearing underneath the still-present ANSam**.

### CM / JM

Use V.21-style 300-bps continuous-phase FSK for the menu/joint-menu exchange.

Useful channel frequencies:

```text
low channel:   ~980 / 1180 Hz
high channel:  ~1650 / 1850 Hz
```

Generate deterministic repeated bit patterns. The purpose here is structural/perceptual reconstruction, not yet a bit-exact implementation of every V.8 information field.

Include a short calling-side terminator before moving into INFO signalling.

---

## Phase 2 — INFO and line probing

### INFO-like signalling

Use short 600-bps differential-PSK-like bursts around the two primary voice-band regions (roughly 1200 Hz and 2400 Hz), followed by A/B-like ranging markers.

### L1 / L2 probes

The probe must be an explicit multitone signal, not noise.

Use a 150-Hz frequency grid with the characteristic omissions. Current V3 frequency set:

```text
150
300
450
600
750
1050
1350
1500
1650
1950
2100
2250
2550
2700
2850
3000
3150
3300
3450
3600
3750 Hz
```

In other words, the 150-Hz grid omits the prominent 900 / 1200 / 1800 / 2400 Hz positions.

Current V3 phase pattern in degrees:

```text
0, 180, 0, 0, 0, 0, 0, 0, 180, 0, 0,
180, 0, 180, 0, 180, 180, 180, 180, 0, 0
```

Use a hotter short L1-like probe followed by a longer lower-level L2-like probe. Then repeat the negotiation/probe structure for the opposite direction with different surrounding markers.

This section is responsible for much of the recognizable **BEEEEEE / comb-like** V.34 sound.

---

## Phase 3 — primary-channel training for 28.8 kbps

Important correction from the earlier prototype:

**Do not use 3429 symbols/s as the default for the 28.8 kbps profile.**

For the current 28.8 reconstruction, use:

```text
carrier:     1800 Hz
symbol rate: 3000 symbols/s
```

The training sequence should be broken into visibly/audibly distinct protocol-shaped blocks rather than one generic 64-QAM burst.

Current V3 progression:

```text
short quiet transition
    -> S-like block (~128T)
    -> Sbar-like block (~16T)
    -> PP polyphase training
    -> extended TRN
    -> short J/J'-like structured exchange
    -> opposite-side S/Sbar
    -> opposite-side PP
    -> opposite-side TRN
    -> final data/training-like tail
```

For PP, use a deterministic polyphase sequence. The current prototype uses a repeated 48-symbol phase pattern to create the correct structured spectral character.

TRN should use deterministic low-order points first, then transition toward data-like shaped symbols.

For final high-density signalling, a shaped V.34-like lattice approximation is preferable to ordinary square 64-QAM. A practical prototype method is to choose the lower-energy points from a larger odd-integer QAM lattice and normalize RMS power.

This is still a perceptual/structural reconstruction; a future implementation may replace these approximations with exact V.34 training tables and scramblers.

---

# Duration matters

A major lesson from the first synthesis attempts: matching the file length is not enough.

If the generated signalling ends early and the rest of the WAV is silence, the startup feels unnaturally short.

The signal timeline itself should remain active for approximately:

```text
9600 / V.32:   ~6.2 s
28800 / V.34:  ~9.0 s
```

These are current perceptual targets from the accepted V3 prototypes and may be retuned later.

---

# Current quality bar

The V3 prototypes are the baseline to preserve.

When changing the implementation:

- preserve the `2100 -> overlapping negotiation -> structured training` progression
- preserve realistic signalling overlaps
- preserve distinct training blocks
- preserve the longer pacing
- preserve V.34 line-probe comb structure
- keep 28.8 kbps at the current 1800 Hz / 3000-symbol/s baseline unless intentionally revisiting the modem mode
- do not replace modulated sequences with generic random-noise sound effects

The objective is: **someone who remembers real dial-up modem startup audio should recognize the connection process, not merely recognize a sound effect as "a modem".**
