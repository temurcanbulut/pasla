package node

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"path/filepath"
	"strings"
)

// safetyNumber is a 12-digit number derived from the identities of two
// paired devices. Both sides compute the same value, so two people can read
// it to each other at any time to confirm they are still talking to the
// device they paired with.
func safetyNumber(a, b string) string {
	if a > b {
		a, b = b, a
	}
	sum := sha256.Sum256([]byte("pasla-safety-v1|" + a + "|" + b))
	var parts []string
	for i := 0; i < 3; i++ {
		parts = append(parts, fmt.Sprintf("%04d", binary.BigEndian.Uint32(sum[4*i:])%10000))
	}
	return strings.Join(parts, " ")
}

// riskyExt lists file types that run code when opened on Windows or Linux.
var riskyExt = map[string]bool{
	".exe": true, ".msi": true, ".msix": true, ".appx": true, ".bat": true, ".cmd": true, ".com": true,
	".scr": true, ".pif": true, ".cpl": true, ".lnk": true, ".url": true, ".hta": true, ".jar": true,
	".js": true, ".jse": true, ".vbs": true, ".vbe": true, ".wsf": true, ".wsh": true, ".ps1": true,
	".psm1": true, ".reg": true, ".dll": true, ".sys": true, ".iso": true, ".img": true, ".vhd": true,
	".vhdx": true, ".docm": true, ".xlsm": true, ".pptm": true, ".sh": true, ".run": true,
	".appimage": true, ".deb": true, ".rpm": true, ".desktop": true,
}

func isRisky(name string) bool {
	return riskyExt[strings.ToLower(filepath.Ext(name))]
}
