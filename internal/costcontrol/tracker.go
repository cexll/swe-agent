package costcontrol

import (
	"log"
	"sync"
	"time"
)

// CostTracker tracks API costs and enforces limits
type CostTracker struct {
	mu                sync.RWMutex
	dailyCallLimit    int
	perIssueCostLimit float64
	alertThreshold    float64
	
	// Daily tracking
	dailyCalls     int
	dailyCost      float64
	dailyResetTime time.Time
	
	// Per-issue tracking
	issueCosts map[string]float64 // key: "repo#issue"
}

// NewCostTracker creates a new cost tracker
func NewCostTracker(dailyCallLimit int, perIssueCostLimit, alertThreshold float64) *CostTracker {
	now := time.Now()
	
	return &CostTracker{
		dailyCallLimit:    dailyCallLimit,
		perIssueCostLimit: perIssueCostLimit,
		alertThreshold:    alertThreshold,
		dailyResetTime:    time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location()),
		issueCosts:        make(map[string]float64),
	}
}

// CanMakeCall checks if a new call is allowed under the daily limit
func (ct *CostTracker) CanMakeCall() bool {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	
	ct.resetDailyIfNeeded()
	
	return ct.dailyCalls < ct.dailyCallLimit
}

// CanSpendIssue checks if spending on an issue is allowed under the per-issue limit
func (ct *CostTracker) CanSpendIssue(repo string, issueNumber int, additionalCost float64) bool {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	
	key := repo + "#" + string(rune(issueNumber))
	currentCost := ct.issueCosts[key]
	
	return (currentCost + additionalCost) <= ct.perIssueCostLimit
}

// RecordCost records the cost of an API call
func (ct *CostTracker) RecordCost(repo string, issueNumber int, cost float64) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	
	ct.resetDailyIfNeeded()
	
	// Update daily stats
	ct.dailyCalls++
	ct.dailyCost += cost
	
	// Update per-issue stats
	key := repo + "#" + string(rune(issueNumber))
	ct.issueCosts[key] += cost
	
	// Log alerts
	if ct.dailyCost >= ct.alertThreshold {
		log.Printf("COST ALERT: Daily cost %.2f exceeds threshold %.2f", ct.dailyCost, ct.alertThreshold)
	}
	
	if ct.issueCosts[key] >= ct.alertThreshold {
		log.Printf("COST ALERT: Issue %s cost %.2f exceeds threshold %.2f", key, ct.issueCosts[key], ct.alertThreshold)
	}
	
	log.Printf("Cost recorded: %s, cost: %.4f, daily total: %.2f (%d calls), issue total: %.2f", 
		key, cost, ct.dailyCost, ct.dailyCalls, ct.issueCosts[key])
}

// resetDailyIfNeeded resets daily counters if a new day has started
func (ct *CostTracker) resetDailyIfNeeded() {
	if time.Now().After(ct.dailyResetTime) {
		ct.dailyCalls = 0
		ct.dailyCost = 0
		ct.dailyResetTime = time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day()+1, 0, 0, 0, 0, time.Now().Location())
		
		// Clear old issue costs (keep only recent ones)
		if len(ct.issueCosts) > 1000 {
			ct.issueCosts = make(map[string]float64)
		}
		
		log.Printf("Daily cost tracking reset. Next reset: %s", ct.dailyResetTime.Format("2006-01-02 15:04:05"))
	}
}

// GetStats returns current cost statistics
func (ct *CostTracker) GetStats() DailyStats {
	ct.mu.RLock()
	defer ct.mu.RUnlock()
	
	ct.resetDailyIfNeeded()
	
	return DailyStats{
		DailyCalls:     ct.dailyCalls,
		DailyCost:      ct.dailyCost,
		DailyLimit:     ct.dailyCallLimit,
		CostAlertThreshold: ct.alertThreshold,
		NextResetTime:  ct.dailyResetTime,
	}
}

// DailyStats represents daily cost statistics
type DailyStats struct {
	DailyCalls        int       `json:"daily_calls"`
	DailyCost         float64   `json:"daily_cost"`
	DailyLimit        int       `json:"daily_limit"`
	CostAlertThreshold float64  `json:"cost_alert_threshold"`
	NextResetTime     time.Time `json:"next_reset_time"`
}

// CheckLimits checks both daily and per-issue limits and returns an error if exceeded
func (ct *CostTracker) CheckLimits(repo string, issueNumber int, estimatedCost float64) error {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	
	ct.resetDailyIfNeeded()
	
	// Check daily call limit
	if ct.dailyCalls >= ct.dailyCallLimit {
		return &LimitError{
			Type:    "daily_calls",
			Limit:   ct.dailyCallLimit,
			Current: ct.dailyCalls,
			Message: "Daily API call limit reached",
		}
	}
	
	// Check per-issue cost limit
	key := repo + "#" + string(rune(issueNumber))
	currentCost := ct.issueCosts[key]
	
	if (currentCost + estimatedCost) > ct.perIssueCostLimit {
		return &LimitError{
			Type:    "per_issue_cost",
			Limit:   ct.perIssueCostLimit,
			Current: currentCost,
			Message: "Per-issue cost limit would be exceeded",
		}
	}
	
	return nil
}

// LimitError represents a cost limit violation
type LimitError struct {
	Type    string
	Limit   interface{}
	Current interface{}
	Message string
}

func (e *LimitError) Error() string {
	return e.Message
}