package mcu

import (
	"os"
	"path/filepath"
	"strings"
)

// HATVersion identifies the Robot HAT generation.
type HATVersion int

const (
	HATv4 HATVersion = iota
	HATv5
)

func (v HATVersion) String() string {
	if v == HATv5 {
		return "V5"
	}
	return "V4"
}

// v5UUID is the known Robot HAT V5 EEPROM UUID (§4).
const v5UUID = "9daeea78-0000-076e-0032-582369ac3e02"

// HAT is the detection result. SpeakerENPin is a HAT pin name (§4, §11):
// V4 = D15 (GPIO20), V5 = D10 (GPIO12). MotorMode: 1 = TC1508S, 2 = TC618S (§6).
type HAT struct {
	Version      HATVersion
	UUID         string
	SpeakerENPin string
	MotorMode    int
}

// DetectHAT scans <root>/proc/device-tree/hat*/uuid (§4). The directory name
// only *contains* "hat", so a glob is used, not a fixed path. Unknown or absent
// → V4 fallback.
func DetectHAT(root string) HAT {
	uuid := strings.TrimSpace(strings.Trim(readHATUUID(root), "\x00"))
	if strings.EqualFold(uuid, v5UUID) {
		return HAT{Version: HATv5, UUID: uuid, SpeakerENPin: "D10", MotorMode: 2}
	}
	return HAT{Version: HATv4, UUID: uuid, SpeakerENPin: "D15", MotorMode: 1}
}

func readHATUUID(root string) string {
	matches, _ := filepath.Glob(filepath.Join(root, "proc", "device-tree", "hat*", "uuid"))
	for _, m := range matches {
		if data, err := os.ReadFile(m); err == nil {
			return string(data)
		}
	}
	return ""
}
