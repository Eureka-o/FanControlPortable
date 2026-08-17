package main

import (
	"github.com/Eureka-o/FanControlPortable/internal/guiapp"
	"github.com/Eureka-o/FanControlPortable/internal/theme"
)

// App keeps the Wails binding surface in package main while delegating implementation to internal/guiapp.
type App struct {
	*guiapp.App
}

func NewApp() *App {
	return NewAppWithThemeManager(newThemeManager())
}

func NewAppWithThemeManager(themeManager *theme.Manager) *App {
	return &App{App: guiapp.New(themeManager)}
}
