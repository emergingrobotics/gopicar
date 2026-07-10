# Manual Testing How-To — `picarctl` on the Real PiCar-X

Step-by-step instructions for hand-testing every function currently implemented in the Go driver, using the `picarctl` CLI on the actual robot. The goal is narrow: **prove the Go code moves the real hardware and reads real sensors.**

Each command below drives the Milestone-1 primitives (`pkg/bus`, `pkg/gpio`, `pkg/mcu`) directly. Section markers like §7 refer to [`../../picar-x-apis.md`](../../picar-x-apis.md).

> **Companion doc:** [`start-testing.md`](../start-testing.md) covers the M1 identity/bus smoke tests (`ping-mcu`, `hat-info`, `blink`, `reset-mcu`) and the milestone growth map. This doc is the hands-on procedure for the actuator/sensor commands.

---

## Contents

1. [Before you start](#1-before-you-start)
2. [Build and deploy](#2-build-and-deploy)
3. [Pre-flight: confirm the bus is alive](#3-pre-flight-confirm-the-bus-is-alive)
4. [GPIO — `blink`](#4-gpio--blink)
5. [Servos — `servo`](#5-servos--servo)
6. [Motors — `motor` / `stop`](#6-motors--motor--stop)
7. [ADC / battery / grayscale — `read-adc`, `read-battery`, `read-grayscale`](#7-adc--battery--grayscale)
8. [Ultrasonic — `distance`](#8-ultrasonic--distance)
9. [MCU reset + recovery — `reset-mcu`](#9-mcu-reset--recovery--reset-mcu)
10. [Full pass checklist](#10-full-pass-checklist)
11. [Troubleshooting](#11-troubleshooting)

---

## 1. Before you start

**Safety first — this is a robot with motors and gears.**

- **Put the car on a stand** (wheels off the ground) before any `motor` test. A "60% forward, 1 second" command on the bench will drive it off your desk.
- Keep fingers clear of the wheels and the steering linkage during `servo`/`motor` tests.
- `motor` **auto-stops** after its `--duration` (default 1 s). `stop` halts both motors immediately. During a timed run, **`Ctrl-C` stops the motor gracefully** (the command traps SIGINT and issues the stop). The one exception is `--duration 0`, which leaves the motor running with no wait to interrupt — you must run `stop` yourself.
- Battery: a healthy 2S pack reads roughly **6.6–8.4 V**. Below ~6.5 V the motors get weak and servos jitter — charge before trusting a "motor doesn't move" result.

**What you need**

- A PiCar-X with the Robot HAT powered on (battery in, switch on — USB power to the Pi alone does **not** power the HAT/motors/servos).
- SSH access to the Pi.
- The one-time Pi setup from [`start-testing.md` §2](../start-testing.md#2-one-time-pi-setup): I²C enabled, `/dev/i2c-1` and `/dev/gpiochip0` present, and your user in the `i2c` and `gpio` groups.

---

## 2. Build and deploy

From the `gopicar/` module root on your dev machine — static binary, no cgo, cross-compiled:

```bash
# Pi 5 / Pi 4 on 64-bit Raspberry Pi OS
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o picarctl ./cmd/picarctl

# 32-bit Pi OS on Pi 4:        GOARCH=arm GOARM=7
# Pi Zero / Zero 2 W (32-bit): GOARCH=arm GOARM=6
```

Verify and copy:

```bash
file picarctl        # expect: ELF 64-bit LSB executable, ARM aarch64, statically linked
scp picarctl pi@raspberrypi.local:/home/pi/
```

Then SSH in and run everything from there:

```bash
ssh pi@raspberrypi.local
./picarctl help      # prints the full command list
```

> **Building on the Pi instead?** `go build -o picarctl ./cmd/picarctl` works too — no cross-compile flags needed.

---

## 3. Pre-flight: confirm the bus is alive

Every command except `distance` needs a working I²C link to the MCU. Confirm it once:

```bash
./picarctl ping-mcu
```

**Expect:** `MCU at 0x14, firmware X.Y.Z` (address may be `0x15`/`0x16`).
**If it fails:** stop and fix the bus before testing actuators/sensors — see [Troubleshooting](#11-troubleshooting). A dead bus makes every servo/motor/ADC test fail for the same root cause.

Cross-check with the stock tool if unsure:

```bash
i2cdetect -y 1       # expect a cell at 0x14 (or 0x15 / 0x16)
```

---

## 4. GPIO — `blink`

The simplest end-to-end test: it uses **only** GPIO (no HAT/MCU needed), so it isolates the `pkg/gpio` character-device path.

```bash
./picarctl blink --pin D14 --count 10
```

**What it does:** claims the line, toggles it high/low 10 times at ~5 Hz.
**Expect:** the on-board **user LED (D14 = GPIO26) blinks 10 times**, then `blinked D14 10 times`.
**Validates:** `gpio.Open` → `RequestOutput` → `Write` on the real `/dev/gpiochip0`.

**Variations**

```bash
./picarctl blink --pin D14 --count 3     # fewer blinks
```

Any pin name from the [§3.1 map](../../picar-x-apis.md) is accepted (`D0`–`D16`, `LED`, `MCURST`, …). Stick to `D14` (the LED) unless you know what a given pin drives.

---

## 5. Servos — `servo`

Drives the three hobby servos via the MCU's timer-0 50 Hz PWM (§7). Each call re-programs the timer prescaler/period (idempotent) then writes the pulse-width duty.

| Servo | Flag | Channel | Software limit |
|-------|------|---------|----------------|
| Steering | `--which steer` | P2 | ±30° |
| Camera pan | `--which pan` | P0 | ±90° |
| Camera tilt | `--which tilt` | P1 | −35°…+65° |

### Steering servo

```bash
./picarctl servo --which steer --angle 0     # center
./picarctl servo --which steer --angle -30   # full left
./picarctl servo --which steer --angle 30    # full right
```

**Expect:** the front wheels swing to each position; the tool prints e.g. `servo P2 → 0.0°`. Out-of-range values are clamped (`--angle 90` on `steer` clamps to `30.0°`).
**Look for:** smooth motion to a held position. A servo that buzzes without moving usually means low battery or a mechanical bind.

### Camera pan / tilt

```bash
./picarctl servo --which pan  --angle 0      # look straight ahead
./picarctl servo --which pan  --angle 45     # look right
./picarctl servo --which tilt --angle 0      # level
./picarctl servo --which tilt --angle 30     # look up
```

**Expect:** the camera gimbal pans/tilts to each angle.

### Raw channel (bypass the named limits)

Use this to test a servo plugged into a non-standard channel, or to probe the full ±90° range:

```bash
./picarctl servo --channel P0 --angle 90     # clamped only to the ±90° servo hardware limit
./picarctl servo --channel P2 --angle 0
```

### Centering all three (a good "known state")

```bash
./picarctl servo --which steer --angle 0
./picarctl servo --which pan   --angle 0
./picarctl servo --which tilt  --angle 0
```

**Validates:** timer-0 prescaler/period register writes (`0x40`/`0x44`) + per-channel duty (`0x20+n`), all as big-endian `[hi,lo]` blocks over `bus.WriteBlock`.

---

## 6. Motors — `motor` / `stop`

> ⚠️ **Car on a stand, wheels up, before the first motor test.**

Drives one rear motor: sets timer-3 PWM (channels P12/P13) and the direction GPIO (D4/D5), following the stock `set_motor_speed` logic (§6).

### Basic forward / reverse

```bash
./picarctl motor --which left  --speed 60    # left wheel forward for 1 s, then auto-stop
./picarctl motor --which right --speed 60    # right wheel forward
./picarctl motor --which left  --speed -60   # left wheel reverse
```

**Expect:** the named wheel spins the commanded direction for ~1 s, then `motor left → speed 60 (P13)` followed by `motor left stopped`.

**Note on the speed remap:** the stock driver remaps `1..100 → 50..100` because the gearbox won't turn below ~50% duty. So `--speed 10` and `--speed 60` both spin (at 55% and 80% duty respectively). This is expected — it matches the Python library.

### Raw duty (no remap)

To test the PWM path linearly — including the dead zone below ~50%:

```bash
./picarctl motor --which left --speed 40 --raw    # writes 40% duty directly
./picarctl motor --which left --speed 80 --raw
```

**Expect:** at `--raw --speed 40` the wheel may barely move or not at all (below the gearbox threshold); at `80` it spins clearly. That difference *confirms* the raw path works.

### Run longer / stop

```bash
./picarctl motor --which right --speed 50 --duration 3s   # run 3 s
./picarctl motor --which left  --speed 50 --duration 0    # run indefinitely (no auto-stop)
./picarctl stop                                           # stop BOTH motors now
```

> With `--duration 0` the motor keeps running after the command returns — you **must** call `./picarctl stop` (or power off) to halt it.

### Direction sanity check

Run both motors forward and confirm which way each wheel turns:

```bash
./picarctl motor --which left  --speed 60 --duration 2s
./picarctl motor --which right --speed 60 --duration 2s
```

On the chassis the two rear wheels face opposite directions, so "drive forward" in a real control loop sends them **opposite signed speeds** (§6.3). For this manual test you're just confirming each motor + its direction pin respond — not that the car drives straight.

**Validates:** timer-3 setup (`0x43`/`0x47`), PWM duty writes (`0x2C`/`0x2D`), and the direction GPIO (`D4`/`D5`) via `pkg/gpio`.

---

## 7. ADC / battery / grayscale

Reads the 12-bit ADC channels behind the MCU using the stock 3-byte `[reg,0,0]` write + 2-byte big-endian read (§5.2), exercised through the new `bus.Tx` combined transaction.

> **Order matters:** do **not** run `reset-mcu` right before an ADC read — resetting the MCU poisons ADC state to the constant tuple `[2571, 3085, 3599]` until re-init (§17). These read commands never reset, so they're safe on their own.

### Battery voltage

```bash
./picarctl read-battery
```

**Expect:** `battery: raw=NNNN X.XX V` — a plausible pack voltage (~6.6–8.4 V charged). The raw value runs the divider math `V_bat = raw/4095 · 3.3 · 3`.
**Sanity:** compare against a multimeter on the pack, or against a charged/discharged pack — the number should track.

### Individual ADC channel

```bash
./picarctl read-adc --channel A0     # grayscale left
./picarctl read-adc --channel A3     # free channel
./picarctl read-adc --channel A4     # battery divider (same source as read-battery)
```

**Expect:** `A0: raw=NNNN  voltage=X.XXX V`, raw in `0..4095`.

### Grayscale line sensors

```bash
./picarctl read-grayscale
```

**Expect:** `grayscale [L M R] = [NNNN NNNN NNNN]`.

Move the sensor over different surfaces and re-run — the values should change clearly:

| Surface | Typical raw (§9.1) |
|---------|--------------------|
| White paper | ~1800–2500 |
| Black tape | ~100–400 |
| Off a table edge (cliff) | ~50–200 |

**Best test:** run it once over white, once with the middle sensor over black tape — the middle number should drop by ~1500+. That swing proves all three channels read independently.

**Validates:** the ADC register addressing (`(7-n)|0x10`) and the `bus.Tx` write-then-read framing on real silicon.

---

## 8. Ultrasonic — `distance`

Fires the HC-SR04 and times the echo using **kernel-timestamped GPIO edges** (§8). This is **GPIO-only** — no I²C — so it works even if the MCU is unreachable.

```bash
./picarctl distance                    # single ping
./picarctl distance --count 5          # five pings
./picarctl distance --count 5 --timeout 30ms
```

**Expect:** `distance: NN.NN cm` per ping.
**Best test:** hold your hand ~20 cm in front of the sensor and watch the number track as you move it in and out. Point it at a far wall and the reading grows; point it at open space past ~3.4 m and you get `echo timeout (...)` (that's the 20 ms default timeout, not a bug — §8).

**Validates:** the D2 trigger pulse (output) + D3 echo edge capture (input, pull-down) with monotonic timestamps from `pkg/gpio`'s `RequestEdges`.

---

## 9. MCU reset + recovery — `reset-mcu`

Pulses the MCU reset line (GPIO5) low 10 ms → high 10 ms (§5.4). Use it to recover a wedged MCU.

```bash
./picarctl reset-mcu && ./picarctl ping-mcu
```

**Expect:** `MCU reset pulsed (GPIO5 low 10ms → high 10ms)`, then `ping-mcu` **still succeeds**. The second success is the real assertion — reset should restore the bus, not wedge it.

> ⚠️ **Side effect:** if you had just been reading the ADC, reset poisons ADC readings to `[2571, 3085, 3599]` until the next power-cycle/re-init (§17). After a `reset-mcu`, re-run other commands fresh; don't trust an ADC read taken immediately after a reset.

---

## 10. Full pass checklist

Run top to bottom on the robot. Every line should behave as noted.

```bash
# --- identity / bus (see start-testing.md for detail) ---
./picarctl ping-mcu                      # MCU at 0xNN, firmware X.Y.Z
./picarctl hat-info                      # HAT V4/V5, speaker pin, motor mode, uuid

# --- GPIO ---
./picarctl blink --pin D14 --count 5     # user LED blinks 5×

# --- servos (car can be on the ground for these) ---
./picarctl servo --which steer --angle -30
./picarctl servo --which steer --angle 30
./picarctl servo --which steer --angle 0
./picarctl servo --which pan   --angle 0
./picarctl servo --which tilt  --angle 0

# --- motors (CAR ON A STAND, wheels up) ---
./picarctl motor --which left  --speed 60 --duration 2s
./picarctl motor --which right --speed 60 --duration 2s
./picarctl motor --which left  --speed -60 --duration 2s
./picarctl stop

# --- sensors ---
./picarctl read-battery                  # ~6.6–8.4 V charged
./picarctl read-grayscale                # 3 values that change with surface
./picarctl distance --count 5            # tracks a hand moved in front

# --- reset + recovery ---
./picarctl reset-mcu && ./picarctl ping-mcu
```

**The Go driver passes manual testing when every command above produces its expected physical behavior and output on the robot.**

---

## 11. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| `ping-mcu` errors / hangs then errors | HAT not powered, or I²C disabled | Battery in + switch on; `sudo raspi-config nonint do_i2c 0`; check `i2cdetect -y 1` shows `0x14` |
| `permission denied` opening `/dev/i2c-1` or gpiochip | user not in `i2c`/`gpio` groups | `sudo usermod -aG i2c,gpio "$USER"`, then log out/in |
| `unknown pin name` | typo in `--pin`/`--channel` | Use names from [§3.1](../../picar-x-apis.md) (`D0`–`D16`) / `P0`–`P19` / `A0`–`A4` |
| Servo buzzes but won't hold a position | low battery, or mechanical bind | Charge pack; check linkage moves freely by hand |
| Motor silent at `--speed 20` | that's the remap dead zone working as designed | Try `--speed 60`, or `--raw --speed 80` to confirm the PWM path |
| Motor runs the "wrong" way | direction cal / wiring convention | Expected for a raw test — the `motor` command doesn't apply per-motor direction calibration; that lives in the future `picarx.Device` layer (§6.3) |
| Grayscale values all identical / stuck | ran `reset-mcu` just before, or sensor unplugged | Power-cycle the HAT and re-read; check the sensor ribbon |
| `read-battery` reads ~0 or nonsense | ADC poisoned by a prior `reset-mcu` | Power-cycle the HAT, then read without resetting first (§17) |
| `distance` always times out | ECHO floating (no pull-down), sensor unplugged, or nothing within ~3.4 m | Point at a wall < 3 m; check the HC-SR04 cabling on D2/D3 |
| `gpiodetect` shows the header on `gpiochip4`, not `gpiochip0` | early Pi 5 kernel | The binary hardcodes `gpiochip0` in `cmd/picarctl/main.go` — needs a `--chip` flag added (small change) |

**General approach when something fails:** work bottom-up. `blink` (GPIO only) → `ping-mcu` (I²C) → actuators/sensors. If `blink` works but `ping-mcu` fails, it's the I²C bus, not your Go code. If both work but one sensor is dead, it's that sensor's wiring, not the driver.
