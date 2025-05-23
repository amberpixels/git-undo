package gitundo

import (
	_ "embed"
)

//go:embed update.sh
var updateScript string

//go:embed uninstall.sh
var uninstallScript string

// GetUpdateScript returns the embedded update script content.
func GetUpdateScript() string {
	return updateScript
}

// GetUninstallScript returns the embedded uninstall script content.
func GetUninstallScript() string {
	return uninstallScript
}
