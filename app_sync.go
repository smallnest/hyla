package main

import (
	"hyla/internal/settings"
	"hyla/locales"
)

// The app as the sync state reads it: the run's own work on the main
// thread, and the lines it says.

func (a *App) Loaded() bool { return a.loaded }

func (a *App) Update(fn func()) { a.update(fn) }

func (a *App) Toast(msg string) { a.toast(msg) }

func (a *App) ToastSyncKept(path string) { a.toast(locales.ToastSyncKept(path)) }

func (a *App) ApplySettings(s settings.Settings) { a.applySettings(s) }
