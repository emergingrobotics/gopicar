package main

// calibstore is the EXAMPLE's calibration persistence. The gopicar library is
// deliberately storage-agnostic — pkg/picarx takes a picarx.Calibration value
// and does no file or env access. This file shows one reasonable way to persist
// that value: a JSON file under the user's config dir. Copy/adapt as you like.

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/emergingrobotics/gopicar/pkg/picarx"
	"github.com/emergingrobotics/gopicar/pkg/servo"
)

// configPath resolves the calibration file: $GOPICAR_CONFIG, else
// $XDG_CONFIG_HOME/gopicar/calibration.json, else ~/.config/gopicar/….
func configPath() string {
	if p := os.Getenv("GOPICAR_CONFIG"); p != "" {
		return p
	}
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "gopicar", "calibration.json")
}

// loadCalibration reads the calibration file. A missing file is not an error:
// the measured defaults for this robot are returned so the CLI works out of the
// box. (A neutral robot would use picarx.NeutralCalibration() here instead.)
func loadCalibration() (picarx.Calibration, string, error) {
	path := configPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return picarx.MeasuredCalibration(), path, nil
		}
		return picarx.Calibration{}, path, fmt.Errorf("read %s: %w", path, err)
	}
	// Start from measured defaults so any field omitted from the file keeps a
	// sane value, then overlay the file.
	cal := picarx.MeasuredCalibration()
	if err := json.Unmarshal(data, &cal); err != nil {
		return picarx.Calibration{}, path, fmt.Errorf("parse %s: %w", path, err)
	}
	return cal, path, nil
}

func saveCalibration(path string, cal picarx.Calibration) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cal, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func printCalibration(out io.Writer, cal picarx.Calibration) {
	fmt.Fprintln(out, "servo   trim    dir   min    max")
	rows := []struct {
		name string
		c    servo.Calibration
	}{
		{"steer", cal.Steer}, {"pan", cal.Pan}, {"tilt", cal.Tilt},
	}
	for _, r := range rows {
		fmt.Fprintf(out, "%-6s  %+5.0f   %+2.0f   %+4.0f  %+4.0f\n",
			r.name, r.c.Trim, r.c.Dir, r.c.Min, r.c.Max)
	}
}

// servoRef returns a pointer to the named servo's calibration within cal.
func servoRef(cal *picarx.Calibration, name string) *servo.Calibration {
	switch name {
	case "steer":
		return &cal.Steer
	case "pan":
		return &cal.Pan
	case "tilt":
		return &cal.Tilt
	default:
		return nil
	}
}

func cmdCalibrate(ctx context.Context, args []string, out io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: picarctl calibrate <show|set|save-defaults> …")
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "show":
		cal, path, err := loadCalibration()
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "config: %s\n", path)
		printCalibration(out, cal)
		return nil

	case "save-defaults":
		cal := picarx.MeasuredCalibration()
		path := configPath()
		if err := saveCalibration(path, cal); err != nil {
			return err
		}
		fmt.Fprintf(out, "wrote defaults to %s\n", path)
		printCalibration(out, cal)
		return nil

	case "set":
		fs := flag.NewFlagSet("calibrate set", flag.ContinueOnError)
		fs.SetOutput(out)
		which := fs.String("which", "", "named servo: pan | tilt | steer")
		trim := fs.Float64("trim", 0, "raw center angle (deg)")
		dir := fs.Float64("dir", 0, "direction sign +1 or -1")
		min := fs.Float64("min", 0, "user-facing min angle (deg)")
		max := fs.Float64("max", 0, "user-facing max angle (deg)")
		if err := fs.Parse(rest); err != nil {
			return err
		}
		cal, path, err := loadCalibration()
		if err != nil {
			return err
		}
		ref := servoRef(&cal, *which)
		if ref == nil {
			return fmt.Errorf("unknown servo %q (want pan, tilt, or steer)", *which)
		}
		// Only overwrite fields the user actually passed.
		visit := map[string]bool{}
		fs.Visit(func(f *flag.Flag) { visit[f.Name] = true })
		if visit["trim"] {
			ref.Trim = *trim
		}
		if visit["dir"] {
			ref.Dir = *dir
		}
		if visit["min"] {
			ref.Min = *min
		}
		if visit["max"] {
			ref.Max = *max
		}
		if err := saveCalibration(path, cal); err != nil {
			return err
		}
		fmt.Fprintf(out, "updated %s in %s\n", *which, path)
		printCalibration(out, cal)
		return nil

	default:
		return fmt.Errorf("unknown calibrate subcommand %q (want show, set, or save-defaults)", sub)
	}
}
