// Package picarx is the high-level driver for the SunFounder PiCar-X robot. It
// is the easy entry point over the composable low-level packages (pkg/pwm,
// pkg/adc, pkg/servo, pkg/motor, pkg/ultrasonic, pkg/bus, pkg/gpio, pkg/mcu).
//
// Open builds the whole stack and returns a PiCarX that centers steering, aims
// the camera, drives the motors, and reads the battery, grayscale, and
// ultrasonic sensors — all with calibration applied. The library performs no
// filesystem or environment access: pass calibration in as a value (see
// NeutralCalibration and MeasuredCalibration); persist it however you like
// (examples/picarctl shows a JSON-file approach).
//
// Quickstart:
//
//	ctx := context.Background()
//	px, err := picarx.Open(ctx, picarx.Options{Calibration: picarx.MeasuredCalibration()})
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer px.Close()
//
//	px.SetDir(ctx, 0)          // steering straight
//	px.SetCamTilt(ctx, 0)      // camera level
//	v, _ := px.Battery(ctx)    // pack voltage
//	px.Forward(ctx, 40)        // both motors forward at 40%
//	time.Sleep(time.Second)
//	px.Stop(ctx)
//
// Servo user-angle conventions: steer +right/-left, pan +right/-left,
// tilt +up/-down; 0 is centered/level.
//
// Concurrency: a PiCarX is safe for concurrent use; the underlying I²C bus is
// mutex-guarded.
package picarx
