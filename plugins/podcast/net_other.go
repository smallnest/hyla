//go:build !darwin

package main

func systemProxyURL() string {
	return ""
}
