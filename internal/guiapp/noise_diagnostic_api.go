package guiapp

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/TIANLI0/THRM/internal/ipc"
	"github.com/TIANLI0/THRM/internal/types"
)

func (a *App) BeginNoiseDiagnostic(request types.NoiseDiagnosticBeginRequest) (types.NoiseDiagnosticSession, error) {
	resp, err := a.sendRequestWithTimeout(ipc.ReqBeginNoiseDiagnostic, ipc.BeginNoiseDiagnosticParams{Request: request}, 3*time.Second)
	if err != nil {
		return types.NoiseDiagnosticSession{}, err
	}
	if !resp.Success {
		return types.NoiseDiagnosticSession{}, fmt.Errorf("%s", resp.Error)
	}
	var session types.NoiseDiagnosticSession
	if err := json.Unmarshal(resp.Data, &session); err != nil {
		return types.NoiseDiagnosticSession{}, err
	}
	return session, nil
}

func (a *App) SetNoiseDiagnosticTarget(sessionID string, value int) (types.NoiseDiagnosticTargetResult, error) {
	resp, err := a.sendRequestWithTimeout(ipc.ReqSetNoiseDiagnosticTarget, ipc.SetNoiseDiagnosticTargetParams{SessionID: sessionID, Value: value}, 5*time.Second)
	if err != nil {
		return types.NoiseDiagnosticTargetResult{}, err
	}
	if !resp.Success {
		return types.NoiseDiagnosticTargetResult{}, fmt.Errorf("%s", resp.Error)
	}
	var result types.NoiseDiagnosticTargetResult
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return types.NoiseDiagnosticTargetResult{}, err
	}
	return result, nil
}

func (a *App) EndNoiseDiagnostic(sessionID string) error {
	return a.finishNoiseDiagnosticRequest(ipc.ReqEndNoiseDiagnostic, sessionID)
}

func (a *App) CancelNoiseDiagnostic(sessionID string) error {
	return a.finishNoiseDiagnosticRequest(ipc.ReqCancelNoiseDiagnostic, sessionID)
}

func (a *App) finishNoiseDiagnosticRequest(reqType ipc.RequestType, sessionID string) error {
	resp, err := a.sendRequestWithTimeout(reqType, ipc.NoiseDiagnosticSessionParams{SessionID: sessionID}, 3*time.Second)
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

func (a *App) SaveNoiseDiagnosticResult(result types.NoiseDiagnosticResult) error {
	resp, err := a.sendRequestWithTimeout(ipc.ReqSaveNoiseDiagnosticResult, ipc.SaveNoiseDiagnosticResultParams{Result: result}, 3*time.Second)
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}
