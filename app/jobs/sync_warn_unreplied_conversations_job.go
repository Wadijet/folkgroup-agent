/*
Package jobs chứa các job cụ thể của ứng dụng.
File này chứa SyncWarnUnrepliedConversationsJob - job cảnh báo các hội thoại chưa được trả lời trong vòng 5-300 phút.
*/
package jobs

import (
	"agent_pancake/app/integrations"
	"agent_pancake/app/scheduler"
	"agent_pancake/global"
	"context"
	"errors"
	"os"
	"strings"
	"time"

	apputility "agent_pancake/app/utility"

	"github.com/sirupsen/logrus"
)

// notificationRateLimitMinutes sẽ được lấy từ config trong hàm DoSyncWarnUnrepliedConversations_v2()

// Sử dụng global.NotificationRateLimiter thay vì local variable để dùng chung giữa các phần của ứng dụng
// Tương tự như global.PanCake_FbPages

// SyncWarnUnrepliedConversationsJob là job cảnh báo các hội thoại chưa được trả lời trong vòng 5-300 phút.
// Job này sẽ:
// - Lấy danh sách conversations từ FolkForm
// - Kiểm tra các điều kiện: thời gian trễ 5-300 phút, không có tag spam/block, khách gửi tin cuối
// - Gửi cảnh báo qua notification system của FolkForm
type SyncWarnUnrepliedConversationsJob struct {
	*scheduler.BaseJob
}

// NewSyncWarnUnrepliedConversationsJob tạo một instance mới của SyncWarnUnrepliedConversationsJob.
// Tham số:
// - name: Tên định danh của job
// - schedule: Biểu thức cron định nghĩa lịch chạy
// Trả về một instance của SyncWarnUnrepliedConversationsJob
func NewSyncWarnUnrepliedConversationsJob(name, schedule string) *SyncWarnUnrepliedConversationsJob {
	job := &SyncWarnUnrepliedConversationsJob{
		BaseJob: scheduler.NewBaseJob(name, schedule),
	}
	// Set callback function để BaseJob.Execute có thể gọi ExecuteInternal đúng cách
	job.BaseJob.SetExecuteInternalCallback(job.ExecuteInternal)
	return job
}

// ExecuteInternal thực thi logic cảnh báo hội thoại chưa trả lời.
// Phương thức này gọi DoWarnUnrepliedConversations() và thêm log wrapper cho job.
// Tham số:
// - ctx: Context để kiểm soát thời gian thực thi
// Trả về error nếu có lỗi xảy ra
func (j *SyncWarnUnrepliedConversationsJob) ExecuteInternal(ctx context.Context) error {
	startTime := time.Now()
	processId := os.Getpid()

	// Log số lượng entry trong rate limiter khi job bắt đầu
	global.NotificationRateLimiterMu.RLock()
	rateLimiterSize := len(global.NotificationRateLimiter)
	global.NotificationRateLimiterMu.RUnlock()

	LogJobStart(j.GetName(), j.GetSchedule()).WithFields(map[string]interface{}{
		"start_time":        startTime.Format("2006-01-02 15:04:05"),
		"process_id":        processId,
		"rate_limiter_size": rateLimiterSize,
	}).Info("🚀 JOB ĐÃ BẮT ĐẦU CHẠY")

	// Gọi hàm logic thực sự
	err := DoWarnUnrepliedConversations()
	duration := time.Since(startTime)
	durationMs := duration.Milliseconds()

	// Log số lượng entry trong rate limiter khi job kết thúc
	global.NotificationRateLimiterMu.RLock()
	rateLimiterSize = len(global.NotificationRateLimiter)
	// Lấy danh sách conversationId trong rate limiter để log
	conversationIds := make([]string, 0, rateLimiterSize)
	for convId := range global.NotificationRateLimiter {
		conversationIds = append(conversationIds, convId)
	}
	global.NotificationRateLimiterMu.RUnlock()

	if err != nil {
		jobLogger := GetJobLoggerByName(j.GetName())
		jobLogger.WithError(err).WithFields(map[string]interface{}{
			"process_id":        processId,
			"rate_limiter_size": rateLimiterSize,
			"conversation_ids":  conversationIds,
			"duration":          duration.String(),
			"duration_ms":       durationMs,
		}).Error("❌ JOB LỖI")
		LogJobError(j.GetName(), err, duration.String(), durationMs)
		return err
	}

	jobLogger := GetJobLoggerByName(j.GetName())
	jobLogger.WithFields(map[string]interface{}{
		"process_id":        processId,
		"rate_limiter_size": rateLimiterSize,
		"conversation_ids":  conversationIds,
		"duration":          duration.String(),
		"duration_ms":       durationMs,
	}).Info("✅ JOB HOÀN THÀNH")

	LogJobEnd(j.GetName(), duration.String(), durationMs)
	return nil
}

// DoWarnUnrepliedConversations thực thi logic cảnh báo hội thoại chưa trả lời.
// Hàm này:
// - Lấy danh sách conversations từ FolkForm cho tất cả pages
// - Kiểm tra các điều kiện: thời gian trễ 5-300 phút, không có tag spam/block, khách gửi tin cuối
// - Gửi cảnh báo qua notification system của FolkForm
// Hàm này có thể được gọi độc lập mà không cần thông qua job interface.
// Trả về error nếu có lỗi xảy ra
func DoWarnUnrepliedConversations() error {
	// Lấy logger riêng cho job này
	// File log sẽ là: logs/sync-warn-unreplied-conversations-job.log
	jobLogger := GetJobLoggerByName("sync-warn-unreplied-conversations-job")

	// Kiểm tra khung giờ làm việc: Từ 8h30 sáng đến 10h30 tối (22:30)
	// Ngoài giờ đó không báo nữa
	now := time.Now()
	currentHour := now.Hour()
	currentMinute := now.Minute()
	currentTimeStr := now.Format("15:04")

	// Giờ bắt đầu: 8h30 (08:30)
	workStartHour := 8
	workStartMinute := 30

	// Giờ kết thúc: 22h30 (10h30 tối)
	workEndHour := 22
	workEndMinute := 30

	// Kiểm tra xem có trong khung giờ làm việc không
	isWorkingHours := false

	// Tính thời gian hiện tại dưới dạng phút từ 00:00
	currentTimeMinutes := currentHour*60 + currentMinute
	workStartMinutes := workStartHour*60 + workStartMinute
	workEndMinutes := workEndHour*60 + workEndMinute

	if currentTimeMinutes >= workStartMinutes && currentTimeMinutes <= workEndMinutes {
		isWorkingHours = true
	}

	if !isWorkingHours {
		jobLogger.WithFields(map[string]interface{}{
			"current_time": currentTimeStr,
			"work_start":   "08:30",
			"work_end":     "22:30",
		}).Info("⏰ Ngoài khung giờ làm việc (8h30 - 22h30), bỏ qua job cảnh báo")
		return nil // Không có lỗi, chỉ là skip job
	}

	jobLogger.WithFields(map[string]interface{}{
		"current_time": currentTimeStr,
		"work_start":   "08:30",
		"work_end":     "22:30",
	}).Info("✅ Trong khung giờ làm việc, tiếp tục chạy job cảnh báo")

	// Kiểm tra token - nếu chưa có thì bỏ qua, đợi CheckInJob login
	if !EnsureApiToken() {
		jobLogger.Debug("Chưa có token, bỏ qua job này. Đợi CheckInJob login...")
		return nil
	}

	// Cleanup rate limiter: Xóa các entry cũ hơn 5 phút (không phải reset toàn bộ)
	// Điều này đảm bảo mỗi lần agent restart, chỉ xóa các entry đã hết hạn
	// ========================================
	// LẤY CẤU HÌNH TỪ CONFIG ĐỘNG
	// ========================================
	// Tất cả các giá trị này có thể được thay đổi từ server mà không cần restart bot
	// Nếu không có config, sử dụng default values
	// Config được gửi lên server trong check-in request và có thể được cập nhật từ server

	// minDelayMinutes: Thời gian trễ tối thiểu (phút) để gửi cảnh báo
	// Conversations chưa trả lời dưới thời gian này sẽ không được cảnh báo
	minDelayMinutes := GetJobConfigInt("sync-warn-unreplied-conversations-job", "minDelayMinutes", 5)

	// maxDelayMinutes: Thời gian trễ tối đa (phút) để gửi cảnh báo
	// Conversations chưa trả lời quá thời gian này sẽ không được cảnh báo (có thể đã quá cũ)
	maxDelayMinutes := GetJobConfigInt("sync-warn-unreplied-conversations-job", "maxDelayMinutes", 300)

	// pageSize: Số lượng conversations được kiểm tra mỗi lần gọi API
	// Tăng giá trị này để kiểm tra nhiều conversations hơn nhưng tốn nhiều bộ nhớ hơn
	pageSize := GetJobConfigInt("sync-warn-unreplied-conversations-job", "pageSize", 50)

	// notificationRateLimitMinutes: Thời gian tối thiểu giữa các lần gửi notification cho cùng một conversation (phút)
	// Tránh spam notification cho cùng một conversation
	// Ví dụ: Nếu đã gửi notification 3 phút trước, phải đợi thêm 2 phút nữa mới gửi lại
	notificationRateLimitMinutes := GetJobConfigInt("sync-warn-unreplied-conversations-job", "notificationRateLimitMinutes", 5)

	cleanupRateLimiter(notificationRateLimitMinutes, jobLogger)

	// Đảm bảo notification template và routing rule đã được tạo
	// Sẽ tự động lấy organizationIds từ role hiện tại
	eventType := "conversation_unreplied"
	err := integrations.FolkForm_EnsureNotificationSetup(eventType, []string{})
	if err != nil {
		jobLogger.WithError(err).Warn("Lưu ý: Có thể notification setup đã tồn tại hoặc có lỗi khi tạo")
		// Không return error, tiếp tục chạy job
	}

	jobLogger.WithFields(map[string]interface{}{
		"minDelayMinutes":              minDelayMinutes,
		"maxDelayMinutes":              maxDelayMinutes,
		"pageSize":                     pageSize,
		"notificationRateLimitMinutes": notificationRateLimitMinutes,
	}).Info("Bắt đầu kiểm tra và cảnh báo hội thoại chưa trả lời...")

	// Lấy tất cả pages từ FolkForm
	limit := pageSize
	page := 1
	totalWarned := 0

	for {
		// Lấy danh sách các pages từ server FolkForm
		resultPages, err := integrations.FolkForm_GetFbPages(page, limit)
		if err != nil {
			jobLogger.WithError(err).Error("❌ Lỗi khi lấy danh sách trang Facebook")
			return errors.New("Lỗi khi lấy danh sách trang Facebook")
		}

		// Xử lý response - có thể là pagination object hoặc array trực tiếp
		// Sử dụng helper function từ helpers.go
		// Log response để debug nếu có lỗi
		if resultPages == nil {
			jobLogger.Error("❌ Response từ API là nil")
			return errors.New("Response từ API là nil")
		}

		items, itemCount, err := parseResponseDataHelper(resultPages)
		if err != nil {
			// Log chi tiết response để debug
			jobLogger.WithError(err).WithFields(map[string]interface{}{
				"resultPages":      resultPages,
				"resultPages_keys": getMapKeys(resultPages),
			}).Error("❌ LỖI khi parse response")
			return err
		}

		if itemCount == 0 || len(items) == 0 {
			jobLogger.Info("Không còn pages nào, dừng kiểm tra")
			break
		}

		jobLogger.WithFields(map[string]interface{}{
			"page":  page,
			"limit": limit,
			"count": len(items),
		}).Info("Nhận được pages")

		// Với mỗi page
		for _, item := range items {
			pageMap, ok := item.(map[string]interface{})
			if !ok {
				jobLogger.Warn("Page không phải là map, bỏ qua")
				continue
			}

			pageId, ok := pageMap["pageId"].(string)
			if !ok || pageId == "" {
				jobLogger.Warn("Page không có pageId, bỏ qua")
				continue
			}

			// Lấy pageUsername từ page data
			// Thử nhiều field names có thể có
			pageUsername, _ := pageMap["pageUsername"].(string)
			if pageUsername == "" {
				pageUsername, _ = pageMap["username"].(string)
			}
			if pageUsername == "" {
				pageUsername, _ = pageMap["page_username"].(string)
			}
			// Nếu vẫn không có, thử lấy từ API
			if pageUsername == "" {
				jobLogger.WithField("pageId", pageId).Info("Page không có pageUsername trong response, đang lấy từ API...")
				pageData, err := integrations.FolkForm_GetFbPageByPageId(pageId)
				if err == nil {
					if dataMap, ok := pageData["data"].(map[string]interface{}); ok {
						if username, ok := dataMap["pageUsername"].(string); ok && username != "" {
							pageUsername = username
						} else if username, ok := dataMap["username"].(string); ok && username != "" {
							pageUsername = username
						}
					}
				}
				if pageUsername == "" {
					jobLogger.WithField("pageId", pageId).Warn("Không thể lấy pageUsername từ API, sẽ dùng pageId thay thế")
					pageUsername = pageId // Fallback: dùng pageId nếu không có username
				}
			}

			isSync, _ := pageMap["isSync"].(bool)

			if !isSync {
				jobLogger.WithField("pageId", pageId).Info("Page không sync (isSync=false), bỏ qua")
				continue
			}

			// Kiểm tra và cảnh báo conversations chưa trả lời cho page này
			warnedCount, err := warnUnrepliedConversationsForPage(pageId, pageUsername, minDelayMinutes, maxDelayMinutes, notificationRateLimitMinutes, jobLogger)
			if err != nil {
				jobLogger.WithError(err).WithField("pageId", pageId).Error("Lỗi khi kiểm tra conversations cho page")
				// Tiếp tục với page tiếp theo, không dừng
				continue
			}

			totalWarned += warnedCount
		}

		// Kiểm tra xem còn pages không
		if len(items) < limit {
			break
		}

		page++
	}

	jobLogger.WithField("total_warned", totalWarned).Info("✅ Hoàn thành kiểm tra và cảnh báo hội thoại chưa trả lời")
	return nil
}

// warnUnrepliedConversationsForPage kiểm tra và cảnh báo conversations chưa trả lời cho một page
// Tham số:
// - pageId: ID của page
// - pageUsername: Username của page
// - delayWarningMinMinutes: Thời gian trễ tối thiểu để cảnh báo (phút)
// - delayWarningMaxMinutes: Thời gian trễ tối đa để cảnh báo (phút)
// - notificationRateLimitMinutes: Thời gian tối thiểu giữa các lần gửi notification (phút)
// - jobLogger: Logger riêng cho job
// Trả về số lượng conversations đã cảnh báo và error
func warnUnrepliedConversationsForPage(pageId string, pageUsername string, delayWarningMinMinutes int, delayWarningMaxMinutes int, notificationRateLimitMinutes int, jobLogger *logrus.Logger) (int, error) {
	jobLogger.WithFields(map[string]interface{}{
		"pageId":                 pageId,
		"pageUsername":           pageUsername,
		"delayWarningMinMinutes": delayWarningMinMinutes,
		"delayWarningMaxMinutes": delayWarningMaxMinutes,
	}).Info("Bắt đầu kiểm tra conversations chưa trả lời cho page")

	// Lấy conversations từ FolkForm với pagination
	page := 1
	// Lấy pageSize từ config động (số lượng conversations lấy mỗi lần gọi API)
	// Nếu không có config, sử dụng default value 60
	// Config này có thể được thay đổi từ server mà không cần restart bot
	limit := GetJobConfigInt("sync-warn-unreplied-conversations-job", "pageSize", 60)
	warnedCount := 0
	rateLimiter := apputility.GetFolkFormRateLimiter()

	for {
		// Áp dụng Rate Limiter
		rateLimiter.Wait()

		// Lấy conversations chưa trả lời từ FolkForm với filter tối ưu
		// Chỉ lấy conversations có updated_at trong khoảng 5-300 phút trước
		result, err := integrations.FolkForm_GetUnrepliedConversationsWithPageId(page, limit, pageId, delayWarningMinMinutes, delayWarningMaxMinutes)
		if err != nil {
			jobLogger.WithError(err).Error("Lỗi khi lấy conversations từ FolkForm")
			return warnedCount, err
		}

		// Parse conversations từ response
		var items []interface{}
		if dataMap, ok := result["data"].(map[string]interface{}); ok {
			if itemsArray, ok := dataMap["items"].([]interface{}); ok {
				items = itemsArray
			}
		} else if dataArray, ok := result["data"].([]interface{}); ok {
			items = dataArray
		}

		if len(items) == 0 {
			jobLogger.WithField("pageId", pageId).Info("Không còn conversations nào")
			break
		}

		jobLogger.WithFields(map[string]interface{}{
			"pageId": pageId,
			"page":   page,
			"count":  len(items),
		}).Info("Lấy được conversations từ FolkForm")

		// Log một vài conversation đầu tiên để debug
		if len(items) > 0 {
			logCount := 3
			if len(items) < logCount {
				logCount = len(items)
			}
			for i := 0; i < logCount; i++ {
				if itemMap, ok := items[i].(map[string]interface{}); ok {
					convId, _ := itemMap["conversationId"].(string)
					if convId == "" {
						if id, ok := itemMap["id"].(string); ok {
							convId = id
						}
					}
					pageUser, _ := itemMap["pageUsername"].(string)
					jobLogger.WithFields(map[string]interface{}{
						"index":          i,
						"conversationId": convId,
						"pageUsername":   pageUser,
					}).Debug("Sample conversation từ API")
				}
			}
		}

		// Kiểm tra từng conversation
		// Lưu ý: Filter đã được áp dụng ở API level cho:
		// - updated_at trong khoảng 5-300 phút trước
		// - Không có tag spam hoặc "khách block"
		// Cần kiểm tra thêm ở application level:
		// - last_sent_by.id != pageId (khách gửi tin cuối, chưa được trả lời)
		for _, item := range items {
			itemMap, ok := item.(map[string]interface{})
			if !ok {
				continue
			}

			// Lấy pageUsername từ conversation data nếu chưa có từ page data
			// (Một số conversation có thể có pageUsername trong response)
			currentPageUsername := pageUsername
			if currentPageUsername == "" || currentPageUsername == pageId {
				if convPageUsername, ok := itemMap["pageUsername"].(string); ok && convPageUsername != "" {
					currentPageUsername = convPageUsername
					jobLogger.WithFields(map[string]interface{}{
						"pageId":       pageId,
						"pageUsername": currentPageUsername,
					}).Debug("Lấy pageUsername từ conversation data")
				}
			}

			// Lấy thông tin conversation
			conversationId, _ := itemMap["conversationId"].(string)
			if conversationId == "" {
				// Thử field "id" nếu không có "conversationId"
				if id, ok := itemMap["id"].(string); ok && id != "" {
					conversationId = id
				} else {
					continue
				}
			}

			// Lấy thông tin customer
			customerName := "Unknown"
			if panCakeData, ok := itemMap["panCakeData"].(map[string]interface{}); ok {
				if from, ok := panCakeData["from"].(map[string]interface{}); ok {
					if name, ok := from["name"].(string); ok {
						customerName = name
					}
				}
				if pageCustomer, ok := panCakeData["page_customer"].(map[string]interface{}); ok {
					if name, ok := pageCustomer["name"].(string); ok {
						customerName = name
					}
				}
			}

			// Lấy loại conversation
			conversationType := "Unknown"
			if panCakeData, ok := itemMap["panCakeData"].(map[string]interface{}); ok {
				if convType, ok := panCakeData["type"].(string); ok {
					conversationType = convType
				}
			}

			// Lấy updated_at
			var updatedAt time.Time
			if panCakeData, ok := itemMap["panCakeData"].(map[string]interface{}); ok {
				// Thử parse từ updated_at string
				if updatedAtStr, ok := panCakeData["updated_at"].(string); ok {
					// Parse ISO 8601 format
					parsedTime, err := time.Parse("2006-01-02T15:04:05.000000", updatedAtStr)
					if err != nil {
						// Thử format khác
						parsedTime, err = time.Parse("2006-01-02T15:04:05", updatedAtStr)
						if err != nil {
							// Thử RFC3339
							parsedTime, err = time.Parse(time.RFC3339, updatedAtStr)
							if err != nil {
								continue
							}
						}
					}
					updatedAt = parsedTime
				} else if updatedAtMs, ok := panCakeData["updated_at"].(float64); ok {
					// Nếu là milliseconds (Unix timestamp)
					updatedAt = time.Unix(int64(updatedAtMs)/1000, 0)
				} else {
					continue
				}
			} else {
				continue
			}

			// Lấy last_sent_by để kiểm tra
			var lastSentById string
			if panCakeData, ok := itemMap["panCakeData"].(map[string]interface{}); ok {
				if lastSentBy, ok := panCakeData["last_sent_by"].(map[string]interface{}); ok {
					if id, ok := lastSentBy["id"].(string); ok {
						lastSentById = id
					}
				}
			}

			// Kiểm tra: last_sent_by.id phải khác pageId (khách gửi tin cuối, chưa được trả lời)
			// Lưu ý: Không thể filter ở database level vì backend không hỗ trợ $ne
			if lastSentById == pageId {
				continue // Page đã trả lời cuối cùng → bỏ qua
			}

			// Tính thời gian trễ (phút) - để hiển thị trong notification
			now := time.Now()
			delayTime := now.Sub(updatedAt)
			delayMinutes := int(delayTime.Minutes())

			// QUAN TRỌNG: Chỉ cảnh báo nếu conversation được cập nhật gần đây (updatedAt < 5 phút)
			// Điều này tránh cảnh báo nhầm cho các conversation đã được sync chậm
			// Nếu updatedAt quá cũ (> 5 phút), có thể conversation chưa được sync, bỏ qua cảnh báo
			if delayTime > 5*time.Minute {
				jobLogger.WithFields(map[string]interface{}{
					"pageId":         pageId,
					"conversationId": conversationId,
					"pageUsername":   currentPageUsername,
					"delayMinutes":   delayMinutes,
					"updatedAt":      updatedAt.Format("2006-01-02 15:04:05"),
				}).Info("⏭️ Bỏ qua conversation vì updatedAt quá cũ (> 5 phút), có thể chưa được sync")
				continue
			}

			// Lấy tags để hiển thị và kiểm tra spam/block
			var tagTexts []string
			if panCakeData, ok := itemMap["panCakeData"].(map[string]interface{}); ok {
				if tags, ok := panCakeData["tags"].([]interface{}); ok {
					for _, tag := range tags {
						if tagMap, ok := tag.(map[string]interface{}); ok {
							if text, ok := tagMap["text"].(string); ok {
								tagTexts = append(tagTexts, strings.ToLower(text))
							}
						}
					}
				}
			}

			// Kiểm tra tag spam và "khách block" - bỏ qua nếu có
			hasSpamTag := false
			hasBlockTag := false
			for _, tagText := range tagTexts {
				if tagText == "spam" {
					hasSpamTag = true
				}
				if tagText == "khách block" {
					hasBlockTag = true
				}
			}

			if hasSpamTag || hasBlockTag {
				jobLogger.WithFields(map[string]interface{}{
					"pageId":         pageId,
					"conversationId": conversationId,
					"pageUsername":   currentPageUsername,
					"customerName":   customerName,
					"tags":           strings.Join(tagTexts, ", "),
					"hasSpamTag":     hasSpamTag,
					"hasBlockTag":    hasBlockTag,
				}).Info("⏭️ Bỏ qua conversation có tag spam hoặc khách block")
				continue
			}

			// Bỏ qua nếu là dữ liệu test
			if conversationId == "test_conversation_123" ||
				currentPageUsername == "test_page_username" ||
				customerName == "Khách hàng Test" ||
				strings.Contains(conversationId, "test_") ||
				strings.Contains(currentPageUsername, "test_") {
				jobLogger.WithFields(map[string]interface{}{
					"pageId":         pageId,
					"conversationId": conversationId,
					"pageUsername":   currentPageUsername,
					"customerName":   customerName,
				}).Warn("⚠️ Bỏ qua dữ liệu test, không gửi cảnh báo")
				continue
			}

			// Tất cả điều kiện đã thỏa mãn → kiểm tra rate limiting và notification đã tồn tại
			jobLogger.WithFields(map[string]interface{}{
				"pageId":           pageId,
				"conversationId":   conversationId,
				"pageUsername":     currentPageUsername,
				"customerName":     customerName,
				"delayMinutes":     delayMinutes,
				"conversationType": conversationType,
			}).Info("⚠️ Phát hiện hội thoại chưa trả lời, đang kiểm tra trước khi gửi cảnh báo")

			// Kiểm tra rate limiting: Dùng local rate limiter để tránh spam
			// List conversation IDs với thời gian gửi gần nhất
			// Nếu thời gian đó < 5 phút thì không gửi nữa
			now = time.Now()
			processId := os.Getpid()

			// QUAN TRỌNG: Kiểm tra và cập nhật rate limiter trong cùng 1 lock để tránh race condition
			// Điều này đảm bảo nếu cùng conversationId xuất hiện nhiều lần trong cùng 1 lần chạy,
			// chỉ gửi notification 1 lần duy nhất
			// Sử dụng global.NotificationRateLimiter để dùng chung giữa các phần của ứng dụng
			global.NotificationRateLimiterMu.Lock()
			lastSentTime, exists := global.NotificationRateLimiter[conversationId]
			shouldSkip := false
			rateLimiterSizeBefore := len(global.NotificationRateLimiter)

			// Log chi tiết để debug
			jobLogger.WithFields(map[string]interface{}{
				"conversationId":      conversationId,
				"processId":           processId,
				"existsInRateLimiter": exists,
				"rateLimiterSize":     rateLimiterSizeBefore,
			}).Info("🔍 Kiểm tra rate limiter cho conversation")

			if exists {
				// Đã có trong list, kiểm tra thời gian đã trôi qua
				timeSinceLastSent := now.Sub(lastSentTime)
				timeSinceLastSentMinutes := int(timeSinceLastSent.Minutes())
				timeSinceLastSentSeconds := int(timeSinceLastSent.Seconds())

				jobLogger.WithFields(map[string]interface{}{
					"conversationId":           conversationId,
					"processId":                processId,
					"lastSentTime":             lastSentTime.Format("2006-01-02 15:04:05.000"),
					"timeSinceLastSentSeconds": timeSinceLastSentSeconds,
					"timeSinceLastSentMinutes": timeSinceLastSentMinutes,
					"rateLimitMinutes":         notificationRateLimitMinutes,
				}).Debug("🔍 Conversation đã có trong rate limiter, kiểm tra thời gian")

				if timeSinceLastSent < time.Duration(notificationRateLimitMinutes)*time.Minute {
					// Chưa đủ 5 phút → bỏ qua
					shouldSkip = true
					remainingMinutes := notificationRateLimitMinutes - timeSinceLastSentMinutes
					remainingSeconds := (notificationRateLimitMinutes * 60) - timeSinceLastSentSeconds
					jobLogger.WithFields(map[string]interface{}{
						"conversationId":           conversationId,
						"processId":                processId,
						"lastSentTime":             lastSentTime.Format("2006-01-02 15:04:05.000"),
						"timeSinceLastSentSeconds": timeSinceLastSentSeconds,
						"timeSinceLastSentMinutes": timeSinceLastSentMinutes,
						"rateLimitMinutes":         notificationRateLimitMinutes,
						"remainingMinutes":         remainingMinutes,
						"remainingSeconds":         remainingSeconds,
					}).Warn("⏭️ BỎ QUA: Conversation đã gửi notification gần đây, cần đợi thêm")
				} else {
					// Đã đủ 5 phút → cho phép gửi (KHÔNG cập nhật rate limiter ở đây)
					jobLogger.WithFields(map[string]interface{}{
						"conversationId":           conversationId,
						"processId":                processId,
						"lastSentTime":             lastSentTime.Format("2006-01-02 15:04:05.000"),
						"timeSinceLastSentSeconds": timeSinceLastSentSeconds,
						"timeSinceLastSentMinutes": timeSinceLastSentMinutes,
					}).Info("✅ Đã đủ thời gian, cho phép gửi notification")
				}
			} else {
				// Chưa có trong list → cho phép gửi (KHÔNG cập nhật rate limiter ở đây)
				jobLogger.WithFields(map[string]interface{}{
					"conversationId": conversationId,
					"processId":      processId,
				}).Info("✅ Conversation chưa có trong list, cho phép gửi notification")
			}

			global.NotificationRateLimiterMu.Unlock()

			// Nếu bỏ qua, continue
			if shouldSkip {
				continue // Bỏ qua conversation này, tiếp tục với conversation tiếp theo
			}

			// Tạo link đến conversation
			conversationLink := ""
			if currentPageUsername != "" && currentPageUsername != pageId {
				conversationLink = "https://pancake.vn/" + currentPageUsername + "?c_id=" + conversationId
			} else {
				// Fallback: dùng pageId nếu không có username
				conversationLink = "https://pancake.vn/" + pageId + "?c_id=" + conversationId
			}

			// Tạo payload cho notification
			// Format tags: join bằng ", " để hiển thị trong notification
			tagsDisplay := ""
			if len(tagTexts) > 0 {
				tagsDisplay = strings.Join(tagTexts, ", ")
			} else {
				tagsDisplay = "Không có tag"
			}

			payload := map[string]interface{}{
				"eventType":        "conversation_unreplied", // Thêm eventType cho webhook template
				"conversationId":   conversationId,
				"pageId":           pageId,
				"pageUsername":     currentPageUsername,
				"customerName":     customerName,
				"conversationType": conversationType,
				"minutes":          delayMinutes,
				"updatedAt":        updatedAt.Format("2006-01-02 15:04:05"),
				"conversationLink": conversationLink,
				"tags":             tagsDisplay, // Tags để hiển thị trong notification
			}

			// Log payload trước khi gửi để debug
			// Thêm process ID và timestamp chính xác (milliseconds) để xác định có nhiều instances chạy cùng lúc không
			nowWithMs := time.Now()
			jobLogger.WithFields(map[string]interface{}{
				"conversationId": conversationId,
				"pageId":         pageId,
				"processId":      processId,
				"timestamp":      nowWithMs.Format("2006-01-02 15:04:05.000"),
				"timestampUnix":  nowWithMs.Unix(),
				"payload":        payload,
			}).Info("📤 Đang gửi notification cho conversationId")

			// Gửi notification qua FolkForm notification system
			result, err := integrations.FolkForm_TriggerNotification("conversation_unreplied", payload)

			// Log response từ API để debug
			jobLogger.WithFields(map[string]interface{}{
				"conversationId": conversationId,
				"pageId":         pageId,
				"processId":      processId,
				"hasError":       err != nil,
				"hasResult":      result != nil,
				"result":         result,
			}).Info("📥 Response từ API trigger notification")

			if err != nil {
				// Lỗi khi gửi → KHÔNG cập nhật rate limiter, để có thể retry lần sau
				jobLogger.WithError(err).WithFields(map[string]interface{}{
					"conversationId": conversationId,
					"pageId":         pageId,
					"processId":      processId,
					"timestamp":      nowWithMs.Format("2006-01-02 15:04:05.000"),
					"timestampUnix":  nowWithMs.Unix(),
					"result":         result,
				}).Error("❌ Lỗi khi gửi notification cho conversationId - KHÔNG cập nhật rate limiter để có thể retry")
				continue
			}

			// Kiểm tra response có thành công không
			// Backend có thể trả về status code 200 nhưng không có status="success"
			success := false
			if result != nil {
				if status, ok := result["status"].(string); ok && status == "success" {
					success = true
				} else if data, ok := result["data"].(map[string]interface{}); ok {
					// Nếu có data, coi như thành công
					success = true
					if queued, ok := data["queued"].(float64); ok {
						jobLogger.WithFields(map[string]interface{}{
							"conversationId": conversationId,
							"pageId":         pageId,
							"queued":         int(queued),
						}).Info("✅ Đã gửi notification thành công - Backend đã tạo %d queue items", int(queued))
					} else {
						jobLogger.WithFields(map[string]interface{}{
							"conversationId": conversationId,
							"pageId":         pageId,
						}).Info("✅ Đã gửi notification thành công")
					}
				} else {
					// Không có status="success" và không có data → có thể là lỗi
					jobLogger.WithFields(map[string]interface{}{
						"conversationId": conversationId,
						"pageId":         pageId,
						"result":         result,
					}).Warn("⚠️ Response không có status='success' hoặc data, kiểm tra lại")
				}
			} else {
				// Không có result → có thể là lỗi
				jobLogger.WithFields(map[string]interface{}{
					"conversationId": conversationId,
					"pageId":         pageId,
				}).Warn("⚠️ Response rỗng, kiểm tra lại")
			}

			if !success {
				// Không thành công → KHÔNG cập nhật rate limiter
				jobLogger.WithFields(map[string]interface{}{
					"conversationId": conversationId,
					"pageId":         pageId,
					"result":         result,
				}).Error("❌ Notification không thành công - KHÔNG cập nhật rate limiter để có thể retry")
				continue
			}

			// THÀNH CÔNG → Cập nhật rate limiter SAU KHI gửi thành công
			global.NotificationRateLimiterMu.Lock()
			global.NotificationRateLimiter[conversationId] = nowWithMs
			rateLimiterSizeAfter := len(global.NotificationRateLimiter)
			global.NotificationRateLimiterMu.Unlock()

			jobLogger.WithFields(map[string]interface{}{
				"conversationId":       conversationId,
				"pageId":               pageId,
				"processId":            processId,
				"newLastSentTime":      nowWithMs.Format("2006-01-02 15:04:05.000"),
				"rateLimiterSizeAfter": rateLimiterSizeAfter,
			}).Info("🔒 Đã cập nhật rate limiter SAU KHI gửi thành công")

			warnedCount++
		}

		// Kiểm tra điều kiện dừng
		if len(items) < limit {
			break
		}

		page++
	}

	jobLogger.WithFields(map[string]interface{}{
		"pageId":      pageId,
		"warnedCount": warnedCount,
	}).Info("✅ Hoàn thành kiểm tra conversations cho page")

	return warnedCount, nil
}

// DoTestNotification gửi một notification test để kiểm tra hệ thống
// Hàm này có thể được gọi từ main.go hoặc test để kiểm tra notification system
func DoTestNotification() error {
	// Lấy logger riêng cho job này
	// File log sẽ là: logs/sync-warn-unreplied-conversations-job.log
	jobLogger := GetJobLoggerByName("sync-warn-unreplied-conversations-job")

	jobLogger.Info("🧪 Bắt đầu test gửi notification...")

	// Kiểm tra token - nếu chưa có thì bỏ qua, đợi CheckInJob login
	if !EnsureApiToken() {
		jobLogger.Debug("Chưa có token, bỏ qua job này. Đợi CheckInJob login...")
		return nil
	}

	// Đảm bảo notification setup đã được tạo
	eventType := "conversation_unreplied"
	err := integrations.FolkForm_EnsureNotificationSetup(eventType, []string{})
	if err != nil {
		jobLogger.WithError(err).Warn("Lưu ý: Có thể notification setup đã tồn tại hoặc có lỗi khi tạo")
		// Không return error, tiếp tục test
	}

	// Tạo payload test
	payload := map[string]interface{}{
		"eventType":        "conversation_unreplied",
		"conversationId":   "test_conversation_123",
		"pageId":           "test_page_123",
		"pageUsername":     "test_page_username",
		"customerName":     "Khách hàng Test",
		"conversationType": "INBOX",
		"minutes":          15,
		"updatedAt":        time.Now().Format("2006-01-02 15:04:05"),
		"conversationLink": "https://pancake.vn/test_page_username?c_id=test_conversation_123",
		"tags":             "test, notification",
	}

	jobLogger.WithFields(map[string]interface{}{
		"eventType":      eventType,
		"conversationId": payload["conversationId"],
		"pageId":         payload["pageId"],
	}).Info("🧪 Đang gửi notification test...")

	// Gửi notification test
	jobLogger.WithFields(map[string]interface{}{
		"payload": payload,
	}).Info("🧪 Payload sẽ được gửi:")

	result, err := integrations.FolkForm_TriggerNotification(eventType, payload)
	if err != nil {
		jobLogger.WithError(err).Error("❌ Lỗi khi gửi notification test")
		jobLogger.Error("⚠️ Kiểm tra logs từ [FolkForm] trong console hoặc app.log để xem chi tiết lỗi")
		return err
	}

	// Log response chi tiết
	if result != nil {
		jobLogger.WithFields(map[string]interface{}{
			"result": result,
		}).Info("📥 Response từ backend:")
	}

	// Log kết quả
	if result != nil {
		if data, ok := result["data"].(map[string]interface{}); ok {
			if message, ok := data["message"].(string); ok {
				jobLogger.WithField("message", message).Info("✅ Notification test đã được gửi thành công")
			}
			if queued, ok := data["queued"].(float64); ok {
				jobLogger.WithField("queued", int(queued)).Info("📊 Số lượng notification đã được thêm vào queue")
			}
		}
		jobLogger.Info("✅ Notification test hoàn thành - Kiểm tra backend để xem notification đã được gửi chưa")
	} else {
		jobLogger.Warn("⚠️ Không nhận được response từ backend")
	}

	return nil
}

// parseResponseDataHelper parse response từ FolkForm API
// Hỗ trợ cả pagination object và array trực tiếp
// Helper function riêng để tránh conflict với parseResponseData trong các file khác
func parseResponseDataHelper(result map[string]interface{}) (items []interface{}, itemCount float64, err error) {
	if result == nil {
		return nil, 0, errors.New("Response là nil")
	}

	// Kiểm tra xem có key "data" không
	dataValue, hasData := result["data"]
	if !hasData {
		// Không có key "data", có thể response có cấu trúc khác
		// Thử kiểm tra xem có phải là array trực tiếp không
		if itemsArray, ok := result["items"].([]interface{}); ok {
			items = itemsArray
			itemCount = float64(len(items))
			return items, itemCount, nil
		}
		// Log để debug
		return nil, 0, errors.New("Không tìm thấy key 'data' trong response")
	}

	// Kiểm tra xem data có phải là pagination object không
	if dataMap, ok := dataValue.(map[string]interface{}); ok {
		// Kiểm tra itemCount
		if count, ok := dataMap["itemCount"].(float64); ok {
			itemCount = count
			// Kiểm tra items
			itemsValue, hasItems := dataMap["items"]
			if hasItems {
				// items có thể là nil, array rỗng, hoặc array có phần tử
				if itemsValue == nil {
					// items là nil → trả về array rỗng
					items = []interface{}{}
					return items, itemCount, nil
				}
				if itemsArray, ok := itemsValue.([]interface{}); ok {
					items = itemsArray
					return items, itemCount, nil
				}
				// items không phải là array hoặc nil
				return nil, itemCount, errors.New("items không phải là array hoặc nil")
			}
			// Không có key "items" → trả về array rỗng nếu itemCount = 0
			if itemCount == 0 {
				items = []interface{}{}
				return items, itemCount, nil
			}
			// Có itemCount > 0 nhưng không có items
			return nil, itemCount, errors.New("Có itemCount > 0 nhưng không tìm thấy items trong response")
		}
		// Không có itemCount, thử lấy items trực tiếp
		itemsValue, hasItems := dataMap["items"]
		if hasItems {
			if itemsValue == nil {
				// items là nil → trả về array rỗng
				items = []interface{}{}
				itemCount = 0
				return items, itemCount, nil
			}
			if itemsArray, ok := itemsValue.([]interface{}); ok {
				items = itemsArray
				itemCount = float64(len(items))
				return items, itemCount, nil
			}
		}
		// Không có cả itemCount và items
		return nil, 0, errors.New("Không tìm thấy itemCount hoặc items trong data object")
	}

	// Kiểm tra xem data có phải là array trực tiếp không
	if dataArray, ok := dataValue.([]interface{}); ok {
		items = dataArray
		itemCount = float64(len(items))
		return items, itemCount, nil
	}

	// Không match với bất kỳ format nào
	return nil, 0, errors.New("Không thể parse response data - cấu trúc không hợp lệ")
}

// getMapKeys helper function để lấy keys của map
func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// cleanupRateLimiter xóa các entry trong rate limiter cũ hơn notificationRateLimitMinutes phút (không phải reset toàn bộ)
// Hàm này được gọi mỗi lần job chạy để dọn dẹp các entry đã hết hạn
func cleanupRateLimiter(notificationRateLimitMinutes int, jobLogger *logrus.Logger) {
	now := time.Now()
	cutoffTime := now.Add(-time.Duration(notificationRateLimitMinutes) * time.Minute) // Xóa các entry cũ hơn notificationRateLimitMinutes phút
	processId := os.Getpid()

	global.NotificationRateLimiterMu.Lock()
	defer global.NotificationRateLimiterMu.Unlock()

	// Đếm số lượng entry trước khi cleanup
	beforeCount := len(global.NotificationRateLimiter)

	// Lưu danh sách conversationId bị xóa để log
	var cleanedConversationIds []string

	// Xóa các entry cũ hơn 5 phút
	for conversationId, lastSentTime := range global.NotificationRateLimiter {
		if lastSentTime.Before(cutoffTime) {
			delete(global.NotificationRateLimiter, conversationId)
			cleanedConversationIds = append(cleanedConversationIds, conversationId)
		}
	}

	// Đếm số lượng entry sau khi cleanup
	afterCount := len(global.NotificationRateLimiter)
	cleanedCount := beforeCount - afterCount

	if cleanedCount > 0 {
		jobLogger.WithFields(map[string]interface{}{
			"processId":              processId,
			"beforeCount":            beforeCount,
			"afterCount":             afterCount,
			"cleanedCount":           cleanedCount,
			"cleanedConversationIds": cleanedConversationIds,
			"cutoffTime":             cutoffTime.Format("2006-01-02 15:04:05.000"),
			"rateLimitMinutes":       notificationRateLimitMinutes,
		}).Info("🧹 Đã cleanup rate limiter - Xóa các conversationId cũ hơn 5 phút")
	} else {
		jobLogger.WithFields(map[string]interface{}{
			"processId":   processId,
			"beforeCount": beforeCount,
			"afterCount":  afterCount,
			"cutoffTime":  cutoffTime.Format("2006-01-02 15:04:05.000"),
		}).Debug("🧹 Cleanup rate limiter - Không có entry nào cần xóa")
	}
}
