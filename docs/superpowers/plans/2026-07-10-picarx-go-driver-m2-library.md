# PiCar-X Go Driver — Milestone 2: Reusable Library + Full picarx Parity

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn `gopicar` from a CLI-with-primitives into a **fully reusable Go module** that external programs import to drive a PiCar-X — with `picarctl` demoted to a working reference example under `./examples/picarctl`. Today every piece of PiCar-X domain logic (servo, motor, ADC, ultrasonic, calibration) lives in `package main` under `cmd/picarctl/` and is therefore un-importable. This milestone lifts that logic into importable packages, reaches **full stock-picarx feature parity**, and ships godoc + runnable examples + tests.

**Architecture:** A high-level facade package `pkg/picarx` (`Open/Close`, `Servo`, `Motor`, `Battery`, `Distance`, `Grayscale`, movement helpers) built on **composable low-level packages** (`pkg/pwm`, `pkg/adc`, `pkg/servo`, `pkg/motor`, `pkg/ultrasonic`), all layered on the existing M1 foundation (`pkg/bus`, `pkg/gpio`, `pkg/mcu`). Downward-only dependencies. Every layer above the hardware wrappers is unit-tested against `internal/fake`. Hardware-touching integration tests are gated behind a `//go:build hardware` tag so `go test ./...` stays hardware-free.

**Tech Stack:** Go 1.25, `golang.org/x/sys/unix`, `github.com/warthog618/go-gpiocdev` v0.9.1. No cgo; cross-compiles to `linux/arm64` as a single static binary.

**Reference:** `picar-x-apis.md` — `§` citations throughout point there. Prior work: `docs/superpowers/plans/2026-07-06-picarx-go-driver-m1.md` (M1: bus/gpio/mcu foundation).

---

## Decisions locked for M2 (from brainstorm 2026-07-10)

1. **API layout:** Facade **+** granular. `pkg/picarx` is the easy entry point; `pkg/{pwm,adc,servo,motor,ultrasonic}` are independently importable.
2. **Scope:** **Full stock-picarx parity** (see Feature Matrix) — not just today's `picarctl` smoke-test set.
3. **Calibration:** **Programmatic only.** The library exposes a `Calibration` value passed via `Options`; the library performs **no filesystem or env access**. Persistence (the `~/.config/gopicar/calibration.json` file we built) is re-homed into the `examples/picarctl` `main` package as the reference "bring-your-own-storage" implementation. *(Reconciliation note: this keeps the library pure while `picarctl` stays fully functional. See Task 5.3.)*
4. **Testing:** Unit tests with fakes for all logic; **fakes stay in `internal/`** (not exported for downstream); hardware integration tests behind `//go:build hardware`.

## Global Constraints

Every task implicitly includes these:

- **Module path:** migrating `github.com/gherlein/gopicar` → **`github.com/emergingrobotics/gopicar`** (repo owner change, done by maintainer on GitHub separately). Task 0.1 flips `go.mod` + all imports up front so all new code uses the final path. There are currently **11** Go files referencing the old path.
- **Go version:** 1.25. Runtime deps unchanged (`x/sys/unix`, `go-gpiocdev`). No new runtime dependencies without explicit sign-off.
- **Libraries are pure:** packages under `pkg/` **never** write to `os.Stdout`/`os.Stderr`, **never** call `os.Exit`, **never** read env vars or files. They return values + errors. All formatting/printing/persistence lives in `examples/`.
- **Context on every hardware-touching method** (already true of `bus`/`mcu`); new methods that do I/O take `context.Context` as the first argument.
- **Big-endian, explicit bytes:** 16-bit register values go on the wire as explicit `[hi, lo]`; never a word-write helper (§5.3, §17).
- **Errors propagate:** never the Python "retry-5-and-move-on" swallow (§17).
- **ADC read protocol:** the fixed sequence — write `[reg,0,0]` as its own transaction (STOP), then two **bare single-byte reads** — is canonical and lives in `pkg/adc`. The combined repeated-START read is a known-bad path on this firmware (HAT V4 / fw 2.1.1); do not reintroduce it (§5.2).
- **No global mutable runtime state.** Test seams (function vars like `pkg/bus.rdwr`) may exist but are overridden only in tests.
- **`mcu.Reset()` warning** preserved: resetting poisons live ADC state (§17).
- **Keep the tree green:** after every task, `go build ./...` and `go test ./...` pass. `picarctl` (wherever it currently lives) must keep working through the whole migration.

**Per-task commands** (run from `gopicar/`):
- Build: `go build ./...`
- Test all (hardware-free): `go test ./...`
- Test on the Pi: `go test -tags hardware ./...`
- Vet + docs check: `go vet ./...` ; `go doc ./pkg/picarx`

---

## Feature Matrix — full picarx parity

Target public capabilities (facade methods in `pkg/picarx`, backed by granular packages). "Stock ref" cites the SunFounder `picarx.py` method the behavior mirrors.

| Area | Capability | Granular pkg | Stock ref |
|---|---|---|---|
| Servo | `Servo(name).Angle(userDeg)` calibrated; `.SetRaw(deg)`; center | `pkg/servo` | `set_*_servo_angle` |
| Steering | `SetDir(angleDeg)` (calibrated steer) | `pkg/servo` | `set_dir_servo_angle` |
| Camera | `SetCamPan(deg)`, `SetCamTilt(deg)` | `pkg/servo` | `set_cam_pan_angle`, `set_cam_tilt_angle` |
| Motor | `Motor(name).Speed(pct)`; `.Stop()`; raw duty | `pkg/motor` | `set_motor_speed` |
| Drive | `Forward(pct)`, `Backward(pct)`, `Stop()` | `pkg/motor` | `forward`/`backward`/`stop` |
| Drive | `SetMotorSpeedCalibration` (per-motor trim) | `pkg/motor` | `motor_speed_calibration` |
| Drive helper | `Ramp(from,to,dur)` speed ramping | `pkg/picarx` | *(new helper)* |
| ADC | `ReadADC(ch)`, `ADCVoltage(ch)` | `pkg/adc` | `Grayscale`/`ADC` classes |
| Power | `Battery()` volts | `pkg/adc` | battery divider ×3 (§5.2) |
| Grayscale | `Grayscale() [3]int` raw | `pkg/adc` | `get_grayscale_data` |
| Grayscale | `LineStatus(ref)` → left/center/right | `pkg/picarx` | `get_line_status` |
| Grayscale | `CliffStatus(ref)` → bool | `pkg/picarx` | `get_cliff_status` |
| Distance | `Distance()` cm (with count/timeout opts) | `pkg/ultrasonic` | `get_distance` / `Ultrasonic` |
| Identity | `FirmwareVersion()`, `HAT()` | (via `mcu`) | — |
| Recovery | `Reset(ctx)` (with ADC-poison warning) | (via `mcu`) | `reset` |
| GPIO | `LED`/`Blink` user-LED helper | `pkg/gpio` | `Pin`/LED |

---

## Target directory layout (after M2)

```
gopicar/
  go.mod                     # module github.com/emergingrobotics/gopicar
  README.md                  # rewritten: library-first, install, quickstart, example
  pkg/
    bus/      gpio/   mcu/    # M1 foundation (minor: bus.Open return-type fix)
    pwm/                      # NEW: timer + duty math (servo/motor PWM)
    adc/                      # NEW: fixed ADC read + voltage + battery/grayscale reads
    servo/                    # NEW: Servo{} over pwm, calibration apply
    motor/                    # NEW: Motor{} over pwm + gpio dir pin, speed remap
    ultrasonic/              # NEW: Distance over gpio trig/echo
    picarx/                   # NEW: facade Open/Close + high-level helpers + Calibration
      doc.go                  # package overview for godoc
      example_test.go         # runnable Example_* funcs
  internal/
    fake/                    # extended: ADC bare-read queue, ultrasonic edges
  examples/
    picarctl/                # MOVED from cmd/picarctl; package main; on top of pkg/picarx
      main.go ...            # thin CLI
      calibstore.go          # example-local JSON persistence (was config.go)
  docs/superpowers/...        # specs + plans (this file)
```

`cmd/` is removed. `bin/picarctl` build output stays git-ignored.

---

## Public API sketch (authoritative for the tasks below)

```go
// pkg/picarx
type PiCarX struct { /* owns bus, chip, mcu; unexported */ }

type Options struct {
    I2CDev      string       // default "/dev/i2c-1"
    GPIOChip    string       // default "gpiochip0"
    Calibration Calibration  // programmatic; zero value = neutral (all trims 0, dir +1, ±90)
    Wiring      Wiring        // channel/pin map; zero value = stock SunFounder defaults
    Retry       bus.RetryPolicy
}
func Open(ctx context.Context, opts Options) (*PiCarX, error)
func (p *PiCarX) Close() error

// servos (calibrated, user-facing angles: +right/+up, 0 = centered)
func (p *PiCarX) SetDir(deg float64) error
func (p *PiCarX) SetCamPan(deg float64) error
func (p *PiCarX) SetCamTilt(deg float64) error
func (p *PiCarX) Servo(name string) *servo.Servo   // escape hatch to granular

// motors
func (p *PiCarX) Forward(pct float64) error
func (p *PiCarX) Backward(pct float64) error
func (p *PiCarX) Stop() error
func (p *PiCarX) Motor(name string) *motor.Motor

// sensors
func (p *PiCarX) Battery(ctx context.Context) (float64, error)
func (p *PiCarX) Grayscale(ctx context.Context) ([3]int, error)
func (p *PiCarX) LineStatus(ctx context.Context, ref [3]int) (LineStatus, error)
func (p *PiCarX) CliffStatus(ctx context.Context, ref [3]int) (bool, error)
func (p *PiCarX) Distance(ctx context.Context) (float64, error)

// identity / recovery
func (p *PiCarX) FirmwareVersion(ctx context.Context) (maj, min, pat uint8, err error)
func (p *PiCarX) HAT() mcu.HAT
func (p *PiCarX) Reset(ctx context.Context) error  // WARNING: poisons ADC (§17)

// calibration (programmatic; no file I/O)
type ServoCal struct { Channel uint8; Trim, Dir, Min, Max float64 }
type Calibration struct { Steer, Pan, Tilt ServoCal; MotorTrim map[string]float64 }
func NeutralCalibration() Calibration            // zero-trim, safe defaults
func MeasuredCalibration() Calibration           // the 2026-07-10 values for THIS robot
```

Granular packages expose constructors that take a `bus.Bus` + `mcu` address (+ `gpio.Chip` where needed), so they are usable standalone without the facade.

---

## Phase 0 — Module path migration & hygiene

### Task 0.1: Rename module path gherlein → emergingrobotics
- [ ] `go mod edit -module github.com/emergingrobotics/gopicar`
- [ ] Rewrite the import path in all Go files: `grep -rl github.com/gherlein/gopicar --include=*.go | xargs sed -i 's#github.com/gherlein/gopicar#github.com/emergingrobotics/gopicar#g'`
- [ ] `go build ./... && go test ./...` green.
- **DoD:** no reference to `gherlein` remains in `.go` files; module builds under the new path. *(GitHub org move is manual/out-of-band.)*

---

## Phase 1 — Extract pure compute packages (no behavior change)

Move logic out of `cmd/picarctl/hw.go` into importable packages. `picarctl` keeps working by calling the new packages.

### Task 1.1: `pkg/pwm` — timer + duty math
- **Files:** create `pkg/pwm/pwm.go`, `pkg/pwm/pwm_test.go`.
- **Move from `hw.go`:** `timerIndex`, `writePWM16`, `setTimer`, `setServoAngle`→`SetServoAngle`, `setupMotorTimer`, `setPWMPercent`→`SetDutyPercent`, servo/motor timer constants (`servoPeriodARR`, `servoPSCReg`, `motorPeriodARR`, `motorPSCReg`).
- **Interface:** `func SetServoAngle(ctx, b bus.Bus, addr, ch uint8, angleDeg float64) error`; `func SetDutyPercent(ctx, b, addr, ch uint8, pct float64) error`; exported `PWMInputClock` reused from `mcu`.
- **Tests:** golden byte-trace via `internal/fake`: angle 0 → `0x22,0x01,0x33`; timer ARR/PSC writes `0x44/0x40`; duty% boundaries (0/50/100). Assert byte-for-byte match to reference (§7.1–7.2).
- **DoD:** package builds, tests pass, `picarctl` updated to call `pwm.*`.

### Task 1.2: `pkg/adc` — fixed ADC read + conversions
- **Files:** create `pkg/adc/adc.go`, `pkg/adc/adc_test.go`.
- **Move from `hw.go`/`sensors.go`:** `readADC` (the FIXED 3-transaction sequence), `adcVoltage`→`Voltage`, battery (`Battery`) and grayscale (`Grayscale`) read helpers. `parseADCChannel` stays near CLI (or expose `ParseChannel`).
- **Interface:** `type ADC struct{ b bus.Bus; addr uint8 }`; `New(b, addr) *ADC`; `(*ADC) Read(ctx, ch uint8) (int, error)`; `Voltage(ctx, ch) (float64, error)`; `Battery(ctx) (float64, error)`; `Grayscale(ctx) ([3]int, error)`.
- **Tests:** extend `internal/fake` with a **bare-read FIFO** (Task 1.3) so a unit test can assert `Read` emits `WriteBlock([reg,0,0])` + two 1-byte `Tx(nil,1)` reads and reassembles MSB/LSB. Battery = `Voltage(A4)*3`.
- **DoD:** `read-battery` still returns ~8.3 V on hardware; unit test locks the transaction shape.

### Task 1.3: Extend `internal/fake` for ADC + ultrasonic
- **Files:** edit `internal/fake/bus.go`; edit/create `internal/fake/chip.go`.
- **Add:** per-address bare-read FIFO (`ReadQueue map[uint8][]byte`) so `Tx(addr, nil, n)` returns queued bytes (currently returns zeros); helper `EnqueueRead(addr, bytes...)`. For ultrasonic: chip edge-injection already exists (`InjectEdge`) — verify it supports echo-pulse timing simulation.
- **DoD:** existing fake tests still pass; new helpers covered by a fake self-test.

---

## Phase 2 — Extract device packages

### Task 2.1: `pkg/servo` — Servo type + calibration apply
- **Files:** create `pkg/servo/servo.go`, `pkg/servo/servo_test.go`.
- **Content:** `type Servo struct{ b bus.Bus; addr, ch uint8; cal ServoCal }`; `New(b, addr, ch, cal)`; `(*Servo) Angle(ctx, userDeg) error` (applies `raw = Trim + Dir*clamp(user,Min,Max)`, clamps raw ±90, calls `pwm.SetServoAngle`); `SetRaw(ctx, deg)`; `Center(ctx)`. Move `ServoCal.apply` math here (pure, unit-tested without hardware).
- **Tests:** table-test the calibrated mapping for steer/pan/tilt using the measured values (0→−58/−11/+25; steer +20→−38; clamping at Min/Max and ±90).
- **DoD:** calibration math has full unit coverage; no file/env access.

### Task 2.2: `pkg/motor` — Motor type + speed logic
- **Files:** create `pkg/motor/motor.go`, `pkg/motor/motor_test.go`.
- **Move from `motor.go`:** `motorWiring`, `driveMotor`→`(*Motor).Speed`, `stopMotor`→`(*Motor).Stop`, the `speed/2+50` remap and `--raw` path, direction-pin logic. Add per-motor calibration trim + optional direction invert.
- **Interface:** `New(b, addr, chip gpio.Chip, w Wiring, trim float64)`; `(*Motor) Speed(ctx, pct float64, raw bool) error`; `Stop(ctx) error` (double-write defensive stop, §6.5).
- **Tests:** remap boundaries (0→0, 1→50.5→? per stock, 100→100), sign→dir-pin, double-stop trace.
- **DoD:** motor tests pass; `picarctl motor` unchanged behavior.

### Task 2.3: `pkg/ultrasonic` — Distance
- **Files:** create `pkg/ultrasonic/ultrasonic.go`, `pkg/ultrasonic/ultrasonic_test.go`.
- **Move from `ultrasonic.go`:** trig pulse + echo-timing → cm, count/timeout options.
- **Interface:** `New(chip gpio.Chip, trigPin, echoPin string)`; `(*U) Distance(ctx, opts) (float64, error)`.
- **Tests:** fake chip edge injection → known pulse width → known cm; timeout path returns sentinel/err.
- **DoD:** distance logic unit-tested against fake edges; hardware path deferred to Phase 5 gated test.

---

## Phase 3 — Facade + high-level parity features

### Task 3.1: `pkg/picarx` core — Open/Close/Options/Wiring/Calibration
- **Files:** create `pkg/picarx/picarx.go`, `pkg/picarx/calibration.go`, `pkg/picarx/wiring.go`, `pkg/picarx/doc.go`.
- **Content:** `Open` builds bus→retry→mcu→chip (reuse the logic currently in `cmd/picarctl/main.go openStack`), constructs `servo/motor/adc/ultrasonic` device objects from `Wiring`; `Close` releases chip+bus. `Calibration`, `ServoCal`, `Wiring` types (programmatic). `NeutralCalibration()` and `MeasuredCalibration()` (the 2026-07-10 values). Stock wiring defaults: pan P0, tilt P1, steer P2, motors P13/P12, dir D4/D5, ultrasonic pins per §.
- **Tests:** `Open` against a fully faked stack (fake bus+chip); `Close` idempotent.
- **DoD:** facade constructs and tears down with zero hardware.

### Task 3.2: `pkg/picarx` servo + motor + drive methods
- [ ] `SetDir`, `SetCamPan`, `SetCamTilt`, `Servo(name)` escape hatch.
- [ ] `Forward`, `Backward`, `Stop`, `Motor(name)`, per-motor calibration.
- [ ] `Ramp(ctx, from, to, dur)` speed-ramp helper (pure timing loop; ctx-cancellable).
- **Tests:** facade delegates to device objects (assert byte traces); `Ramp` steps monotonically and honors ctx cancel.

### Task 3.3: `pkg/picarx` sensor + interpretation methods
- [ ] `Battery`, `Grayscale`, `Distance` passthrough.
- [ ] `LineStatus(ctx, ref [3]int) (LineStatus, error)` — port `get_line_status` threshold logic (§9).
- [ ] `CliffStatus(ctx, ref [3]int) (bool, error)` — port `get_cliff_status`.
- **Tests:** table-tests for line/cliff interpretation across sensor combinations vs. reference thresholds.
- **DoD:** parity behaviors match the documented stock semantics.

### Task 3.4: `pkg/picarx` identity/recovery passthrough
- [ ] `FirmwareVersion`, `HAT`, `Reset` (carry the ADC-poison warning in the doc comment).
- **DoD:** facade covers the full Feature Matrix.

---

## Phase 4 — Re-home the CLI as an example

### Task 4.1: Move `cmd/picarctl` → `examples/picarctl`
- [ ] `git mv cmd/picarctl examples/picarctl`; remove empty `cmd/`.
- [ ] Update any build scripts / `Makefile` targets and `.gitignore` (`bin/` path).
- **DoD:** `go build ./examples/picarctl` produces the binary; tree green.

### Task 4.2: Rewrite `picarctl` on top of `pkg/picarx`
- [ ] Replace direct `hw.go`/`servo.go`/`motor.go`/`sensors.go` logic with calls into `pkg/picarx` + granular packages. Delete the now-duplicated `hw.go` etc. from the example (logic now lives in `pkg/`).
- [ ] Keep every existing subcommand + flag (`ping-mcu`, `hat-info`, `blink`, `reset-mcu`, `servo`, `motor`, `stop`, `read-adc`, `read-battery`, `read-grayscale`, `distance`, `calibrate`) behaving identically.
- **DoD:** `examples/picarctl --help` and each command match current behavior; `scripts/test-all.sh` still valid (update path to `./examples/picarctl/...` or built binary).

### Task 4.3: Example-local calibration persistence (`calibstore.go`)
- [ ] Move the JSON load/save + `~/.config/gopicar/calibration.json` path resolution (env/XDG) out of the library into `examples/picarctl/calibstore.go` (package main). It maps file ⇄ `picarx.Calibration`.
- [ ] `calibrate show/set/save-defaults` operate on this example-local store, then pass the resulting `picarx.Calibration` into `picarx.Open`.
- **DoD:** persistence works exactly as today, but is demonstrably *example code*, not library API. `go doc ./pkg/...` shows no filesystem/env surface.

---

## Phase 5 — Testing

### Task 5.1: Unit coverage sweep
- [ ] Ensure each new package has table/trace tests (pwm math, adc transaction shape, servo calibration, motor remap, ultrasonic timing, line/cliff interpretation, facade delegation).
- [ ] `go test ./...` fast and hardware-free; target meaningful coverage on pure logic (calibration/remap/interpretation ~100%).

### Task 5.2: Gated hardware integration tests
- [ ] Create `pkg/picarx/hw_integration_test.go` with `//go:build hardware`.
- [ ] Tests (run only with `-tags hardware` on the Pi): `Open`→`FirmwareVersion` non-zero; `Battery` in a plausible 6–8.5 V band; `SetDir(0)` then read-back-free "no error"; ultrasonic `Distance` returns >0 with a target; `Reset` warning path. Include a skippable "servo sweep" behind an extra env (`GOPICAR_HW_MOVE=1`) so CI-less manual runs don't thrash servos unexpectedly.
- **DoD:** `go test ./...` never touches hardware; `go test -tags hardware ./...` exercises the real stack.

### Task 5.3: Migration regression check
- [ ] Diff old vs new `picarctl` output for every subcommand on hardware; confirm identical.
- [ ] Re-run `scripts/test-all.sh` end-to-end.

---

## Phase 6 — Documentation

### Task 6.1: godoc doc-comments on all exported symbols
- [ ] Every exported type/func/const in `pkg/{pwm,adc,servo,motor,ultrasonic,picarx}` has a doc comment; `pkg/picarx/doc.go` gives a package overview with a quickstart snippet. Carry `§` citations where they aid maintenance.
- [ ] `go vet ./...` clean; spot-check `go doc ./pkg/picarx`.

### Task 6.2: Runnable examples
- [ ] `pkg/picarx/example_test.go` with `Example_open`, `Example_setDir`, `Example_battery` (compile-checked; use build tag or fake so they don't require hardware to `go test`). These render in godoc.

### Task 6.3: Rewrite top-level `README.md`
- [ ] Library-first: what it is, install (`go get github.com/emergingrobotics/gopicar/pkg/picarx`), 10-line quickstart, package map, calibration explainer (programmatic + how the example persists), link to `examples/picarctl`, hardware-test instructions, module-path/org note.

---

## Phase 7 — Cleanup & final verification

### Task 7.1: Fix `bus.Open` return type wart
- [ ] `bus.Open` currently returns unexported `*i2cBus`. Change to return the `Bus` interface (or an exported concrete type) so external callers can name the result. Update call sites.
- **DoD:** external pseudo-consumer (a throwaway `_test` in `pkg/picarx`) can declare the returned type.

### Task 7.2: Dead-code & lint pass
- [ ] Remove leftovers from the old `cmd/picarctl` logic now living in `pkg/`. `go vet ./...`, `gofmt -l` clean.

### Task 7.3: Final acceptance
- [ ] `go build ./...`, `go test ./...`, `go test -tags hardware ./...` all green on the Pi.
- [ ] A ~15-line standalone program in a scratch module (outside this repo) imports `github.com/emergingrobotics/gopicar/pkg/picarx`, opens the robot, centers steering, reads battery — compiles and runs. This is the real "is it a usable module?" acceptance gate.

---

## Definition of Done (milestone)

- External Go programs can `go get` the module and drive a PiCar-X via `pkg/picarx` (facade) or the granular packages — **no code lives in `package main` that a consumer needs.**
- Full picarx feature parity per the Feature Matrix.
- Calibration is programmatic in the library; `examples/picarctl` demonstrates file persistence.
- `picarctl` works identically to today, as a reference example under `examples/picarctl`.
- Unit tests hardware-free and green; hardware tests gated and green on the Pi.
- godoc complete; README rewritten; runnable examples present.
- Module path is `github.com/emergingrobotics/gopicar` throughout.

## Risks / open questions

- **R1 — Ultrasonic pin mapping & timing:** confirm trig/echo HAT pins and that fake edge-injection can model echo width precisely enough for a meaningful unit test (§ ultrasonic). If not, lean on the gated hardware test and keep the unit test to the math only.
- **R2 — Grayscale line/cliff thresholds:** stock uses a caller-supplied reference list; confirm we expose `ref` rather than hardcoding, and document how to obtain it (calibration routine could come in a later milestone).
- **R3 — Motor remap fidelity:** the `speed/2+50` gearbox remap and per-motor trim interaction needs a decided precedence (trim before or after remap). Propose: apply trim to the pre-remap magnitude; lock in Task 2.2.
- **R4 — `MeasuredCalibration()` in the library:** baking this robot's specific numbers into a library constant is a convenience but is per-unit. Keep it clearly named/documented as "example values for the author's unit," or move it to the example. **Decision needed** (default: keep as documented convenience in `pkg/picarx`, mirrored by the example's default file).
- **R5 — `scripts/test-all.sh` path:** update to the built binary path after the move; consider a `Makefile` `example` target.
```
