package recorder

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/gorilla/websocket"
	"autoui-platform/backend/pkg/chrome"
)

type ChromeRecorder struct {
	ctx        context.Context
	cancel     context.CancelFunc
	isRecording bool
	steps      []RecordStep
	mutex      sync.RWMutex
	wsConn     *websocket.Conn
	deviceInfo DeviceInfo
	sessionID  string
	targetURL  string
}

type DeviceInfo struct {
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	UserAgent string `json:"user_agent"`
}

type RecordStep struct {
	Type        string                 `json:"type"`
	Selector    string                 `json:"selector"`
	Value       string                 `json:"value"`
	Coordinates map[string]interface{} `json:"coordinates"`
	Options     map[string]interface{} `json:"options"`
	Timestamp   int64                  `json:"timestamp"`
	Screenshot  string                 `json:"screenshot"`
}

type RecorderManager struct {
	recorders map[string]*ChromeRecorder
	mutex     sync.RWMutex
}

var Manager = &RecorderManager{
	recorders: make(map[string]*ChromeRecorder),
}

func NewChromeRecorder(sessionID string, device DeviceInfo) *ChromeRecorder {
	return &ChromeRecorder{
		isRecording: false,
		steps:       make([]RecordStep, 0),
		deviceInfo:  device,
		sessionID:   sessionID,
	}
}

func (r *ChromeRecorder) StartRecording(targetURL string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.isRecording {
		return fmt.Errorf("recording is already in progress")
	}

	// Check if Chrome is available
	chromePath := chrome.GetChromePath()
	if chromePath == "" {
		return fmt.Errorf("Chrome browser not found. Please install Google Chrome or Chromium")
	}

	// Create Chrome context with device emulation
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(chromePath),
		chromedp.Flag("headless", false),
		chromedp.Flag("disable-web-security", true),
		chromedp.Flag("disable-features", "VizDisplayCompositor"),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("disable-extensions", true),
		chromedp.Flag("disable-plugins", true),
		chromedp.Flag("disable-images", false),
		chromedp.Flag("disable-background-timer-throttling", true),
		chromedp.Flag("disable-backgrounding-occluded-windows", true),
		chromedp.Flag("disable-renderer-backgrounding", true),
		chromedp.Flag("ignore-certificate-errors", true),
		chromedp.Flag("ignore-ssl-errors", true),
		chromedp.Flag("ignore-certificate-errors-spki-list", true),
		chromedp.Flag("disable-ipc-flooding-protection", true),
		chromedp.WindowSize(r.deviceInfo.Width, r.deviceInfo.Height),
		chromedp.UserAgent(r.deviceInfo.UserAgent),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	r.ctx, r.cancel = chromedp.NewContext(allocCtx, chromedp.WithLogf(log.Printf))

	// Navigate to target URL and inject recording script
	err := chromedp.Run(r.ctx,
		chromedp.Navigate(targetURL),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Sleep(3*time.Second), // Wait for dynamic content to load
		chromedp.Evaluate(getRecordingScript(), nil),
	)

	if err != nil {
		cancel()
		return fmt.Errorf("failed to start recording: %w", err)
	}

	r.isRecording = true
	r.steps = make([]RecordStep, 0)

	// Start listening for events
	go r.listenForEvents()

	return nil
}

func (r *ChromeRecorder) StopRecording() error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if !r.isRecording {
		return fmt.Errorf("no recording in progress")
	}

	if r.cancel != nil {
		r.cancel()
	}

	r.isRecording = false
	return nil
}

func (r *ChromeRecorder) GetSteps() []RecordStep {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	return append([]RecordStep(nil), r.steps...)
}

func (r *ChromeRecorder) IsRecording() bool {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	return r.isRecording
}

func (r *ChromeRecorder) listenForEvents() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-r.ctx.Done():
			return
		case <-ticker.C:
			if !r.isRecording {
				return
			}

			var events []RecordStep
			err := chromedp.Run(r.ctx,
				chromedp.Evaluate(`window.autoUIRecorder && window.autoUIRecorder.getEvents()`, &events),
			)

			if err != nil {
				log.Printf("Error getting events: %v", err)
				continue
			}

			if len(events) > 0 {
				r.mutex.Lock()
				r.steps = append(r.steps, events...)
				r.mutex.Unlock()

				// Send events via WebSocket if connected
				if r.wsConn != nil {
					for _, event := range events {
						eventData, _ := json.Marshal(event)
						r.wsConn.WriteMessage(websocket.TextMessage, eventData)
					}
				}
			}
		}
	}
}

func (r *ChromeRecorder) SetWebSocketConnection(conn *websocket.Conn) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.wsConn = conn
}

func (rm *RecorderManager) StartRecording(sessionID, targetURL string, device DeviceInfo) error {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	if _, exists := rm.recorders[sessionID]; exists {
		return fmt.Errorf("recording session %s already exists", sessionID)
	}

	recorder := NewChromeRecorder(sessionID, device)
	err := recorder.StartRecording(targetURL)
	if err != nil {
		return err
	}

	rm.recorders[sessionID] = recorder
	return nil
}

func (rm *RecorderManager) StopRecording(sessionID string) error {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	recorder, exists := rm.recorders[sessionID]
	if !exists {
		return fmt.Errorf("recording session %s not found", sessionID)
	}

	err := recorder.StopRecording()
	if err != nil {
		return err
	}

	// Don't delete the session here - keep it for saving
	// The session will be cleaned up when saving is complete
	return nil
}

func (rm *RecorderManager) GetRecorder(sessionID string) (*ChromeRecorder, bool) {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()

	recorder, exists := rm.recorders[sessionID]
	return recorder, exists
}

func (rm *RecorderManager) GetRecordingStatus(sessionID string) (bool, []RecordStep, error) {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()

	recorder, exists := rm.recorders[sessionID]
	if !exists {
		return false, nil, fmt.Errorf("recording session %s not found", sessionID)
	}

	return recorder.IsRecording(), recorder.GetSteps(), nil
}

func (rm *RecorderManager) CleanupRecording(sessionID string) error {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	if _, exists := rm.recorders[sessionID]; exists {
		delete(rm.recorders, sessionID)
	}
	return nil
}

func getRecordingScript() string {
	return `
(function() {
	if (window.autoUIRecorder) return;
	
	window.autoUIRecorder = {
		events: [],
		isRecording: true,
		
		addEvent: function(event) {
			if (this.isRecording) {
				this.events.push(event);
			}
		},
		
		getEvents: function() {
			const events = [...this.events];
			this.events = [];
			return events;
		},
		
		getSelector: function(element) {
			if (element.id) {
				return '#' + element.id;
			}
			
			let path = [];
			while (element && element.nodeType === Node.ELEMENT_NODE) {
				let selector = element.nodeName.toLowerCase();
				if (element.className) {
					selector += '.' + element.className.trim().split(/\s+/).join('.');
				}
				path.unshift(selector);
				element = element.parentNode;
			}
			return path.join(' > ');
		},
		
		getCoordinates: function(event) {
			const rect = event.target.getBoundingClientRect();
			return {
				x: event.clientX - rect.left,
				y: event.clientY - rect.top,
				pageX: event.pageX,
				pageY: event.pageY
			};
		}
	};
	
	// Click events
	document.addEventListener('click', function(event) {
		if (event.isTrusted) {
			window.autoUIRecorder.addEvent({
				type: 'click',
				selector: window.autoUIRecorder.getSelector(event.target),
				coordinates: window.autoUIRecorder.getCoordinates(event),
				timestamp: Date.now(),
				options: {
					button: event.button,
					detail: event.detail
				}
			});
		}
	}, true);
	
	// Input events
	document.addEventListener('input', function(event) {
		if (event.isTrusted && event.target.tagName) {
			const tagName = event.target.tagName.toLowerCase();
			if (tagName === 'input' || tagName === 'textarea') {
				window.autoUIRecorder.addEvent({
					type: 'input',
					selector: window.autoUIRecorder.getSelector(event.target),
					value: event.target.value,
					timestamp: Date.now(),
					options: {
						inputType: event.inputType
					}
				});
			}
		}
	}, true);
	
	// Key events
	document.addEventListener('keydown', function(event) {
		if (event.isTrusted) {
			window.autoUIRecorder.addEvent({
				type: 'keydown',
				selector: window.autoUIRecorder.getSelector(event.target),
				value: event.key,
				timestamp: Date.now(),
				options: {
					keyCode: event.keyCode,
					ctrlKey: event.ctrlKey,
					shiftKey: event.shiftKey,
					altKey: event.altKey,
					metaKey: event.metaKey
				}
			});
		}
	}, true);
	
	// Touch events for mobile simulation
	document.addEventListener('touchstart', function(event) {
		if (event.isTrusted) {
			const touch = event.touches[0];
			window.autoUIRecorder.addEvent({
				type: 'touchstart',
				selector: window.autoUIRecorder.getSelector(event.target),
				coordinates: {
					x: touch.clientX,
					y: touch.clientY,
					pageX: touch.pageX,
					pageY: touch.pageY
				},
				timestamp: Date.now(),
				options: {
					touchCount: event.touches.length
				}
			});
		}
	}, true);
	
	document.addEventListener('touchend', function(event) {
		if (event.isTrusted) {
			window.autoUIRecorder.addEvent({
				type: 'touchend',
				selector: window.autoUIRecorder.getSelector(event.target),
				timestamp: Date.now(),
				options: {
					touchCount: event.changedTouches.length
				}
			});
		}
	}, true);
	
	// Scroll events
	document.addEventListener('scroll', function(event) {
		if (event.isTrusted) {
			window.autoUIRecorder.addEvent({
				type: 'scroll',
				selector: window.autoUIRecorder.getSelector(event.target),
				coordinates: {
					scrollX: window.scrollX,
					scrollY: window.scrollY
				},
				timestamp: Date.now()
			});
		}
	}, true);
	
	// Form submission
	document.addEventListener('submit', function(event) {
		if (event.isTrusted) {
			window.autoUIRecorder.addEvent({
				type: 'submit',
				selector: window.autoUIRecorder.getSelector(event.target),
				timestamp: Date.now()
			});
		}
	}, true);
	
	// Select changes
	document.addEventListener('change', function(event) {
		if (event.isTrusted && event.target.tagName) {
			const tagName = event.target.tagName.toLowerCase();
			if (tagName === 'select' || tagName === 'input') {
				window.autoUIRecorder.addEvent({
					type: 'change',
					selector: window.autoUIRecorder.getSelector(event.target),
					value: event.target.value,
					timestamp: Date.now(),
					options: {
						type: event.target.type
					}
				});
			}
		}
	}, true);
	
	console.log('AutoUI Recorder initialized');
})();
`
}