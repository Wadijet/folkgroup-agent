package utility

import (
	"log"
	"sync"
	"time"
)

// ANSI color codes cho terminal
const (
	colorReset   = "\033[0m"
	colorRed     = "\033[31m"
	colorYellow  = "\033[33m"
	colorGreen   = "\033[32m"
	colorBlue    = "\033[34m"
	colorMagenta = "\033[35m"
	colorCyan    = "\033[36m"
)

// AdaptiveRateLimiter quản lý thời gian nghỉ động dựa trên phản ứng của server
type AdaptiveRateLimiter struct {
	mu                 sync.RWMutex
	currentDelay       time.Duration // Thời gian nghỉ hiện tại
	minDelay           time.Duration // Thời gian nghỉ tối thiểu
	maxDelay           time.Duration // Thời gian nghỉ tối đa
	successCount       int           // Số lần request thành công liên tiếp
	failureCount       int           // Số lần request thất bại liên tiếp
	backoffMultiplier  float64       // Hệ số tăng delay khi gặp lỗi
	recoveryMultiplier float64       // Hệ số giảm delay khi thành công
	successThreshold   int           // Số lần thành công cần để giảm delay
	lastAdjustmentTime time.Time     // Thời gian điều chỉnh lần cuối
	adjustmentCooldown time.Duration // Thời gian chờ giữa các lần điều chỉnh
}

var (
	// Global rate limiter instance cho Pancake API
	globalPancakeRateLimiter *AdaptiveRateLimiter
	oncePancake              sync.Once

	// Global rate limiter instance cho FolkForm API
	globalFolkFormRateLimiter *AdaptiveRateLimiter
	onceFolkForm              sync.Once
)

// NewAdaptiveRateLimiter tạo một rate limiter mới với cấu hình mặc định
// Tham số:
//   - initialDelay: Thời gian nghỉ ban đầu (mặc định: 100ms)
//   - minDelay: Thời gian nghỉ tối thiểu (mặc định: 50ms)
//   - maxDelay: Thời gian nghỉ tối đa (mặc định: 5s)
func NewAdaptiveRateLimiter(initialDelay, minDelay, maxDelay time.Duration) *AdaptiveRateLimiter {
	if initialDelay < minDelay {
		initialDelay = minDelay
	}
	if initialDelay > maxDelay {
		initialDelay = maxDelay
	}

	return &AdaptiveRateLimiter{
		currentDelay:       initialDelay,
		minDelay:           minDelay,
		maxDelay:           maxDelay,
		backoffMultiplier:  1.2,              // Tăng 20% mỗi lần gặp lỗi
		recoveryMultiplier: 0.9,              // Giảm 10% mỗi lần thành công
		successThreshold:   5,                // Cần 5 lần thành công để giảm delay
		adjustmentCooldown: 10 * time.Second, // Chỉ điều chỉnh mỗi 10 giây
		lastAdjustmentTime: time.Now(),
	}
}

// GetPancakeRateLimiter trả về instance global của rate limiter cho Pancake
// Pancake có rate limit chặt chẽ hơn, cần delay lớn hơn
func GetPancakeRateLimiter() *AdaptiveRateLimiter {
	oncePancake.Do(func() {
		// Khởi tạo với delay ban đầu 100ms, min 50ms, max 5s
		// Pancake thường bị rate limit nhanh hơn nên cần delay lớn hơn
		globalPancakeRateLimiter = NewAdaptiveRateLimiter(
			100*time.Millisecond,
			50*time.Millisecond,
			5*time.Second,
		)
		// Không log khởi tạo để giảm log
	})
	return globalPancakeRateLimiter
}

// GetFolkFormRateLimiter trả về instance global của rate limiter cho FolkForm
// FolkForm có rate limit khác Pancake, có thể cho phép request nhanh hơn
func GetFolkFormRateLimiter() *AdaptiveRateLimiter {
	onceFolkForm.Do(func() {
		// Khởi tạo với delay ban đầu 50ms, min 25ms, max 2s
		// FolkForm thường cho phép request nhanh hơn Pancake
		globalFolkFormRateLimiter = NewAdaptiveRateLimiter(
			50*time.Millisecond,
			25*time.Millisecond,
			2*time.Second,
		)
		// Không log khởi tạo để giảm log
	})
	return globalFolkFormRateLimiter
}

// Wait thực hiện nghỉ với thời gian hiện tại
func (rl *AdaptiveRateLimiter) Wait() {
	rl.mu.RLock()
	delay := rl.currentDelay
	rl.mu.RUnlock()

	time.Sleep(delay)
}

// GetCurrentDelay trả về thời gian nghỉ hiện tại
func (rl *AdaptiveRateLimiter) GetCurrentDelay() time.Duration {
	rl.mu.RLock()
	defer rl.mu.RUnlock()
	return rl.currentDelay
}

// RecordSuccess ghi nhận một request thành công và điều chỉnh delay nếu cần
func (rl *AdaptiveRateLimiter) RecordSuccess() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.successCount++
	rl.failureCount = 0

	// Chỉ điều chỉnh nếu đã đủ thời gian cooldown
	now := time.Now()
	if now.Sub(rl.lastAdjustmentTime) < rl.adjustmentCooldown {
		return
	}

	// Nếu có đủ số lần thành công liên tiếp, giảm delay
	if rl.successCount >= rl.successThreshold {
		newDelay := time.Duration(float64(rl.currentDelay) * rl.recoveryMultiplier)
		if newDelay < rl.minDelay {
			newDelay = rl.minDelay
		}

		if newDelay != rl.currentDelay {
			oldDelay := rl.currentDelay
			rl.currentDelay = newDelay
			rl.lastAdjustmentTime = now
			rl.successCount = 0 // Reset counter sau khi điều chỉnh
			// Chỉ log khi thay đổi đáng kể (> 50% hoặc giảm xuống minDelay)
			changePercent := float64(oldDelay-newDelay) / float64(oldDelay) * 100
			if changePercent > 50 || newDelay == rl.minDelay {
				var prefix string
				if rl == globalPancakeRateLimiter {
					prefix = colorCyan + "[Pancake RateLimiter]"
				} else if rl == globalFolkFormRateLimiter {
					prefix = colorMagenta + "[FolkForm RateLimiter]"
				} else {
					prefix = "[RateLimiter]"
				}
				log.Printf("%s %s✅ Request thành công → Giảm delay: %v → %v%s",
					prefix, colorGreen, oldDelay, newDelay, colorReset)
			}
		}
	}
}

// RecordFailure ghi nhận một request thất bại và tăng delay
// Tham số:
//   - statusCode: HTTP status code từ response
//   - errorCode: Error code từ Pancake API (nếu có)
func (rl *AdaptiveRateLimiter) RecordFailure(statusCode int, errorCode interface{}) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.failureCount++
	rl.successCount = 0

	// Kiểm tra xem có phải rate limit/quá tải không
	// CHỈ điều chỉnh rate limit khi gặp status 429 hoặc error_code 429
	isRateLimit := false

	// Chỉ coi là rate limit khi gặp status code 429
	if statusCode == 429 {
		isRateLimit = true
	}

	// Kiểm tra error codes từ Pancake API (nếu có)
	// Pancake trả về error_code 429 trong response body khi quá tải
	if errorCode != nil {
		// Convert errorCode sang số để so sánh
		var errCodeNum float64
		switch v := errorCode.(type) {
		case float64:
			errCodeNum = v
		case int:
			errCodeNum = float64(v)
		case int64:
			errCodeNum = float64(v)
		}

		// Error code 429 từ Pancake API cho biết quá tải
		// Response: {"message":"Too many requests","success":false,"error_code":429}
		if errCodeNum == 429 {
			isRateLimit = true
		}
	}

	// CHỈ điều chỉnh rate limit khi gặp status 429 hoặc error_code 429
	// Không điều chỉnh cho các status code khác (400, 401, 404, 500, 503, etc.)
	now := time.Now()
	shouldAdjust := isRateLimit

	if shouldAdjust {
		// Tăng delay khi gặp lỗi
		newDelay := time.Duration(float64(rl.currentDelay) * rl.backoffMultiplier)
		if newDelay > rl.maxDelay {
			newDelay = rl.maxDelay
		}

		if newDelay != rl.currentDelay {
			oldDelay := rl.currentDelay
			rl.currentDelay = newDelay
			rl.lastAdjustmentTime = now
			rl.failureCount = 0 // Reset counter sau khi điều chỉnh

			// Chỉ log khi rate limit (quan trọng) hoặc thay đổi đáng kể (> 50% hoặc đạt maxDelay)
			changePercent := float64(newDelay-oldDelay) / float64(oldDelay) * 100
			shouldLog := isRateLimit || changePercent > 50 || newDelay == rl.maxDelay

			if shouldLog {
				var prefix string
				if rl == globalPancakeRateLimiter {
					prefix = colorCyan + "[Pancake RateLimiter]"
				} else if rl == globalFolkFormRateLimiter {
					prefix = colorMagenta + "[FolkForm RateLimiter]"
				} else {
					prefix = "[RateLimiter]"
				}

				var colorCode, rateLimitMsg string
				if isRateLimit {
					colorCode = colorRed
					rateLimitMsg = " (RATE LIMIT - QUÁ TẢI)"
				} else {
					colorCode = colorYellow
					rateLimitMsg = ""
				}
				log.Printf("%s %s⚠️ Request thất bại%s → Tăng delay: %v → %v (status: %d)%s",
					prefix, colorCode, rateLimitMsg, oldDelay, newDelay, statusCode, colorReset)
			}
		}
	}
}

// RecordResponse ghi nhận kết quả của một request và tự động điều chỉnh
// Tham số:
//   - statusCode: HTTP status code từ response
//   - success: true nếu request thành công (status 200 và success=true trong response)
//   - errorCode: Error code từ Pancake API (nếu có)
func (rl *AdaptiveRateLimiter) RecordResponse(statusCode int, success bool, errorCode interface{}) {
	if success && statusCode == 200 {
		rl.RecordSuccess()
	} else {
		rl.RecordFailure(statusCode, errorCode)
	}
}

// Reset đặt lại rate limiter về trạng thái ban đầu
func (rl *AdaptiveRateLimiter) Reset() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.currentDelay = rl.minDelay
	rl.successCount = 0
	rl.failureCount = 0
	rl.lastAdjustmentTime = time.Now()
	// Không log reset để giảm log
}

// GetStats trả về thống kê hiện tại của rate limiter
func (rl *AdaptiveRateLimiter) GetStats() map[string]interface{} {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	return map[string]interface{}{
		"current_delay":        rl.currentDelay.String(),
		"min_delay":            rl.minDelay.String(),
		"max_delay":            rl.maxDelay.String(),
		"success_count":        rl.successCount,
		"failure_count":        rl.failureCount,
		"last_adjustment_time": rl.lastAdjustmentTime.Format("2006-01-02 15:04:05"),
	}
}
