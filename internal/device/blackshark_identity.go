package device

import (
	"strings"

	"github.com/Eureka-o/FanControlPortable/internal/types"
)

const legacyBlackSharkHIDProfileID = "builtin.blackshark.brb02.hid.rpm"

func isLegacyBlackSharkHIDProfileID(id string) bool {
	return strings.EqualFold(strings.TrimSpace(id), legacyBlackSharkHIDProfileID)
}

func isBlackSharkProfileID(id string) bool {
	return id == types.BlackSharkBRB02ProfileID ||
		id == types.BlackSharkBRB02USBProfileID
}
