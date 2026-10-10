//go:build !darwin

package main

func diskSpace(path string) string { return "" }
