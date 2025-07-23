package executor

import (
	"autoui-platform/backend/internal/models"
	"autoui-platform/backend/pkg/chrome"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
)

type TestExecutor struct {
	ctx        context.Context
	cancel     context.CancelFunc
	device     models.Device
	maxWorkers int
	workQueue  chan ExecutionJob
	wg         sync.WaitGroup
	mutex      sync.RWMutex
	running    map[uint]bool
}

type ExecutionJob struct {
	Execution  *models.TestExecution
	TestCase   *models.TestCase
	IsVisual   bool
	ResultChan chan ExecutionResult
}

type ExecutionResult struct {
	Success      bool
	ErrorMessage string
	Screenshots  []string
	Logs         []ExecutionLog
	Metrics      *models.PerformanceMetric
}

type ExecutionLog struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	StepIndex int       `json:"step_index"`
}

var GlobalExecutor *TestExecutor

func InitExecutor(maxWorkers int) {
	GlobalExecutor = &TestExecutor{
		maxWorkers: maxWorkers,
		workQueue:  make(chan ExecutionJob, maxWorkers*2),
		running:    make(map[uint]bool),
	}

	// Start worker goroutines
	for i := 0; i < maxWorkers; i++ {
		go GlobalExecutor.worker()
	}

	log.Printf("Test executor initialized with %d workers", maxWorkers)
}

func (te *TestExecutor) worker() {
	for job := range te.workQueue {
		// Execute the test case
		result := te.executeTestCase(job.TestCase, job.IsVisual)

		// Mark execution as completed
		te.mutex.Lock()
		delete(te.running, job.Execution.ID)
		te.mutex.Unlock()

		// Send result
		job.ResultChan <- result
	}
}

func (te *TestExecutor) ExecuteTestCase(execution *models.TestExecution, testCase *models.TestCase) <-chan ExecutionResult {
	return te.ExecuteTestCaseWithOptions(execution, testCase, false)
}

func (te *TestExecutor) ExecuteTestCaseWithOptions(execution *models.TestExecution, testCase *models.TestCase, isVisual bool) <-chan ExecutionResult {
	te.mutex.Lock()
	te.running[execution.ID] = true
	te.mutex.Unlock()

	resultChan := make(chan ExecutionResult, 1)
	job := ExecutionJob{
		Execution:  execution,
		TestCase:   testCase,
		IsVisual:   isVisual,
		ResultChan: resultChan,
	}

	te.workQueue <- job
	return resultChan
}

func (te *TestExecutor) IsRunning(executionID uint) bool {
	te.mutex.RLock()
	defer te.mutex.RUnlock()
	return te.running[executionID]
}

func (te *TestExecutor) GetRunningCount() int {
	te.mutex.RLock()
	defer te.mutex.RUnlock()
	return len(te.running)
}

func (te *TestExecutor) executeTestCase(testCase *models.TestCase, isVisual bool) ExecutionResult {
	result := ExecutionResult{
		Screenshots: make([]string, 0),
		Logs:        make([]ExecutionLog, 0),
	}

	// Parse test steps
	steps, err := testCase.GetSteps()
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("Failed to parse test steps: %v", err)
		return result
	}

	// Check if Chrome is available
	chromePath := chrome.GetChromePath()
	if chromePath == "" {
		result.Success = false
		result.ErrorMessage = "Chrome browser not found. Please install Google Chrome or Chromium"
		result.addLog("error", "Chrome not found in system", -1)
		return result
	}
	
	result.addLog("info", fmt.Sprintf("Using Chrome path: %s", chromePath), -1)
	
	// Test if Chrome executable exists and is accessible
	if _, err := os.Stat(chromePath); err != nil {
		result.Success = false
		result.ErrorMessage = fmt.Sprintf("Chrome executable not accessible: %v", err)
		result.addLog("error", fmt.Sprintf("Chrome path not accessible: %v", err), -1)
		return result
	}

	// Create Chrome context with device emulation using DevTools
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(chromePath),
		chromedp.Flag("headless", !isVisual), // Set headless based on visual execution preference
		chromedp.Flag("disable-web-security", true),
		chromedp.Flag("disable-features", "VizDisplayCompositor"),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-gpu", false), // Enable GPU for better rendering
		chromedp.Flag("disable-software-rasterizer", false),
		chromedp.Flag("disable-background-timer-throttling", true),
		chromedp.Flag("disable-backgrounding-occluded-windows", true),
		chromedp.Flag("disable-renderer-backgrounding", true),
		chromedp.Flag("ignore-certificate-errors", true),
		chromedp.Flag("ignore-ssl-errors", true),
		chromedp.Flag("ignore-certificate-errors-spki-list", true),
		chromedp.Flag("ignore-ssl-errors-spki-list", true),
		chromedp.Flag("allow-running-insecure-content", true),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("excludeSwitches", "enable-automation"),
		chromedp.Flag("useAutomationExtension", false),
		// Remove WindowSize and UserAgent as we'll use DevTools device emulation
	)

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	// Set timeout
	ctx, cancel = context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	startTime := time.Now()

	// Enable device emulation using DevTools (equivalent to Ctrl+Shift+M)
	result.addLog("info", fmt.Sprintf("Setting up device emulation: %s (%dx%d)", testCase.Device.Name, testCase.Device.Width, testCase.Device.Height), -1)
	err = chromedp.Run(ctx, chromedp.EmulateViewport(int64(testCase.Device.Width), int64(testCase.Device.Height)))
	if err != nil {
		result.addLog("warn", fmt.Sprintf("Failed to set viewport emulation: %v", err), -1)
	} else {
		result.addLog("info", "Device viewport emulation enabled", -1)
	}

	// Navigate to target URL
	result.addLog("info", "Navigating to target URL: "+testCase.TargetURL, -1)
	err = chromedp.Run(ctx, chromedp.Navigate(testCase.TargetURL))
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("Failed to navigate to URL: %v", err)
		return result
	}

	// Wait for page load and check if content is loaded
	result.addLog("info", "Waiting for page to load...", -1)
	err = chromedp.Run(ctx, 
		chromedp.Sleep(3*time.Second),
		chromedp.WaitReady("body", chromedp.ByQuery),
	)
	if err != nil {
		// If body is not ready, try to get page title and current URL for debugging
		var title, currentURL string
		titleErr := chromedp.Run(ctx, chromedp.Title(&title))
		urlErr := chromedp.Run(ctx, chromedp.Location(&currentURL))
		
		debugInfo := fmt.Sprintf("Page load timeout - Title: '%s', URL: '%s', TitleErr: %v, URLErr: %v", 
			title, currentURL, titleErr, urlErr)
		result.addLog("warn", debugInfo, -1)
		
		// Continue execution even if page is not fully loaded
		result.addLog("warn", "Page not fully loaded, continuing with execution", -1)
	} else {
		result.addLog("info", "Page loaded successfully", -1)
	}

	// Additional check: get page title and URL for verification
	var pageTitle, pageURL string
	chromedp.Run(ctx, chromedp.Title(&pageTitle))
	chromedp.Run(ctx, chromedp.Location(&pageURL))
	result.addLog("info", fmt.Sprintf("Page info - Title: '%s', URL: '%s'", pageTitle, pageURL), -1)

	// Take initial screenshot
	screenshotPath := te.takeScreenshot(ctx, "initial", 0)
	if screenshotPath != "" {
		result.Screenshots = append(result.Screenshots, screenshotPath)
	}

	// Execute test steps
	for i, step := range steps {
		result.addLog("info", fmt.Sprintf("Executing step %d: %s", i+1, step.Type), i)

		err = te.executeStep(ctx, step, i)
		if err != nil {
			result.ErrorMessage = fmt.Sprintf("Step %d failed: %v", i+1, err)
			result.addLog("error", result.ErrorMessage, i)

			// Take error screenshot
			screenshotPath := te.takeScreenshot(ctx, "error", i)
			if screenshotPath != "" {
				result.Screenshots = append(result.Screenshots, screenshotPath)
			}
			return result
		}

		result.addLog("info", fmt.Sprintf("Step %d completed successfully", i+1), i)

		// Take screenshot for key steps
		if te.shouldTakeScreenshot(step) {
			screenshotPath := te.takeScreenshot(ctx, "step", i)
			if screenshotPath != "" {
				result.Screenshots = append(result.Screenshots, screenshotPath)
			}
		}

		// Small delay between steps
		chromedp.Run(ctx, chromedp.Sleep(500*time.Millisecond))
	}

	// Take final screenshot
	screenshotPath = te.takeScreenshot(ctx, "final", len(steps))
	if screenshotPath != "" {
		result.Screenshots = append(result.Screenshots, screenshotPath)
	}

	// Collect performance metrics
	result.Metrics = te.collectPerformanceMetrics(ctx)
	result.Metrics.PageLoadTime = int(time.Since(startTime).Milliseconds())

	result.Success = true
	result.addLog("info", "Test case execution completed successfully", -1)

	return result
}

func (te *TestExecutor) executeStep(ctx context.Context, step models.TestStep, stepIndex int) error {
	switch step.Type {
	case "click":
		return te.executeClick(ctx, step)
	case "input":
		return te.executeInput(ctx, step)
	case "keydown":
		return te.executeKeydown(ctx, step)
	case "scroll":
		return te.executeScroll(ctx, step)
	case "touchstart", "touchend":
		return te.executeTouch(ctx, step)
	case "change":
		return te.executeChange(ctx, step)
	case "submit":
		return te.executeSubmit(ctx, step)
	default:
		return fmt.Errorf("unsupported step type: %s", step.Type)
	}
}

func (te *TestExecutor) executeClick(ctx context.Context, step models.TestStep) error {
	// Simple approach - just wait for the element and click it
	err := chromedp.Run(ctx,
		chromedp.WaitVisible(step.Selector, chromedp.ByQuery),
		chromedp.Click(step.Selector, chromedp.ByQuery),
		chromedp.Sleep(200*time.Millisecond),
	)

	if err != nil {
		return fmt.Errorf("failed to click element %s: %v", step.Selector, err)
	}

	return nil
}

func (te *TestExecutor) executeInput(ctx context.Context, step models.TestStep) error {
	return chromedp.Run(ctx,
		chromedp.Clear(step.Selector),
		chromedp.SendKeys(step.Selector, step.Value),
		chromedp.Sleep(200*time.Millisecond),
	)
}

func (te *TestExecutor) executeKeydown(ctx context.Context, step models.TestStep) error {
	return chromedp.Run(ctx,
		chromedp.KeyEvent(step.Value),
		chromedp.Sleep(200*time.Millisecond),
	)
}

func (te *TestExecutor) executeScroll(ctx context.Context, step models.TestStep) error {
	if coords, ok := step.Coordinates["scrollY"].(float64); ok {
		return chromedp.Run(ctx,
			chromedp.Evaluate(fmt.Sprintf("window.scrollTo(0, %f)", coords), nil),
			chromedp.Sleep(200*time.Millisecond),
		)
	}
	return nil
}

func (te *TestExecutor) executeTouch(ctx context.Context, step models.TestStep) error {
	// For touch events, we simulate them as clicks for now
	if step.Type == "touchstart" {
		return te.executeClick(ctx, step)
	}
	return nil
}

func (te *TestExecutor) executeChange(ctx context.Context, step models.TestStep) error {
	return chromedp.Run(ctx,
		chromedp.SetValue(step.Selector, step.Value),
		chromedp.Sleep(200*time.Millisecond),
	)
}

func (te *TestExecutor) executeSubmit(ctx context.Context, step models.TestStep) error {
	return chromedp.Run(ctx,
		chromedp.Submit(step.Selector),
		chromedp.Sleep(1*time.Second),
	)
}

func (te *TestExecutor) takeScreenshot(ctx context.Context, stepType string, stepIndex int) string {
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("%s_%s_%d_%s.png", stepType, timestamp, stepIndex, generateRandomString(8))

	// Create screenshots directory if not exists
	screenshotDir := "./screenshots"
	if err := os.MkdirAll(screenshotDir, 0755); err != nil {
		log.Printf("Failed to create screenshots directory: %v", err)
		return ""
	}

	fullPath := filepath.Join(screenshotDir, filename)

	var buf []byte
	err := chromedp.Run(ctx, chromedp.CaptureScreenshot(&buf))
	if err != nil {
		log.Printf("Failed to take screenshot: %v", err)
		return ""
	}

	// Save screenshot to file
	err = ioutil.WriteFile(fullPath, buf, 0644)
	if err != nil {
		log.Printf("Failed to save screenshot file: %v", err)
		return ""
	}

	log.Printf("Screenshot saved: %s", fullPath)
	return filename
}

func (te *TestExecutor) shouldTakeScreenshot(step models.TestStep) bool {
	// Take screenshots for key interaction types
	keyTypes := []string{"click", "submit", "change"}
	for _, keyType := range keyTypes {
		if step.Type == keyType {
			return true
		}
	}
	return false
}

func (te *TestExecutor) collectPerformanceMetrics(ctx context.Context) *models.PerformanceMetric {
	metric := &models.PerformanceMetric{}

	// Collect performance timing data using string evaluation
	var performanceDataStr string
	err := chromedp.Run(ctx,
		chromedp.Evaluate(`
			JSON.stringify({
				domContentLoaded: performance.timing.domContentLoadedEventEnd - performance.timing.navigationStart,
				firstPaint: performance.getEntriesByType('paint').find(entry => entry.name === 'first-paint')?.startTime || 0,
				firstContentfulPaint: performance.getEntriesByType('paint').find(entry => entry.name === 'first-contentful-paint')?.startTime || 0,
				memoryUsage: performance.memory ? performance.memory.usedJSHeapSize / 1024 / 1024 : 0,
				networkRequests: performance.getEntriesByType('resource').length,
				networkTime: performance.getEntriesByType('navigation')[0] ? performance.getEntriesByType('navigation')[0].loadEventEnd - performance.getEntriesByType('navigation')[0].fetchStart : 0,
				jsHeapSize: performance.memory ? performance.memory.totalJSHeapSize / 1024 / 1024 : 0
			})
		`, &performanceDataStr),
	)

	if err != nil {
		log.Printf("Failed to collect performance metrics: %v", err)
		return metric
	}

	// Parse the JSON string manually
	performanceDataStr = strings.Trim(performanceDataStr, "\"")
	performanceDataStr = strings.ReplaceAll(performanceDataStr, "\\", "")

	// Extract values using string parsing (simple implementation)
	if strings.Contains(performanceDataStr, "domContentLoaded") {
		if idx := strings.Index(performanceDataStr, "domContentLoaded\":"); idx != -1 {
			valueStr := performanceDataStr[idx+17:]
			if commaIdx := strings.Index(valueStr, ","); commaIdx != -1 {
				valueStr = valueStr[:commaIdx]
			}
			if val := parseFloat(valueStr); val > 0 {
				metric.DOMContentLoaded = int(val)
			}
		}
	}

	if strings.Contains(performanceDataStr, "memoryUsage") {
		if idx := strings.Index(performanceDataStr, "memoryUsage\":"); idx != -1 {
			valueStr := performanceDataStr[idx+13:]
			if commaIdx := strings.Index(valueStr, ","); commaIdx != -1 {
				valueStr = valueStr[:commaIdx]
			}
			if val := parseFloat(valueStr); val > 0 {
				metric.MemoryUsage = val
			}
		}
	}

	if strings.Contains(performanceDataStr, "networkRequests") {
		if idx := strings.Index(performanceDataStr, "networkRequests\":"); idx != -1 {
			valueStr := performanceDataStr[idx+17:]
			if commaIdx := strings.Index(valueStr, ","); commaIdx != -1 {
				valueStr = valueStr[:commaIdx]
			} else if closeIdx := strings.Index(valueStr, "}"); closeIdx != -1 {
				valueStr = valueStr[:closeIdx]
			}
			if val := parseFloat(valueStr); val > 0 {
				metric.NetworkRequests = int(val)
			}
		}
	}

	return metric
}

// Simple float parsing helper
func parseFloat(s string) float64 {
	s = strings.TrimSpace(s)
	var result float64 = 0
	var decimal float64 = 0.1
	var isDecimal bool = false

	for _, char := range s {
		if char >= '0' && char <= '9' {
			digit := float64(char - '0')
			if isDecimal {
				result += digit * decimal
				decimal *= 0.1
			} else {
				result = result*10 + digit
			}
		} else if char == '.' && !isDecimal {
			isDecimal = true
		} else {
			break
		}
	}
	return result
}

func (result *ExecutionResult) addLog(level, message string, stepIndex int) {
	result.Logs = append(result.Logs, ExecutionLog{
		Timestamp: time.Now(),
		Level:     level,
		Message:   message,
		StepIndex: stepIndex,
	})
}

func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	result := make([]byte, length)
	for i := range result {
		result[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(result)
}

// Stop gracefully shuts down the executor
func (te *TestExecutor) Stop() {
	te.mutex.Lock()
	defer te.mutex.Unlock()

	if te.workQueue != nil {
		close(te.workQueue)
	}

	if te.cancel != nil {
		te.cancel()
	}

	log.Println("Test executor stopped")
}

// GetExecutionStatus returns the current status of an execution
func (te *TestExecutor) GetExecutionStatus(executionID uint) string {
	te.mutex.RLock()
	defer te.mutex.RUnlock()

	if te.running[executionID] {
		return "running"
	}
	return "completed"
}

// CancelExecution cancels a running execution
func (te *TestExecutor) CancelExecution(executionID uint) bool {
	te.mutex.Lock()
	defer te.mutex.Unlock()

	if te.running[executionID] {
		delete(te.running, executionID)
		log.Printf("Execution %d cancelled", executionID)
		return true
	}
	return false
}
