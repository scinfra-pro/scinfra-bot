package webhook

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// Event represents a webhook event from switch-gate
type Event struct {
	Name      string                 `json:"event"`
	Timestamp time.Time              `json:"timestamp"`
	Source    string                 `json:"source"`
	Payload   map[string]interface{} `json:"payload"`
}

// handleSwitchGate handles webhooks from switch-gate
func (s *Server) handleSwitchGate(w http.ResponseWriter, r *http.Request) {
	// Only accept POST
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Validate secret
	if r.Header.Get("X-Webhook-Secret") != s.secret {
		log.Printf("WARN: Webhook unauthorized from %s", r.RemoteAddr)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse event
	var event Event
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("WARN: Webhook bad request: %v", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	log.Printf("INFO: Webhook received: %s from %s", event.Name, event.Source)

	// Format and send notification
	text := formatNotification(event)
	if text != "" {
		if err := s.notifier.SendNotification(text); err != nil {
			log.Printf("ERROR: Failed to send notification: %v", err)
		}
	}

	w.WriteHeader(http.StatusOK)
}

// formatNotification formats event into Telegram message
func formatNotification(event Event) string {
	source := capitalize(event.Source)

	switch event.Name {
	case "mode.changed":
		return formatModeChanged(source, event.Payload)
	case "limit.reached":
		return formatLimitReached(source, event.Payload)
	default:
		log.Printf("WARN: Unknown event type: %s", event.Name)
		return ""
	}
}

// formatModeChanged formats mode.changed event
func formatModeChanged(source string, payload map[string]interface{}) string {
	from := getStringPayload(payload, "from")
	to := getStringPayload(payload, "to")
	trigger := getStringPayload(payload, "trigger")

	icon := "🔄"
	if trigger == "limit_reached" {
		icon = "⚠️"
	}

	return fmt.Sprintf(`%s <b>%s VPS</b>

Mode: %s → %s`, icon, source, from, to)
}

// formatLimitReached formats limit.reached event
func formatLimitReached(source string, payload map[string]interface{}) string {
	usedMB := getFloatPayload(payload, "used_mb")
	limitMB := getFloatPayload(payload, "limit_mb")
	switchedTo := getStringPayload(payload, "switched_to")

	return fmt.Sprintf(`⚠️ <b>%s VPS</b>

Home limit reached: %.0f/%.0f MB
Auto-switched to: %s`, source, usedMB, limitMB, switchedTo)
}

// Helper functions

func getStringPayload(payload map[string]interface{}, key string) string {
	if val, exists := payload[key]; exists {
		if str, isString := val.(string); isString {
			return str
		}
	}
	return ""
}

func getFloatPayload(payload map[string]interface{}, key string) float64 {
	if val, ok := payload[key]; ok {
		switch v := val.(type) {
		case float64:
			return v
		case int:
			return float64(v)
		}
	}
	return 0
}

func capitalize(s string) string {
	if len(s) == 0 {
		return s
	}
	// Only capitalize ASCII lowercase letters
	if s[0] >= 'a' && s[0] <= 'z' {
		return string(s[0]-32) + s[1:]
	}
	return s
}
