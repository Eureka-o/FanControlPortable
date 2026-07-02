package deviceprofiles

import "github.com/TIANLI0/THRM/internal/types"

func BuiltInProfileByID(profileID string) (types.DeviceProfile, bool) {
	return types.BuiltInDeviceProfileByID(profileID)
}

func BuiltInProfiles(endpoint string) []types.DeviceProfile {
	return types.BuiltInDeviceProfiles(endpoint)
}
