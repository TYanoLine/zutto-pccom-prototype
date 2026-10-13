# PC-9801 テキスト画面におけるカーソル明滅（点滅）仕様調査

## 1. 概要

本ドキュメントは、「ずっとパソコン通信」の端末エミュレーション（Webクライアント）において、PC-9801相当のカーソル点滅（Blink）を忠実に再現するための歴史的ハードウェア仕様、クロック・同期周波数、および点滅周期の計算根拠を記録する。

## 2. 歴史的証拠区分

- **Confirmed（確定事実）**:
  - PC-9801シリーズのテキスト画面表示は、専用のテキストGDC（Graphic Display Controller: **NEC μPD7220 / μPD7220A**、Master GDC）によってハードウェア制御される。
  - PC-9801の標準テキスト画面（640×400ドット、水平同期 24.83 kHz）における垂直同期周波数（画面リフレッシュレート）は **約 56.42 Hz** である。
  - μPD7220 GDCの `CSRFORM`（Cursor Form、コマンドコード `4Bh`）仕様において、カーソルの点灯時間および消灯時間はビデオフレーム単位でカウントされ、それぞれ $2 \times BR$ （BR: Blink Rate パラメータ）フレームである（デューティ比 50%）。
  - PC-98の標準設定における GDC の点滅レートは、点灯 **32 フレーム** / 消灯 **32 フレーム**（1周期 **64 フレーム**）。
  - カーソルの幅は、全角文字（漢字等）にある場合は全角幅（16ドット）、半角文字の場合は半角幅（8ドット）に作用する。カーソル形状（下線か全セル矩形か）は、この文書の史料では確認していない（下記 Modern adaptation を参照）。
- **Modern adaptation（現代的適合）**:
  - ブラウザ環境での電力消費抑制のため、タブ非表示（`document.hidden`）時はアニメーションタイマーを停止。
  - アクセシビリティ規格（WCAG / `prefers-reduced-motion`）への適合のため、ユーザーが動作抑制を指定している場合は点滅を無効化（常時点灯）。
  - キー打鍵時および文字受信時の視認性維持のため、入力イベント発生時に点灯フェーズへのリセットを行う。
  - カーソル形状は、現在は全セル（高さ100%）の矩形で描く。挿入モード時に下40%の矩形にする案は未実装（切り替え方法は未決）。形状は史料で確認されていないため、この項目は現代的な適合（UIの仕様）である。

## 3. 点滅周期の計算根拠

### 3.1 ハードウェアパラメータ

1. **画面同期周波数**:
   - 水平同期周波数: 24.83 kHz
   - 垂直同期周波数: $f_v \approx 56.422\text{ Hz}$（約 56.4 Hz）
2. **GDC μPD7220 CSRFORM パラメータ**:
   - `CSRFORM` コマンドバイト: `4Bh`
   - パラメータ `BR`（Blink Rate）: デフォルト 16（$2 \times BR = 32$ フレーム）
   - 点灯時間（Blink-On Time）: 32 ビデオフレーム
   - 消灯時間（Blink-Off Time）: 32 ビデオフレーム
   - 1周期（Period）: 64 ビデオフレーム

### 3.2 時間（ミリ秒）および周波数の算出

- **点灯時間（ON）**:
  $$\text{Time}_{\text{on}} = \frac{32\text{ frames}}{56.422\text{ Hz}} \approx 0.56715\text{ 秒} \approx 567\text{ ms}$$
- **消灯時間（OFF）**:
  $$\text{Time}_{\text{off}} = \frac{32\text{ frames}}{56.422\text{ Hz}} \approx 0.56715\text{ 秒} \approx 567\text{ ms}$$
- **点滅周期（Total Period）**:
  $$\text{Period} = \frac{64\text{ frames}}{56.422\text{ Hz}} \approx 1.1343\text{ 秒} \approx 1134\text{ ms}$$
- **点滅周波数（Frequency）**:
  $$f_{\text{blink}} = \frac{56.422\text{ Hz}}{64\text{ frames}} \approx 0.8816\text{ Hz} \approx 0.88\text{ Hz}$$
- **デューティ比（Duty Cycle）**:
  $$\frac{32}{64} = 50\%$$（等間隔）

一般的なPC/AT互換機（DOS/V、VGA 70Hz、BIOS INT 10h等）のカーソル点滅（約1Hz〜2Hz、点滅間隔約250ms〜500ms）と比較して、PC-9801のカーソル点滅は約0.88Hz（点灯約567ms / 消灯約567ms、1周期約1.13秒）と、ややゆったりとした上品な明滅リズムを持つのが特徴である。

## 4. 参照資料

1. NEC Microcomputers, *μPD7220 / μPD7220A Graphic Display Controller User's Manual*, 1987.
2. Jeff Wise and Henryk Szejnwald, *A Single-Chip Graphics Display Controller for Sophisticated Display Terminals*, IEEE Transactions on Consumer Electronics / NEC Technical Paper, 1981.
3. アスキー出版局, 『Undocumented PC-9801/9821 Vol.1/Vol.2』.
4. DOSBox-X プロジェクト (`src/hardware/vga_pc98_gdc.cpp`, `src/hardware/vga_draw.cpp`).
   - `cursor_blink_rate = 0x20` (32 frames)
   - `cursor_blink_state` (4-state cycle: 0=off, 1=on, 2=off, 3=on)
   - `/* based on real hardware, the cursor seems to act like a reverse attribute */`
   - `/* if the character is double-wide, and the cursor is on the left half, the cursor affects the right half too. */`
