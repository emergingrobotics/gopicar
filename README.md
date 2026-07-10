# gopicar

A Go driver library for the [SunFounder PiCar-X](https://www.sunfounder.com/products/picar-x) robot and its Robot HAT MCU. Import it into your own programs to drive the servos, motors, and sensors — no cgo, cross-compiles to a single static `linux/arm64` binary.

Module path: `github.com/emergingrobotics/gopicar`

## Install

```sh
go get github.com/emergingrobotics/gopicar/pkg/picarx
```

## Quickstart

```go
package main

import (
	"context"
	"log"
	"time"

	"github.com/emergingrobotics/gopicar/pkg/picarx"
)

func main() {
	ctx := context.Background()
	px, err := picarx.Open(ctx, picarx.Options{
		Calibration: picarx.MeasuredCalibration(), // or your own values
	})
	if err != nil {
		log.Fatal(err)
	}
	defer px.Close()

	px.SetDir(ctx, 0)       // steering straight
	px.SetCamTilt(ctx, 0)   // camera level

	v, _ := px.Battery(ctx)
	log.Printf("battery: %.2f V", v)

	px.Forward(ctx, 40)     // both motors forward @ 40%
	time.Sleep(time.Second)
	px.Stop(ctx)
}
```

## Package map

| Package | Role |
|---|---|
| `pkg/picarx` | **Start here.** High-level facade: `Open`/`Close`, servos, motors, drive helpers, sensors, calibration. |
| `pkg/servo` | One calibrated servo (`Angle`, `SetRaw`, `Center`). |
| `pkg/motor` | One rear motor (`Speed`, `Stop`) with speed remap + calibration. |
| `pkg/adc` | 12-bit analog reads, battery, grayscale. |
| `pkg/pwm` | Timer + duty math (servo frame, percentage duty). |
| `pkg/ultrasonic` | HC-SR04 distance over GPIO edges. |
| `pkg/bus` | Mutex-guarded, context-aware I²C with a retry decorator. |
| `pkg/gpio` | libgpiod v2 GPIO with HAT pin-name resolution. |
| `pkg/mcu` | Robot HAT MCU: register map, HAT detect, firmware, reset. |

Use the facade for the common case; drop to the granular packages when you need finer control (they take a `bus.Bus`/`gpio.Chip` and work standalone).

## Calibration

Servo "0°" rarely matches the mechanical center — it depends on how each horn was seated. The library is **storage-agnostic**: you pass a `picarx.Calibration` value into `Options`; it does no file or environment access.

- `picarx.NeutralCalibration()` — identity mapping (freshly-centered horns).
- `picarx.MeasuredCalibration()` — the author's unit's values; **per-robot**, use as a starting point.

Obtain your own by centering each servo and recording the raw angle that looks straight/level. The example CLI shows one way to **persist** calibration to a JSON file — see [`examples/picarctl/calibstore.go`](examples/picarctl/calibstore.go). User-angle conventions: steer `+`right/`-`left, pan `+`right/`-`left, tilt `+`up/`-`down; `0` centered.

## Example CLI: `picarctl`

[`examples/picarctl`](examples/picarctl) is a runnable reference that drives the whole library — a working smoke-test tool and a template for your own program.

```sh
go build -o bin/picarctl ./examples/picarctl
./bin/picarctl ping-mcu
./bin/picarctl read-battery
./bin/picarctl servo --which steer --angle 0
./bin/picarctl calibrate show
```

## Testing

```sh
make test        # fast, hardware-free (fakes)
make test-hw     # on the Pi: go test -tags hardware ./...
```

Hardware integration tests are gated behind the `hardware` build tag so `go test ./...` never touches a device. Actuator-moving tests additionally require `GOPICAR_HW_MOVE=1`.

## Build / cross-compile

```sh
make build        # host
make build-arm64  # Raspberry Pi 64-bit OS
make deploy       # build arm64 + scp to the Pi (see Makefile vars)
```

## Notes

- Repo owner is moving from `gherlein` to `emergingrobotics`; the module path already reflects the destination.
- Protocol details cite sections of `../picar-x-apis.md`.
