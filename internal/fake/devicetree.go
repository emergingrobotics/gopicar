package fake

import (
	"os"
	"path/filepath"
)

// WriteDeviceTree creates <root>/proc/device-tree/hat/uuid containing uuid
// (NUL-terminated, as the real device-tree files are).
func WriteDeviceTree(root, uuid string) error {
	dir := filepath.Join(root, "proc", "device-tree", "hat")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "uuid"), []byte(uuid+"\x00"), 0o644)
}
