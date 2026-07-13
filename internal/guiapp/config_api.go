package guiapp

import (
	"encoding/json"
	"fmt"

	"github.com/TIANLI0/THRM/internal/ipc"
)

// GetConfig 获取当前配置
func (a *App) GetConfig() (AppConfig, error) {
	resp, err := a.sendRequest(ipc.ReqGetConfig, nil)
	if err != nil {
		return AppConfig{}, err
	}
	if !resp.Success {
		return AppConfig{}, fmt.Errorf("%s", resp.Error)
	}
	var cfg AppConfig
	if err := json.Unmarshal(resp.Data, &cfg); err != nil {
		return AppConfig{}, fmt.Errorf("decode config: %w", err)
	}
	return cfg, nil
}

// UpdateConfig 更新配置
func (a *App) UpdateConfig(cfg AppConfig) error {
	resp, err := a.sendRequest(ipc.ReqUpdateConfig, cfg)
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}
