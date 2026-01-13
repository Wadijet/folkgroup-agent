/*
Package jobs chứa các job cụ thể của ứng dụng.
File này chứa SyncPriorityConversationsJob - job đồng bộ các conversations có flag needsPrioritySync=true.
Job này chạy mỗi 1 phút để đảm bảo các conversations được đánh dấu ưu tiên được sync ngay lập tức.
*/
package jobs

import (
	"agent_pancake/app/integrations"
	"agent_pancake/app/scheduler"
	"context"
	"time"

	apputility "agent_pancake/app/utility"
)

// SyncPriorityConversationsJob là job đồng bộ các conversations có flag needsPrioritySync=true.
// Job này sẽ:
// - Lấy danh sách conversations có needsPrioritySync=true từ FolkForm
// - Sync từng conversation từ Pancake về FolkForm
// - Sau khi sync xong, set needsPrioritySync=false
type SyncPriorityConversationsJob struct {
	*scheduler.BaseJob
}

// NewSyncPriorityConversationsJob tạo một instance mới của SyncPriorityConversationsJob.
// Tham số:
// - name: Tên định danh của job
// - schedule: Biểu thức cron định nghĩa lịch chạy
// Trả về một instance của SyncPriorityConversationsJob
func NewSyncPriorityConversationsJob(name, schedule string) *SyncPriorityConversationsJob {
	job := &SyncPriorityConversationsJob{
		BaseJob: scheduler.NewBaseJob(name, schedule),
	}
	// Set callback function để BaseJob.Execute có thể gọi ExecuteInternal đúng cách
	job.BaseJob.SetExecuteInternalCallback(job.ExecuteInternal)
	return job
}

// ExecuteInternal thực thi logic đồng bộ conversations có flag needsPrioritySync.
// Phương thức này gọi DoSyncPriorityConversations() và thêm log wrapper cho job.
// Tham số:
// - ctx: Context để kiểm soát thời gian thực thi
// Trả về error nếu có lỗi xảy ra
func (j *SyncPriorityConversationsJob) ExecuteInternal(ctx context.Context) error {
	startTime := time.Now()
	LogJobStart(j.GetName(), j.GetSchedule()).WithFields(map[string]interface{}{
		"start_time": startTime.Format("2006-01-02 15:04:05"),
	}).Info("🚀 JOB ĐÃ BẮT ĐẦU CHẠY")

	// Gọi hàm logic thực sự
	err := DoSyncPriorityConversations()
	duration := time.Since(startTime)
	durationMs := duration.Milliseconds()

	if err != nil {
		LogJobError(j.GetName(), err, duration.String(), durationMs)
		return err
	}

	LogJobEnd(j.GetName(), duration.String(), durationMs)
	return nil
}

// DoSyncPriorityConversations thực thi logic đồng bộ conversations có flag needsPrioritySync.
// Hàm này:
// - Lấy danh sách conversations có needsPrioritySync=true từ FolkForm
// - Sync từng conversation từ Pancake về FolkForm
// - Sau khi sync xong, set needsPrioritySync=false
// Hàm này có thể được gọi độc lập mà không cần thông qua job interface.
// Trả về error nếu có lỗi xảy ra
func DoSyncPriorityConversations() error {
	// Lấy logger riêng cho job này
	// File log sẽ là: logs/sync-priority-conversations-job.log
	jobLogger := GetJobLoggerByName("sync-priority-conversations-job")

	// Kiểm tra token - nếu chưa có thì bỏ qua, đợi CheckInJob login
	if !EnsureApiToken() {
		jobLogger.Debug("Chưa có token, bỏ qua job này. Đợi CheckInJob login...")
		return nil
	}

	// Lấy pageSize từ config động (có thể thay đổi từ server)
	// Nếu không có config, sử dụng default value 50
	pageSize := GetJobConfigInt("sync-priority-conversations-job", "pageSize", 50)
	jobLogger.WithField("pageSize", pageSize).Info("📋 Sử dụng pageSize từ config")

	// Lấy conversations có needsPrioritySync=true từ FolkForm với pagination
	page := 1
	limit := pageSize
	totalSynced := 0
	rateLimiter := apputility.GetFolkFormRateLimiter()

	for {
		// Áp dụng Rate Limiter
		rateLimiter.Wait()

		// Lấy conversations có needsPrioritySync=true từ FolkForm
		result, err := integrations.FolkForm_GetPrioritySyncConversations(page, limit)
		if err != nil {
			jobLogger.WithError(err).Error("Lỗi khi lấy conversations cần ưu tiên sync từ FolkForm")
			return err
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
			jobLogger.Info("Không còn conversations nào cần ưu tiên sync")
			break
		}

		jobLogger.WithFields(map[string]interface{}{
			"page":  page,
			"count": len(items),
		}).Info("Lấy được conversations cần ưu tiên sync từ FolkForm")

		// Sync từng conversation
		for _, item := range items {
			itemMap, ok := item.(map[string]interface{})
			if !ok {
				continue
			}

			// Lấy thông tin conversation
			conversationId, _ := itemMap["conversationId"].(string)
			if conversationId == "" {
				// Thử field "id" nếu không có "conversationId"
				if id, ok := itemMap["id"].(string); ok && id != "" {
					conversationId = id
				} else {
					jobLogger.Warn("Conversation không có conversationId, bỏ qua")
					continue
				}
			}

			pageId, _ := itemMap["pageId"].(string)
			if pageId == "" {
				jobLogger.WithField("conversationId", conversationId).Warn("Conversation không có pageId, bỏ qua")
				continue
			}

			pageUsername, _ := itemMap["pageUsername"].(string)
			if pageUsername == "" {
				// Fallback: dùng pageId nếu không có username
				pageUsername = pageId
			}

			jobLogger.WithFields(map[string]interface{}{
				"conversationId": conversationId,
				"pageId":         pageId,
				"pageUsername":   pageUsername,
			}).Info("🔄 Bắt đầu sync conversation ưu tiên")

			// Lấy conversation từ Pancake
			rateLimiterPancake := apputility.GetPancakeRateLimiter()
			rateLimiterPancake.Wait()

			// Lấy conversation từ Pancake bằng conversationId
			// Sử dụng Pancake_GetConversationById nếu có, hoặc dùng Pancake_GetConversations_v2 với filter
			// Tạm thời dùng cách lấy từ page và tìm conversationId trong danh sách
			conversationData, err := getConversationFromPancake(pageId, conversationId)
			if err != nil {
				jobLogger.WithError(err).WithFields(map[string]interface{}{
					"conversationId": conversationId,
					"pageId":         pageId,
				}).Error("❌ Lỗi khi lấy conversation từ Pancake")
				// Tiếp tục với conversation tiếp theo, không dừng
				continue
			}

			if conversationData == nil {
				jobLogger.WithFields(map[string]interface{}{
					"conversationId": conversationId,
					"pageId":         pageId,
				}).Warn("⚠️ Không tìm thấy conversation trong Pancake, có thể đã bị xóa")
				// Vẫn set needsPrioritySync=false để không sync lại nữa
				_, _ = integrations.FolkForm_UpdateConversationNeedsPrioritySync(conversationId, false)
				continue
			}

			// Sync conversation từ Pancake về FolkForm
			_, err = integrations.FolkForm_CreateConversation(pageId, pageUsername, conversationData)
			if err != nil {
				jobLogger.WithError(err).WithFields(map[string]interface{}{
					"conversationId": conversationId,
					"pageId":         pageId,
				}).Error("❌ Lỗi khi sync conversation về FolkForm")
				// Tiếp tục với conversation tiếp theo, không dừng
				continue
			}

			// Sync messages của conversation
			// Sử dụng function từ bridge_v2.go thông qua BridgeV2_SyncNewData hoặc tạo helper
			// Tạm thời bỏ qua sync messages vì đã có job sync messages riêng
			// Conversation đã được sync, messages sẽ được sync bởi job sync messages
			jobLogger.WithFields(map[string]interface{}{
				"conversationId": conversationId,
				"pageId":         pageId,
			}).Info("💡 Conversation đã được sync, messages sẽ được sync bởi job sync messages")

			// Sau khi sync xong, set needsPrioritySync=false
			rateLimiter.Wait()
			_, err = integrations.FolkForm_UpdateConversationNeedsPrioritySync(conversationId, false)
			if err != nil {
				jobLogger.WithError(err).WithFields(map[string]interface{}{
					"conversationId": conversationId,
					"pageId":         pageId,
				}).Warn("⚠️ Lỗi khi cập nhật flag needsPrioritySync, conversation đã được sync")
				// Tiếp tục, không dừng
			} else {
				jobLogger.WithFields(map[string]interface{}{
					"conversationId": conversationId,
					"pageId":         pageId,
				}).Info("✅ Đã sync và cập nhật flag needsPrioritySync=false")
			}

			totalSynced++
		}

		// Kiểm tra điều kiện dừng
		if len(items) < limit {
			break
		}

		page++
	}

	jobLogger.WithField("total_synced", totalSynced).Info("✅ Hoàn thành sync conversations ưu tiên")
	return nil
}

// getConversationFromPancake lấy conversation từ Pancake bằng conversationId
// Hàm này tìm conversation trong danh sách conversations của page
// Tìm trong tối đa 10 batches để đảm bảo tìm thấy conversation
func getConversationFromPancake(pageId string, conversationId string) (interface{}, error) {
	rateLimiter := apputility.GetPancakeRateLimiter()
	lastConversationId := ""
	maxBatches := 10 // Tìm trong tối đa 10 batches

	for batch := 0; batch < maxBatches; batch++ {
		rateLimiter.Wait()

		// Lấy conversations từ Pancake
		result, err := integrations.Pancake_GetConversations_v2(pageId, lastConversationId, 0, 0, "", false)
		if err != nil {
			return nil, err
		}

		// Parse conversations từ response
		var conversations []interface{}
		if convs, ok := result["conversations"].([]interface{}); ok {
			conversations = convs
		}

		if len(conversations) == 0 {
			// Không còn conversations nào
			break
		}

		// Tìm conversation có id = conversationId trong batch này
		for _, conv := range conversations {
			convMap, ok := conv.(map[string]interface{})
			if !ok {
				continue
			}

			if id, ok := convMap["id"].(string); ok && id == conversationId {
				return conv, nil
			}
		}

		// Cập nhật lastConversationId để lấy batch tiếp theo
		lastConv := conversations[len(conversations)-1]
		if lastConvMap, ok := lastConv.(map[string]interface{}); ok {
			if lastId, ok := lastConvMap["id"].(string); ok {
				lastConversationId = lastId
			}
		}
	}

	// Không tìm thấy sau khi tìm trong tất cả batches
	return nil, nil
}
