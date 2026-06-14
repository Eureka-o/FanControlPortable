package types

// DiagnosticsBundle is the zip payload produced by the core service and saved
// by the GUI through a native file dialog.
type DiagnosticsBundle struct {
	FileName   string `json:"fileName"`
	DataBase64 string `json:"dataBase64"`
	SizeBytes  int64  `json:"sizeBytes"`
}
