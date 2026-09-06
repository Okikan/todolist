//go:build windows

package main

import (
	"os"

	"golang.org/x/sys/windows/registry"
)

const (
	runKeyPath   = `Software\Microsoft\Windows\CurrentVersion\Run`
	autoStartVal = "TodoList"
)

// GetAutoStart 查询是否已设置开机自启
func (a *App) GetAutoStart() (bool, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.QUERY_VALUE)
	if err != nil {
		return false, err
	}
	defer k.Close()
	if _, _, err := k.GetStringValue(autoStartVal); err == nil {
		return true, nil
	} else if err == registry.ErrNotExist {
		return false, nil
	} else {
		return false, err
	}
}

// SetAutoStart 开启或关闭开机自启（写入 HKCU 的 Run 键），返回设置后的状态
func (a *App) SetAutoStart(enable bool) (bool, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return false, err
	}
	defer k.Close()
	if enable {
		exe, err := os.Executable()
		if err != nil {
			return false, err
		}
		if err := k.SetStringValue(autoStartVal, `"`+exe+`"`); err != nil {
			return false, err
		}
	} else if err := k.DeleteValue(autoStartVal); err != nil && err != registry.ErrNotExist {
		return false, err
	}
	return a.GetAutoStart()
}
