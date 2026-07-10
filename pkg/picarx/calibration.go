package picarx

import (
	"github.com/emergingrobotics/gopicar/pkg/motor"
	"github.com/emergingrobotics/gopicar/pkg/servo"
)

// Calibration aggregates the per-device calibration for a whole PiCar-X. It is
// a plain value with no filesystem or environment coupling; callers construct
// it in code (or load it from their own storage — see examples/picarctl) and
// pass it to Open via Options.
//
// Servo user-angle sign conventions: steer +right/-left, pan +right/-left,
// tilt +up/-down; 0 is centered/level for all three.
type Calibration struct {
	Steer servo.Calibration `json:"steer"`
	Pan   servo.Calibration `json:"pan"`
	Tilt  servo.Calibration `json:"tilt"`

	LeftMotor  motor.Calibration `json:"left_motor"`
	RightMotor motor.Calibration `json:"right_motor"`
}

// NeutralCalibration returns an identity calibration: no trim, +1 direction,
// full ±90° servo range, unity motor scale. Correct for a freshly-assembled
// robot whose servo horns are seated at their true centers.
func NeutralCalibration() Calibration {
	s := servo.Calibration{Trim: 0, Dir: 1, Min: -90, Max: 90}
	return Calibration{
		Steer: s, Pan: s, Tilt: s,
		LeftMotor:  motor.Calibration{Scale: 1},
		RightMotor: motor.Calibration{Scale: 1},
	}
}

// MeasuredCalibration returns the values measured live on the author's PiCar-X
// (2026-07-10). These are PER-UNIT — every robot differs, especially where a
// servo horn was not reseated. They are provided as a convenient starting point
// and a worked example; obtain your own via an interactive centering pass and
// store them yourself (see examples/picarctl's calibrate command).
//
//	steer: trim -58° (horn not reseated — large trim, ±30° usable)
//	pan:   trim -11° (reseated), tilt: trim +25° (reseated; ~85° up / 40° down)
func MeasuredCalibration() Calibration {
	return Calibration{
		Steer:      servo.Calibration{Trim: -58, Dir: 1, Min: -30, Max: 30},
		Pan:        servo.Calibration{Trim: -11, Dir: -1, Min: -80, Max: 80},
		Tilt:       servo.Calibration{Trim: 25, Dir: -1, Min: -40, Max: 85},
		LeftMotor:  motor.Calibration{Scale: 1},
		RightMotor: motor.Calibration{Scale: 1},
	}
}
