package deviceprofiles

import "github.com/Eureka-o/FanControlPortable/internal/types"

func BuiltInProfileByID(profileID string) (types.DeviceProfile, bool) {
	return types.BuiltInDeviceProfileByID(profileID)
}

func BuiltInProfiles(endpoint string) []types.DeviceProfile {
	return types.BuiltInDeviceProfiles(endpoint)
}
