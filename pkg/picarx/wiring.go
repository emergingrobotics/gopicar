package picarx

// Wiring maps PiCar-X functions to PWM channels and HAT GPIO pins. The zero
// value is not useful; use DefaultWiring and override fields as needed. The
// mapping is a software convention (the stock SunFounder layout), not fixed
// hardware wiring — servo/motor leads can be moved to other channels (§7.3).
type Wiring struct {
	PanChan   uint8 // camera pan PWM channel (P0)
	TiltChan  uint8 // camera tilt PWM channel (P1)
	SteerChan uint8 // steering PWM channel (P2)

	LeftMotorChan  uint8  // left rear motor PWM channel (P13)
	RightMotorChan uint8  // right rear motor PWM channel (P12)
	LeftDirPin     string // left motor direction GPIO (D4)
	RightDirPin    string // right motor direction GPIO (D5)

	TrigPin string // ultrasonic trigger (D2)
	EchoPin string // ultrasonic echo (D3)

	LEDPin string // user LED (D14)
}

// DefaultWiring returns the stock SunFounder PiCar-X wiring (§3, §6, §7).
func DefaultWiring() Wiring {
	return Wiring{
		PanChan:   0,
		TiltChan:  1,
		SteerChan: 2,

		LeftMotorChan:  13,
		RightMotorChan: 12,
		LeftDirPin:     "D4",
		RightDirPin:    "D5",

		TrigPin: "D2",
		EchoPin: "D3",

		LEDPin: "D14",
	}
}

// withDefaults fills any zero fields from DefaultWiring so a partially-specified
// Wiring still works.
func (w Wiring) withDefaults() Wiring {
	d := DefaultWiring()
	if w.LeftDirPin == "" {
		w.LeftDirPin = d.LeftDirPin
	}
	if w.RightDirPin == "" {
		w.RightDirPin = d.RightDirPin
	}
	if w.TrigPin == "" {
		w.TrigPin = d.TrigPin
	}
	if w.EchoPin == "" {
		w.EchoPin = d.EchoPin
	}
	if w.LEDPin == "" {
		w.LEDPin = d.LEDPin
	}
	// Channels: a zero SteerChan is legitimate (P0), so only backfill the whole
	// set when the Wiring is entirely empty (all channel fields zero).
	if w.PanChan == 0 && w.TiltChan == 0 && w.SteerChan == 0 &&
		w.LeftMotorChan == 0 && w.RightMotorChan == 0 {
		w.PanChan, w.TiltChan, w.SteerChan = d.PanChan, d.TiltChan, d.SteerChan
		w.LeftMotorChan, w.RightMotorChan = d.LeftMotorChan, d.RightMotorChan
	}
	return w
}
