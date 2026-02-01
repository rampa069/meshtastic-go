package ble

// Meshtastic BLE Service and Characteristic UUIDs
const (
	// ServiceUUID is the main Meshtastic BLE service UUID
	ServiceUUID = "6ba1b218-15a8-461f-9fa8-5dcae273eafd"

	// ToRadioUUID is the characteristic for writing data to the radio
	ToRadioUUID = "f75c76d2-129e-4dad-a1dd-7866124401e7"

	// FromNumUUID is the characteristic that notifies when data is available
	FromNumUUID = "ed9da18c-a800-4f66-a670-aa7547e34453"

	// FromRadioUUID is the characteristic for reading data from the radio
	FromRadioUUID = "2c55e69e-4993-11ed-b878-0242ac120002"

	// LogRadioUUID is the characteristic for receiving device logs
	LogRadioUUID = "5a3d6e49-06e6-4423-9944-e9de8cdf9547"
)

// BLE name pattern for Meshtastic devices
// Matches names like "Meshtastic_1234" where 1234 is the last 4 hex digits of node ID
const BLENamePattern = `^.*_([0-9a-fA-F]{4})$`
