package main

import (
	"autoui-platform/backend/pkg/chrome"
	"fmt"
)

func main() {
	fmt.Println("Chrome path:", chrome.GetChromePath())
	fmt.Println("Chrome available:", chrome.IsChromeAvailable())
}