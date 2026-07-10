# PiCar-X Go Driver — Milestone 1 Design

**Date:** 2026-07-06
**Status:** Approved (design), pending implementation plan
**Scope:** Milestone 1 of the `VISION.md` roadmap — the `bus`, `gpio`, `mcu`, and `internal/fake` foundation, plus a minimal `picarctl`.
**Acceptance test:** Talk to the HAT over I²C, read its UUID, and blink the user LED (D14).
**Module root:** All code lives under `gopicar/` (the Go module root; `go.mod` lives here). Package paths below (`pkg/bus`, `cmd/picarctl`, `internal/fake`, …) are relative to that root. Module import path is TBC with the maintainer (default assumption: `github.com/gherlein/gopicar`) and will be pinned in the implementation plan.

This document specs Milestone 1 only. `VISION.md` remains the umbrella architecture; each later milestone (servos, ADC/grayscale, motors, ultrasonic, config, CLI demo) gets its own spec → plan → build cycle. The reference for all wire-level behavior is `picar-x-apis.md` (section citations below use its `§` numbers).

---

## Umbrella decisions (cross-cutting, ratified during brainstorming)

These four decisions apply to the whole driver. Three of them are load-bearing at Milestone 1 because they live in `bus`/`mcu`; the fourth is recorded here so later milestones inherit it.

1. **Context on every `Bus` method.** `Bus` methods take `context.Context` so the 5-attempt exponential-backoff retry loop is cancellable. This fully honors VISION Goal 3 ("context cancellation on every long-running op") — the retry backoff *is* a long-running op. The blocking `ioctl` syscall itself cannot be interrupted mid-flight, but a single I²C transaction is ~500 µs; `ctx` is checked before starting and between retry attempts, which is the meaningful cancellation point.

2. **`Device.Recover(ctx)` owns MCU recovery.** §17's ADC-poisoning trap means a `Reset()` while ADC objects are live poisons them to `[2571, 3085, 3599]` forever. Resolution: callers never hold raw `adc.Device` references — they reach ADC only through `picarx.Device` methods. `Device.Recover(ctx)` pulses reset **and** rebuilds its internal ADC/grayscale objects atomically under lock, so no stale handle can survive a reset. `mcu.Reset()` is a low-level primitive documented as unsafe to call directly after init. *(Implemented in later milestones; the `Reset()` contract shipped in M1 must state this.)*

3. **`picarx.Device` is fully goroutine-safe.** Real callers run a control loop calling `Forward()` while another goroutine calls `Ping()` or reads sensors. The `bus` mutex serializes all I²C; `Ping()` uses GPIO edges (no I²C) so it runs concurrently with motor/servo writes; `Recover()` takes a write-lock that excludes everything. *(Foundation — the `bus` mutex — ships in M1; the Device-level locking ships with `picarx`.)*

4. **Motor reversal ramp is deferred, not dropped.** §18.3 recommends the justinbetabox gearbox-protection ramp (5 %/10 ms, force zero-crossing on direction change). It is out of scope for M1 (motors are Milestone 4) and will be decided in the motor milestone spec.

---

## I²C access mechanism

`pkg/bus` talks to `/dev/i2c-1` via **raw `ioctl` through `golang.org/x/sys/unix`** — `I2C_SLAVE` for addressing and `I2C_RDWR` for combined transactions.

Rationale:
- Pure Go, no cgo → cross-compiles from any host to the Pi as a single static binary (VISION Goal 2). `x/sys/unix` is a std-adjacent dependency, not a runtime dep beyond `libc`.
- `I2C_RDWR` gives repeated-START combined write-then-read, which the ADC 3-byte protocol (§5.2, Milestone 3) requires.
- Direct control over the exact bytes on the wire — this is what makes the golden-trace testing strategy (VISION §"Testing strategy") possible byte-for-byte.

Rejected: `periph.io/x/conn/i2c` (heavier tree, own device model to adapt); thin SMBus-style wrappers like `d2r2/go-i2c` (risk of replicating the little-endian word-order trap §5.3 warns about instead of doing raw block I/O).

---

## §1 — `pkg/bus`

### Interface

```go
type Bus interface {
    WriteBlock(ctx context.Context, addr, reg uint8, data []byte) error
    ReadBlock(ctx context.Context, addr, reg uint8, n int) ([]byte, error)
    WriteRawByte(ctx context.Context, addr, v uint8) error
}
```

- `WriteBlock` → `S addr W reg data… P` (single write message).
- `ReadBlock` → `S addr W reg  Sr addr R n-bytes P` (one `I2C_RDWR` call, two messages, repeated START).
- `WriteRawByte` → `S addr W v P`.

### Concrete implementation

- One `*i2cBus` type owns the `/dev/i2c-1` file descriptor and a `sync.Mutex`. `Open(path string) (*i2cBus, error)` opens the fd; `Close() error` closes it.
- Every public method routes through a single private `xfer()` helper that builds the `[]unix.I2CMsg` slice and issues the `I2C_RDWR` ioctl. The mutex is taken in `xfer()` — this single serialization point is what makes the entire stack above it goroutine-safe.
- **16-bit values are never written through a word helper.** All multi-byte register writes pass explicit `[hi, lo]` byte slices (big-endian on the wire, §5.3, §17). There is deliberately no `WriteWord` method that could reorder bytes.

### Retry decorator

- `retryBus` wraps any `Bus` and implements the same interface. Tests inject a zero-retry base bus directly.
- Policy: **5 attempts** on `EIO`/`ENXIO`, exponential backoff starting at **1 ms**, matching the Python reference (§5.1). `ctx` is checked before each attempt and the backoff `time.After` races against `ctx.Done()`.
- On exhaustion it **returns the final error** — never the Python "5-retry-and-move-on" swallow (§17, VISION "What we're deliberately not doing").

### Address probing

- Helper probes `0x14`, `0x15`, `0x16` in order (§5.1) via a minimal firmware-version read (register `0x05`, §15); the first address that ACKs wins. Result is cached (probing happens once, at `mcu.Open()` time). If none respond, return an explicit error (not a silent fall-back to `0x14`).

### Refinement to VISION flagged for M3

The ADC read (§5.2) is a 3-byte write (`[reg, 0, 0]`) followed by a repeated-START 2-byte read — more than `ReadBlock`'s single reg byte. Rather than bend `ReadBlock`, Milestone 3 will add one combined method (e.g. `Tx(ctx, addr, w []byte, readN int) ([]byte, error)`) to the interface. Recording it here so the interface growth is expected, not a surprise.

---

## §2 — `pkg/gpio`

- `Chip` wraps `go-gpiocdev` (warthog618) on `/dev/gpiochip0` — libgpiod v2 character device only, no sysfs, no mmap (VISION Goal 4). `Open(path)` / `Close()`.
- `Chip` is an **interface** so `internal/fake` substitutes it wholesale.
- `Pin` supports:
  - `RequestOutput(initial bool)`
  - `RequestInput(bias Bias)` — pull-up / pull-down / none via libgpiod v2 bias flags; no-pull = "as-is" (matches lgpio semantics, VISION `pkg/gpio`).
  - `Write(bool)`, `Read() bool`
  - `WatchEdges(edge Edge, handler func(LineEvent))` — hands back a **kernel-timestamped** `LineEvent` stream (the basis for accurate ultrasonic in M5).
- Pin-name resolution: a static map from HAT names (`D0..D16`) and aliases (`MCURST=5`, `LED=26`, `SW=25`, `RST=16`, …, §3.1) to BCM GPIO offsets on `gpiochip0`.

---

## §3 — `pkg/mcu`

### HAT detection

- Glob `/proc/device-tree/hat*/uuid` — the directory name only *contains* "hat" (§4), so a hardcoded `/proc/device-tree/hat/uuid` is wrong. Read the `uuid` file and compare against the known V5 UUID (`9daeea78-0000-076e-0032-582369ac3e02`, §4). Match → `HATv5`; anything else (including absent tree) → `HATv4` fallback.
- Returns:

```go
type HAT struct {
    Version      HATVersion // HATv4 | HATv5
    SpeakerENPin string     // "D15"/GPIO20 (V4) | "D10"/GPIO12 (V5)  (§4, §11)
    MotorMode    int        // 1 (TC1508S, V4) | 2 (TC618S, V5)       (§4, §6)
}
```

- The detection root is **injectable** (default `/`) so unit tests point it at a fixture device-tree directory.

### Reset

- `Reset(ctx)` pulses `MCURST` (GPIO5) **LOW ≥10 ms → HIGH ≥10 ms** (§5.4).
- Documented contract, verbatim in the doc comment: *"Resetting the MCU poisons any ADC state established beforehand (§17) — reads return `[2571, 3085, 3599]` until ADC objects are rebuilt. After init, reach reset only through `picarx.Device.Recover`, never directly."*
- The 10 ms sleeps go through an **injectable clock** (a `sleep func(time.Duration)` or small clock interface) so unit tests assert the LOW→HIGH sequence without burning 20 ms of wall time.

### Register constants

All MCU register/address constants live here as the single source of truth, imported by every other package (§5.3, §15): `REG_CHN=0x20`, `REG_PSC=0x40`, `REG_ARR=0x44`, `REG_PSC2=0x50`, `REG_ARR2=0x54`, the ADC read registers (`0x13`–`0x17`), firmware-version `0x05`, `CLOCK=72_000_000`, and the probe addresses `0x14/0x15/0x16`.

### Open

- `Open(ctx, bus, chip) (*MCU, error)` runs HAT detection and address probing (caching the active address on the bus), and returns a handle exposing `Reset`, `HAT()`, and `FirmwareVersion(ctx)`. It does **not** auto-reset — the reset ordering relative to ADC construction is `picarx.Device`'s responsibility (§13), enforced in a later milestone.

---

## §4 — `internal/fake`

- `fakeBus` implements `bus.Bus`. It **records every transaction** as `{Addr uint8, Write []byte, Read []byte}` for golden-trace assertions, serves responses from a canned table keyed by `(addr, reg)`, and supports error injection (e.g. return `EIO` for the first N calls to exercise the retry decorator).
- `fakeChip` implements the gpio `Chip` interface with in-memory per-pin state (`Read`/`Write`) plus `InjectEdge(pin string, edge Edge, tsNanos int64)` to drive `WatchEdges` handlers. `InjectEdge` is unused in M1 but defined now so M5 ultrasonic edge timing is testable without hardware.
- A device-tree fixture helper produces temp directories matching the `hat*/uuid` layout for HAT-detection tests (V4-fallback and V5-match cases).

---

## §5 — `cmd/picarctl` (Milestone 1 subset)

| Subcommand | Effect |
|---|---|
| `ping-mcu` | Open bus, probe address, read firmware version (`0x05`, §15); print active address + version. |
| `hat-info` | Print detected HAT version, speaker-EN pin, motor mode, and UUID. |
| `blink [--pin D14] [--count N]` | Blink the user LED (D14 / GPIO26, §3.1). Default `--pin D14`. |
| `reset-mcu` | Pulse the reset sequence; `--help` carries the ADC-poisoning warning. |

Later milestones add `read-battery`, `sweep-servo`, `pulse-motor`, and the drive-a-square demo.

---

## §6 — Testing

All Milestone 1 unit tests run **hardware-free** via `internal/fake`:

- **Retry logic:** inject `EIO` → assert exactly 5 attempts, exponential backoff schedule, and that a cancelled `ctx` aborts mid-backoff with the ctx error.
- **Golden byte-framing:** assert the exact `Write`/`Read` byte sequences `fakeBus` records for `WriteBlock`, `ReadBlock`, `WriteRawByte`, and for `ping-mcu` end-to-end. Any accidental byte-order or framing change breaks the test (§5.3, §17).
- **Address probe:** correct `0x14→0x15→0x16` order; error (not silent `0x14`) when none respond.
- **Pin-name resolution:** HAT names and aliases map to the right BCM offsets (§3.1).
- **HAT detection:** fixture device-trees for V4-fallback (no/unknown uuid) and V5-match; assert `Version`, `SpeakerENPin`, `MotorMode`.
- **Reset sequence:** fake clock → assert `MCURST` goes LOW then HIGH with the required dwell, no real sleep.

Hardware smoke test: run `picarctl ping-mcu`, `hat-info`, and `blink` on a real Pi during development.

---

## Dependency direction

Strictly downward, matching VISION (all packages under the `gopicar/` module root):

```
gopicar/pkg/mcu → gopicar/pkg/bus, gopicar/pkg/gpio
gopicar/pkg/bus → golang.org/x/sys/unix
gopicar/pkg/gpio → github.com/warthog618/go-gpiocdev
gopicar/internal/fake → (implements bus.Bus and gpio.Chip interfaces)
gopicar/cmd/picarctl → gopicar/pkg/{mcu,bus,gpio}
```

No upward calls, no global state.

---

## Out of scope for Milestone 1

Servos, PWM/timer arithmetic, ADC, grayscale, motors, ultrasonic, config file I/O, and the full `picarx.Device` facade (including `Recover()` and the goroutine-safety locking) — all land in later milestones. M1 delivers only the foundation needed to reach the HAT, identify it, reset it, and toggle a GPIO.
