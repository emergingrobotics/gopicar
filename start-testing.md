# Start Testing — PiCar-X Go Driver on Real Hardware

How to build `picarctl`, load it onto the PiCar-X, and smoke-test each subsystem on the robot.

> **Scope:** The Milestone 1 subsystems have CLI subcommands — the **I²C bus**, **GPIO**, and **MCU** layers (`ping-mcu`, `hat-info`, `blink`, `reset-mcu`). On top of those M1 primitives, `picarctl` also carries **manual actuator/sensor test commands** (`servo`, `motor`, `stop`, `read-adc`, `read-battery`, `read-grayscale`, `distance`) so you can poke each piece of hardware directly and confirm the Go stack talks to the real robot — see [§6](#6-manual-actuator--sensor-tests). These are thin test tools that drive the MCU registers/GPIO straight from `cmd/picarctl`; the fully-tested `pkg/servo`, `pkg/motor`, `pkg/adc`, … driver packages are still the Milestone 2–7 deliverables ([§7](#7-what-isnt-built-yet)). Section references like §5.4 point to `../picar-x-apis.md`.

---

## 1. Build for the Pi

From the `gopicar/` module root — static binary, no cgo, cross-compiled:

```bash
# Pi 5 / Pi 4 on 64-bit Raspberry Pi OS (the VISION target)
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o picarctl ./cmd/picarctl

# 32-bit Pi OS on Pi 4:        GOARCH=arm GOARM=7
# Pi Zero / Zero 2W (32-bit):  GOARCH=arm GOARM=6
```

Verify the artifact:

```bash
file picarctl
# expect: ELF 64-bit LSB executable, ARM aarch64, statically linked
```

## 2. One-time Pi setup

```bash
ssh pi@raspberrypi.local          # your Pi's hostname / IP

# Enable I²C (Bookworm shown; older OS edits /boot/config.txt instead)
sudo raspi-config nonint do_i2c 0     # or add: dtparam=i2c_arm=on to /boot/firmware/config.txt
sudo reboot

# --- after reboot ---

# Kernel device nodes the binary needs must exist:
ls -l /dev/i2c-1 /dev/gpiochip0

# Ground truth that the HAT answers on the bus (this is what ping-mcu confirms):
sudo apt install -y i2c-tools
i2cdetect -y 1                    # expect a cell at 0x14 (or 0x15 / 0x16)

# Confirm which gpiochip owns the 40-pin header (the binary hardcodes gpiochip0):
gpiodetect                        # gpiochip0 = RP1 (Pi 5) / bcm2711 (Pi 4) header

# Run without sudo — add yourself to the device groups, then log out/in:
sudo usermod -aG i2c,gpio "$USER"
```

> **gpiochip gotcha:** if `gpiodetect` shows the 40-pin header on a chip *other than* `gpiochip0` (some early Pi 5 kernels used `gpiochip4`), the M1 binary — which hardcodes `gpiochip0` in `cmd/picarctl/main.go` — needs a `--chip` flag. Flag it and it becomes a quick change.

## 3. Copy it over

```bash
scp picarctl pi@raspberrypi.local:/home/pi/
```

## 4. Test each subsystem

Run on the Pi, in this order — each step isolates one layer.

### ① GPIO layer — standalone (no HAT / MCU needed)

```bash
./picarctl blink --pin D14 --count 10
```

**Expect:** the on-board **user LED (D14 = GPIO26) blinks 10 times**, then `blinked D14 10 times`.
**Validates:** `gpio.Open` → `RequestOutput` → `SetValue` on the real character device. Uses only GPIO, so it works even if the MCU is dead — cleanly isolating GPIO from I²C.

### ② MCU / HAT identity — device-tree (no I²C)

```bash
./picarctl hat-info
cat /proc/device-tree/hat*/uuid; echo    # cross-check
```

**Expect:** e.g. `HAT V5 / speaker-EN pin: D10 / motor mode: 2 / uuid: 9daeea78-…`, and the printed UUID matches the `/proc` file.
**Validates:** the `hat*` glob and the V4/V5-vs-fallback logic against real EEPROM (§4).

### ③ I²C bus + probe + combined read — the big one

```bash
./picarctl ping-mcu
```

**Expect:** `MCU at 0x14, firmware X.Y.Z` — and the address matches the `i2cdetect` cell from step 2.
**Validates:** the entire bus stack on hardware — fd open → address probe (0x14/0x15/0x16 in order) → the raw `I2C_RDWR` combined write-then-read framing → the retry decorator → the `runtime.KeepAlive` unsafe path (§5.1, §5.2, §15).

### ④ MCU reset — GPIO + recovery

```bash
./picarctl reset-mcu && ./picarctl ping-mcu
```

**Expect:** `MCU reset pulsed (GPIO5 low 10ms → high 10ms)`, then `ping-mcu` **still succeeds**.
**Validates:** the reset pulse (§5.4) restores rather than wedges the bus — the second success is the real assertion. (The §17 ADC-poisoning caveat doesn't apply in M1: no ADC exists yet.)

## 5. Failure-mode tests

These confirm the design's promises actually hold on hardware, not just in unit tests.

```bash
# Power off / unplug the HAT, then:
./picarctl ping-mcu ; echo "exit=$?"
```

**Expect:** a **clean non-zero exit with an error** — not a hang, not a false success. Confirms "propagate errors, bounded retry, no silent fallback to 0x14" (§17).

```bash
# As a user NOT in the i2c group:
./picarctl ping-mcu
```

**Expect:** a clean "permission denied" on open — no panic, no stack trace.

## 6. Manual actuator + sensor tests

These commands drive the hardware directly from `cmd/picarctl` (MCU PWM/ADC registers over I²C, GPIO for the ultrasonic) so you can confirm the Go stack actually moves servos, spins motors, and reads sensors on the real robot. They need a working I²C bus + MCU (run `ping-mcu` first), except `distance`, which is GPIO-only.

> **Safety:** put the car on a stand (wheels off the ground) before the first `motor` test. `motor` auto-stops after `--duration` (default 1 s); `stop` halts both motors immediately.

### Servos (§7)

```bash
./picarctl servo --which steer --angle 0     # center the steering servo (P2, clamped ±30°)
./picarctl servo --which pan   --angle 45    # camera pan (P0)
./picarctl servo --which tilt  --angle -20   # camera tilt (P1)
./picarctl servo --channel P0  --angle 90    # raw channel, clamped to the servo limit ±90°
```

**Expect:** the addressed servo snaps to the angle; `servo P2 → 0.0°`. **Validates:** timer-0 prescaler/period + pulse-width register writes (`WriteBlock`, big-endian [hi,lo]).

### Motors (§6)

```bash
./picarctl motor --which left  --speed 60               # left motor forward 1 s, then auto-stop
./picarctl motor --which right --speed -60              # right motor reverse
./picarctl motor --which left  --speed 40 --raw         # write 40% duty directly (skip 50..100 remap)
./picarctl motor --which right --speed 50 --duration 3s # run 3 s
./picarctl stop                                         # stop both
```

**Expect:** the wheel spins the commanded way for the duration, then `motor left stopped`. **Validates:** timer-3 setup, PWM duty writes, and the direction GPIO (D4/D5).

### Sensors — ADC / battery / grayscale (§5.2, §9)

```bash
./picarctl read-battery        # e.g. battery: raw=3010 7.28 V
./picarctl read-grayscale      # e.g. grayscale [L M R] = [1980 2100 1875]
./picarctl read-adc --channel A3
```

**Expect:** plausible values (battery ~6–8.4 V on a charged pack; grayscale ~1800–2500 over white, ~100–400 over black tape). **Validates:** the ADC 3-byte `[reg,0,0]` + 2-byte read path via the new `bus.Tx`.

### Ultrasonic distance (§8)

```bash
./picarctl distance --count 5
```

**Expect:** `distance: NN.NN cm` per ping; hold your hand in front and watch it drop. Timeout → `echo timeout (...)`. **Validates:** the GPIO trig pulse + kernel-timestamped echo edge timing (no I²C).

## 7. What isn't built yet

The manual test commands above exercise the hardware, but the **fully unit-tested driver packages** (`pkg/servo`, `pkg/adc`, `pkg/motor`, …) and the `picarx.Device` control API are still per-milestone deliverables. Each milestone also formalizes its acceptance command.

| Milestone | Subsystem | Driver package + acceptance | Manual test command today |
|-----------|-----------|-----------------------------|---------------------------|
| 1 (done)  | Bus / GPIO / MCU | done | `ping-mcu`, `hat-info`, `blink`, `reset-mcu` |
| 2 | Servos | `pkg/servo`, `sweep-servo`/`center` | `servo` ✓ |
| 3 | ADC / grayscale / battery | `pkg/adc`, `pkg/grayscale` | `read-adc`, `read-battery`, `read-grayscale` ✓ |
| 4 | Motors | `pkg/motor`, `drive` | `motor`, `stop` ✓ |
| 5 | Ultrasonic | `pkg/ultrasonic`, `ping-distance` | `distance` ✓ |
| 6 | Config round-trip | `calibrate`, `dump-config` | — |
| 7 | Full demo | `drive-square` | — |

---

## Quick reference

```bash
# build (Pi 5 / Pi 4 64-bit)
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o picarctl ./cmd/picarctl
scp picarctl pi@raspberrypi.local:/home/pi/

# on the Pi — full M1 smoke test
./picarctl blink --pin D14 --count 10   # ① GPIO
./picarctl hat-info                     # ② HAT identity
./picarctl ping-mcu                     # ③ I²C + MCU
./picarctl reset-mcu && ./picarctl ping-mcu   # ④ reset + recovery
```

Milestone 1 passes when ①–④ all produce correct output on the robot.
