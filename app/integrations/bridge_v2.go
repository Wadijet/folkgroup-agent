package integrations

import (
	"errors"
	"log"
	"time"

	apputility "agent_pancake/app/utility"
)

// BridgeV2_SyncNewData sync conversations mới (incremental sync)
// Sử dụng order_by=updated_at và dừng khi gặp lastConversationId từ FolkForm
func BridgeV2_SyncNewData() error {
	log.Println("[BridgeV2] Bắt đầu sync conversations mới (incremental sync)")

	// Lấy tất cả pages từ FolkForm
	limit := 50
	page := 1

	for {
		// Lấy danh sách các pages từ server FolkForm
		resultPages, err := FolkForm_GetFbPages(page, limit)
		if err != nil {
			logError("[BridgeV2] Lỗi khi lấy danh sách trang Facebook: %v", err)
			return errors.New("Lỗi khi lấy danh sách trang Facebook")
		}

		// Xử lý response - có thể là pagination object hoặc array trực tiếp
		items, itemCount, err := parseResponseData(resultPages)
		if err != nil {
			logError("[BridgeV2] LỖI khi parse response: %v", err)
			return err
		}

		if itemCount == 0 || len(items) == 0 {
			log.Printf("[BridgeV2] Không còn pages nào, dừng sync")
			break
		}

		log.Printf("[BridgeV2] Nhận được %d pages (page=%d, limit=%d)", len(items), page, limit)

		// Với mỗi page
		for _, item := range items {
			pageMap, ok := item.(map[string]interface{})
			if !ok {
				logError("[BridgeV2] Page không phải là map, bỏ qua")
				continue
			}

			pageId, ok := pageMap["pageId"].(string)
			if !ok || pageId == "" {
				logError("[BridgeV2] Page không có pageId, bỏ qua")
				continue
			}

			pageUsername, _ := pageMap["pageUsername"].(string)
			isSync, _ := pageMap["isSync"].(bool)

			if !isSync {
				log.Printf("[BridgeV2] Page %s không sync (isSync=false), bỏ qua", pageId)
				continue
			}

			// Lấy conversation mới nhất từ FolkForm
			lastConversationId, err := FolkForm_GetLastConversationId(pageId)
			if err != nil {
				logError("[BridgeV2] Lỗi khi lấy lastConversationId cho page %s: %v", pageId, err)
				continue
			}

			log.Printf("[BridgeV2] Page %s - lastConversationId: %s", pageId, lastConversationId)

			// Sync conversations mới hơn lastConversationId
			// Pancake API: last_conversation_id trả về conversations cũ hơn, nên để lấy conversations mới hơn:
			// - Bắt đầu từ đầu (last_conversation_id = "") để lấy conversations mới nhất
			// - Dừng khi gặp lastConversationId (đã sync hết conversations mới hơn)
			last_conversation_id := ""

			// Sử dụng adaptive rate limiter để tránh rate limit
			rateLimiter := apputility.GetPancakeRateLimiter()

			for {
				// Áp dụng Rate Limiter: Gọi Wait() trước mỗi API call
				rateLimiter.Wait()

				// Gọi Pancake API (đã có retry logic sẵn trong Pancake_GetConversations_v2)
				// Sử dụng unread_first=true để ưu tiên lấy conversations chưa đọc trước
				resultGetConversations, err := Pancake_GetConversations_v2(pageId, last_conversation_id, 0, 0, "updated_at", true)
				if err != nil {
					logError("[BridgeV2] Lỗi khi lấy danh sách hội thoại: %v", err)
					break
				}

				// Parse conversations từ response
				var conversations []interface{}
				if convs, ok := resultGetConversations["conversations"].([]interface{}); ok {
					conversations = convs
				}

				if len(conversations) == 0 {
					log.Printf("[BridgeV2] Không còn conversations nào cho page %s", pageId)
					break
				}

				log.Printf("[BridgeV2] Page %s - Lấy được %d conversations", pageId, len(conversations))

				foundLastConversation := false

				// Sync từng conversation
				for _, conv := range conversations {
					convMap, ok := conv.(map[string]interface{})
					if !ok {
						logError("[BridgeV2] Conversation không phải là map, bỏ qua")
						continue
					}

					convId, ok := convMap["id"].(string)
					if !ok || convId == "" {
						logError("[BridgeV2] Conversation không có id, bỏ qua")
						continue
					}

					customerId := ""
					if cid, ok := convMap["customer_id"].(string); ok {
						customerId = cid
					}

					// Kiểm tra: Đã gặp conversation cuối cùng chưa?
					// Chỉ check nếu đã có conversation trong FolkForm (lastConversationId != "")
					if lastConversationId != "" && convId == lastConversationId {
						foundLastConversation = true
						log.Printf("[BridgeV2] Đã gặp folkform_last_conversation_id (%s), dừng sync cho page %s", lastConversationId, pageId)
						break
					}

					// Sync conversation
					_, err = FolkForm_CreateConversation(pageId, pageUsername, conv)
					if err != nil {
						logError("[BridgeV2] Lỗi khi tạo/cập nhật conversation %s: %v", convId, err)
						continue
					}

					// Sync messages mới
					// Lưu ý: bridge_SyncMessageOfConversation đã có rate limiter bên trong
					err = bridge_SyncMessageOfConversation(pageId, pageUsername, convId, customerId)
					if err != nil {
						logError("[BridgeV2] Lỗi khi sync messages cho conversation %s: %v", convId, err)
						// Tiếp tục với conversation tiếp theo, không dừng
					}
				}

				if foundLastConversation {
					break // Dừng pagination cho page này
				}

				// Cập nhật last_conversation_id để pagination
				if len(conversations) > 0 {
					lastConv := conversations[len(conversations)-1].(map[string]interface{})
					if newLastId, ok := lastConv["id"].(string); ok {
						last_conversation_id = newLastId
					} else {
						logError("[BridgeV2] Không thể lấy id từ conversation cuối cùng, dừng pagination")
						break
					}
				} else {
					break
				}
			}
		}

		page++
	}

	log.Println("[BridgeV2] ✅ Hoàn thành sync conversations mới")
	return nil
}

// BridgeV2_SyncAllData sync tất cả conversations cũ (full sync)
// Sử dụng order_by=updated_at và bắt đầu từ oldestConversationId từ FolkForm
func BridgeV2_SyncAllData() error {
	log.Println("[BridgeV2] Bắt đầu sync tất cả conversations (full sync)")

	// Lấy tất cả pages từ FolkForm
	limit := 50
	page := 1

	for {
		// Lấy danh sách các pages từ server FolkForm
		resultPages, err := FolkForm_GetFbPages(page, limit)
		if err != nil {
			logError("[BridgeV2] Lỗi khi lấy danh sách trang Facebook: %v", err)
			return errors.New("Lỗi khi lấy danh sách trang Facebook")
		}

		// Xử lý response - có thể là pagination object hoặc array trực tiếp
		items, itemCount, err := parseResponseData(resultPages)
		if err != nil {
			logError("[BridgeV2] LỖI khi parse response: %v", err)
			return err
		}

		if itemCount == 0 || len(items) == 0 {
			log.Printf("[BridgeV2] Không còn pages nào, dừng sync")
			break
		}

		log.Printf("[BridgeV2] Nhận được %d pages (page=%d, limit=%d)", len(items), page, limit)

		// Với mỗi page
		for _, item := range items {
			pageMap, ok := item.(map[string]interface{})
			if !ok {
				logError("[BridgeV2] Page không phải là map, bỏ qua")
				continue
			}

			pageId, ok := pageMap["pageId"].(string)
			if !ok || pageId == "" {
				logError("[BridgeV2] Page không có pageId, bỏ qua")
				continue
			}

			pageUsername, _ := pageMap["pageUsername"].(string)
			isSync, _ := pageMap["isSync"].(bool)

			if !isSync {
				log.Printf("[BridgeV2] Page %s không sync (isSync=false), bỏ qua", pageId)
				continue
			}

			// Lấy conversation cũ nhất từ FolkForm
			oldestConversationId, err := FolkForm_GetOldestConversationId(pageId)
			if err != nil {
				logError("[BridgeV2] Lỗi khi lấy oldestConversationId cho page %s: %v", pageId, err)
				continue
			}

			log.Printf("[BridgeV2] Page %s - oldestConversationId: %s", pageId, oldestConversationId)

			// Sync conversations cũ hơn oldestConversationId
			// Nếu có oldestConversationId, bắt đầu từ đó để lấy conversations cũ hơn
			// Nếu không có oldestConversationId, bắt đầu từ đầu (last_conversation_id = "") để lấy conversations mới nhất, rồi paginate về cũ hơn
			last_conversation_id := oldestConversationId

			// Sử dụng adaptive rate limiter để tránh rate limit
			rateLimiter := apputility.GetPancakeRateLimiter()

			// Đếm số batches để lấy lại oldestConversationId sau mỗi N batches
			batchCount := 0
			conversationCount := 0
			const REFRESH_OLDEST_AFTER_BATCHES = 10 // Lấy lại oldestConversationId sau mỗi 10 batches

			for {
				// Áp dụng Rate Limiter: Gọi Wait() trước mỗi API call
				rateLimiter.Wait()

				// Lấy lại oldestConversationId sau mỗi N batches để cập nhật mốc
				if batchCount > 0 && batchCount%REFRESH_OLDEST_AFTER_BATCHES == 0 {
					newOldestConversationId, err := FolkForm_GetOldestConversationId(pageId)
					if err != nil {
						logError("[BridgeV2] Lỗi khi lấy lại oldestConversationId cho page %s: %v", pageId, err)
						// Tiếp tục với oldestConversationId cũ
					} else if newOldestConversationId != "" && newOldestConversationId != oldestConversationId {
						log.Printf("[BridgeV2] Page %s - Cập nhật oldestConversationId: %s -> %s (đã sync %d conversations)", pageId, oldestConversationId, newOldestConversationId, conversationCount)
						oldestConversationId = newOldestConversationId
						// Cập nhật last_conversation_id để tiếp tục sync từ conversation cũ nhất hiện tại
						last_conversation_id = oldestConversationId
					}
				}

				batchCount++

				// Gọi Pancake API (đã có retry logic sẵn trong Pancake_GetConversations_v2)
				// Full sync: Không dùng unread_first (chỉ dùng cho real-time sync)
				// Dùng order_by=updated_at để sync từ cũ → mới
				resultGetConversations, err := Pancake_GetConversations_v2(pageId, last_conversation_id, 0, 0, "updated_at", false)
				if err != nil {
					logError("[BridgeV2] Lỗi khi lấy danh sách hội thoại: %v", err)
					break
				}

				// Parse conversations từ response
				var conversations []interface{}
				if convs, ok := resultGetConversations["conversations"].([]interface{}); ok {
					conversations = convs
				}

				if len(conversations) == 0 {
					log.Printf("[BridgeV2] Không còn conversations cũ hơn cho page %s, dừng sync", pageId)
					break
				}

				log.Printf("[BridgeV2] Page %s - Lấy được %d conversations cũ hơn (batch %d, tổng %d conversations)", pageId, len(conversations), batchCount, conversationCount)

				// Sync từng conversation
				for _, conv := range conversations {
					conversationCount++
					convMap, ok := conv.(map[string]interface{})
					if !ok {
						logError("[BridgeV2] Conversation không phải là map, bỏ qua")
						continue
					}

					convId, ok := convMap["id"].(string)
					if !ok || convId == "" {
						logError("[BridgeV2] Conversation không có id, bỏ qua")
						continue
					}

					customerId := ""
					if cid, ok := convMap["customer_id"].(string); ok {
						customerId = cid
					}

					// Sync conversation
					_, err = FolkForm_CreateConversation(pageId, pageUsername, conv)
					if err != nil {
						logError("[BridgeV2] Lỗi khi tạo/cập nhật conversation %s: %v", convId, err)
						continue
					}

					// Sync TẤT CẢ messages
					// Lưu ý: bridge_SyncMessageOfConversation đã có rate limiter bên trong
					// Và đã có logic để sync tất cả messages (không chỉ mới)
					err = bridge_SyncMessageOfConversation(pageId, pageUsername, convId, customerId)
					if err != nil {
						logError("[BridgeV2] Lỗi khi sync messages cho conversation %s: %v", convId, err)
						// Tiếp tục với conversation tiếp theo, không dừng
					}
				}

				// Cập nhật last_conversation_id để pagination
				if len(conversations) > 0 {
					lastConv := conversations[len(conversations)-1].(map[string]interface{})
					if newLastId, ok := lastConv["id"].(string); ok {
						last_conversation_id = newLastId
					} else {
						logError("[BridgeV2] Không thể lấy id từ conversation cuối cùng, dừng pagination")
						break
					}
				} else {
					break
				}
			}
		}

		page++
	}

	log.Println("[BridgeV2] ✅ Hoàn thành sync tất cả conversations")
	return nil
}

// Helper function: Parse inserted_at từ Pancake (ISO 8601 string) sang Unix timestamp (seconds)
// Format: "2022-08-22T03:09:27" hoặc "2022-08-22T03:09:27.000000"
func parsePostInsertedAt(insertedAtStr string) (int64, error) {
	// Thử parse với format có microseconds
	layouts := []string{
		"2006-01-02T15:04:05.000000",
		"2006-01-02T15:04:05",
		time.RFC3339,
		time.RFC3339Nano,
	}

	for _, layout := range layouts {
		t, err := time.Parse(layout, insertedAtStr)
		if err == nil {
			return t.Unix(), nil
		}
	}

	return 0, errors.New("Không thể parse inserted_at: " + insertedAtStr)
}

// BridgeV2_SyncNewPosts sync posts mới (incremental sync) cho tất cả pages
func BridgeV2_SyncNewPosts() error {
	log.Println("[BridgeV2] Bắt đầu sync posts mới (incremental sync)")

	// Lấy tất cả pages từ FolkForm
	limit := 50
	page := 1

	for {
		// Lấy danh sách các pages từ server FolkForm
		resultPages, err := FolkForm_GetFbPages(page, limit)
		if err != nil {
			logError("[BridgeV2] Lỗi khi lấy danh sách trang Facebook: %v", err)
			return errors.New("Lỗi khi lấy danh sách trang Facebook")
		}

		// Xử lý response
		items, itemCount, err := parseResponseData(resultPages)
		if err != nil {
			logError("[BridgeV2] LỖI khi parse response: %v", err)
			return err
		}

		if itemCount == 0 || len(items) == 0 {
			log.Printf("[BridgeV2] Không còn pages nào, dừng sync")
			break
		}

		log.Printf("[BridgeV2] Nhận được %d pages (page=%d, limit=%d)", len(items), page, limit)

		// Với mỗi page
		for _, item := range items {
			pageMap, ok := item.(map[string]interface{})
			if !ok {
				logError("[BridgeV2] Page không phải là map, bỏ qua")
				continue
			}

			pageId, ok := pageMap["pageId"].(string)
			if !ok || pageId == "" {
				logError("[BridgeV2] Page không có pageId, bỏ qua")
				continue
			}

			pageUsername, _ := pageMap["pageUsername"].(string)
			isSync, _ := pageMap["isSync"].(bool)

			if !isSync {
				log.Printf("[BridgeV2] Page %s không sync (isSync=false), bỏ qua", pageId)
				continue
			}

			// Sync posts mới cho page này
			err = bridgeV2_SyncNewPostsOfPage(pageId, pageUsername)
			if err != nil {
				logError("[BridgeV2] Lỗi khi sync posts mới cho page %s: %v", pageId, err)
				// Tiếp tục với page tiếp theo
				continue
			}
		}

		page++
	}

	log.Println("[BridgeV2] ✅ Hoàn thành sync posts mới")
	return nil
}

// bridgeV2_SyncNewPostsOfPage sync posts mới (incremental sync) cho một page
func bridgeV2_SyncNewPostsOfPage(pageId string, pageUsername string) error {
	log.Printf("[BridgeV2] Bắt đầu sync posts mới cho page %s", pageId)

	// 1. Lấy mốc từ FolkForm
	_, lastInsertedAtMs, err := FolkForm_GetLastPostId(pageId)
	if err != nil {
		logError("[BridgeV2] Lỗi khi lấy lastPostId cho page %s: %v", pageId, err)
		return err
	}

	// 2. Convert milliseconds → seconds
	var since, until int64
	if lastInsertedAtMs == 0 {
		// Chưa có posts → giới hạn 30 ngày
		until = time.Now().Unix()
		since = until - (30 * 24 * 60 * 60) // 30 ngày trước
		log.Printf("[BridgeV2] Page %s - Chưa có posts, sync 30 ngày gần nhất", pageId)
	} else {
		since = lastInsertedAtMs / 1000
		until = time.Now().Unix()
		log.Printf("[BridgeV2] Page %s - Sync posts từ %d đến %d", pageId, since, until)
	}

	// 3. Pagination loop
	pageNumber := 1
	pageSize := 30
	rateLimiter := apputility.GetPancakeRateLimiter()

	for {
		// Rate limiter
		rateLimiter.Wait()

		// Gọi Pancake API
		result, err := Pancake_GetPosts(pageId, pageNumber, pageSize, since, until, "")
		if err != nil {
			logError("[BridgeV2] Lỗi khi lấy posts cho page %s: %v", pageId, err)
			break
		}

		// Parse posts
		posts, ok := result["posts"].([]interface{})
		if !ok || len(posts) == 0 {
			log.Printf("[BridgeV2] Page %s - Không còn posts nào, dừng sync", pageId)
			break
		}

		log.Printf("[BridgeV2] Page %s - Lấy được %d posts (page_number=%d)", pageId, len(posts), pageNumber)

		// 4. Xử lý từng post
		foundOldPost := false
		for _, post := range posts {
			postMap, ok := post.(map[string]interface{})
			if !ok {
				continue
			}

			// Parse inserted_at từ Pancake
			insertedAtStr, ok := postMap["inserted_at"].(string)
			if !ok {
				log.Printf("[BridgeV2] Post không có inserted_at, bỏ qua")
				continue
			}

			// Convert ISO 8601 → Unix timestamp (seconds)
			insertedAtSeconds, err := parsePostInsertedAt(insertedAtStr)
			if err != nil {
				log.Printf("[BridgeV2] Lỗi khi parse inserted_at: %v", err)
				continue
			}

			// ⚠️ LOGIC DỪNG: Nếu post cũ hơn since → đã sync hết
			if insertedAtSeconds < since {
				foundOldPost = true
				log.Printf("[BridgeV2] Page %s - Gặp post cũ hơn since (%d < %d), dừng sync", pageId, insertedAtSeconds, since)
				break // Dừng xử lý batch này
			}

			// ✅ Upsert post (tự động xử lý duplicate theo postId)
			_, err = FolkForm_CreateFbPost(post)
			if err != nil {
				logError("[BridgeV2] Lỗi khi upsert post: %v", err)
				// Tiếp tục với post tiếp theo
			}
		}

		// 5. Kiểm tra điều kiện dừng
		if foundOldPost {
			log.Printf("[BridgeV2] Page %s - Đã sync hết posts mới (gặp post cũ hơn since)", pageId)
			break
		}

		if len(posts) < pageSize {
			log.Printf("[BridgeV2] Page %s - Đã lấy hết posts (len=%d < page_size=%d)", pageId, len(posts), pageSize)
			break
		}

		// Tiếp tục với page tiếp theo
		pageNumber++
	}

	log.Printf("[BridgeV2] ✅ Hoàn thành sync posts mới cho page %s", pageId)
	return nil
}

// BridgeV2_SyncAllPosts sync posts cũ (backfill sync) cho tất cả pages
func BridgeV2_SyncAllPosts() error {
	log.Println("[BridgeV2] Bắt đầu sync posts cũ (backfill sync)")

	// Lấy tất cả pages từ FolkForm
	limit := 50
	page := 1

	for {
		// Lấy danh sách các pages từ server FolkForm
		resultPages, err := FolkForm_GetFbPages(page, limit)
		if err != nil {
			logError("[BridgeV2] Lỗi khi lấy danh sách trang Facebook: %v", err)
			return errors.New("Lỗi khi lấy danh sách trang Facebook")
		}

		// Xử lý response
		items, itemCount, err := parseResponseData(resultPages)
		if err != nil {
			logError("[BridgeV2] LỖI khi parse response: %v", err)
			return err
		}

		if itemCount == 0 || len(items) == 0 {
			log.Printf("[BridgeV2] Không còn pages nào, dừng sync")
			break
		}

		log.Printf("[BridgeV2] Nhận được %d pages (page=%d, limit=%d)", len(items), page, limit)

		// Với mỗi page
		for _, item := range items {
			pageMap, ok := item.(map[string]interface{})
			if !ok {
				logError("[BridgeV2] Page không phải là map, bỏ qua")
				continue
			}

			pageId, ok := pageMap["pageId"].(string)
			if !ok || pageId == "" {
				logError("[BridgeV2] Page không có pageId, bỏ qua")
				continue
			}

			pageUsername, _ := pageMap["pageUsername"].(string)
			isSync, _ := pageMap["isSync"].(bool)

			if !isSync {
				log.Printf("[BridgeV2] Page %s không sync (isSync=false), bỏ qua", pageId)
				continue
			}

			// Sync posts cũ cho page này
			err = bridgeV2_SyncAllPostsOfPage(pageId, pageUsername)
			if err != nil {
				logError("[BridgeV2] Lỗi khi sync posts cũ cho page %s: %v", pageId, err)
				// Tiếp tục với page tiếp theo
				continue
			}
		}

		page++
	}

	log.Println("[BridgeV2] ✅ Hoàn thành sync posts cũ")
	return nil
}

// bridgeV2_SyncAllPostsOfPage sync posts cũ (backfill sync) cho một page
func bridgeV2_SyncAllPostsOfPage(pageId string, pageUsername string) error {
	log.Printf("[BridgeV2] Bắt đầu sync posts cũ cho page %s", pageId)

	// 1. Lấy mốc từ FolkForm
	_, oldestInsertedAtMs, err := FolkForm_GetOldestPostId(pageId)
	if err != nil {
		logError("[BridgeV2] Lỗi khi lấy oldestPostId cho page %s: %v", pageId, err)
		return err
	}

	// 2. Convert milliseconds → seconds
	var since, until int64
	if oldestInsertedAtMs == 0 {
		// Chưa có posts → sync toàn bộ
		since = 0
		until = time.Now().Unix()
		log.Printf("[BridgeV2] Page %s - Chưa có posts, sync toàn bộ", pageId)
	} else {
		since = 0 // Hoặc 1 năm trước: time.Now().Unix() - (365 * 24 * 60 * 60)
		until = oldestInsertedAtMs / 1000
		log.Printf("[BridgeV2] Page %s - Sync posts cũ hơn %d (từ %d đến %d)", pageId, until, since, until)
	}

	// 3. Pagination loop với refresh mốc
	pageNumber := 1
	pageSize := 30
	batchCount := 0
	const REFRESH_OLDEST_AFTER_BATCHES = 10
	rateLimiter := apputility.GetPancakeRateLimiter()

	for {
		// Refresh oldestPostId sau mỗi N batches
		if batchCount > 0 && batchCount%REFRESH_OLDEST_AFTER_BATCHES == 0 {
			_, newOldestMs, _ := FolkForm_GetOldestPostId(pageId)
			newOldestSeconds := newOldestMs / 1000
			if newOldestSeconds > 0 && newOldestSeconds < until {
				// Có post cũ hơn → cập nhật until
				log.Printf("[BridgeV2] Page %s - Cập nhật until: %d -> %d (có post cũ hơn)", pageId, until, newOldestSeconds)
				until = newOldestSeconds
				oldestInsertedAtMs = newOldestMs
			}
		}

		batchCount++

		// Rate limiter
		rateLimiter.Wait()

		// Gọi Pancake API
		result, err := Pancake_GetPosts(pageId, pageNumber, pageSize, since, until, "")
		if err != nil {
			logError("[BridgeV2] Lỗi khi lấy posts cho page %s: %v", pageId, err)
			break
		}

		// Parse posts
		posts, ok := result["posts"].([]interface{})
		if !ok || len(posts) == 0 {
			log.Printf("[BridgeV2] Page %s - Không còn posts nào, dừng sync", pageId)
			break
		}

		log.Printf("[BridgeV2] Page %s - Lấy được %d posts cũ (page_number=%d, batch=%d)", pageId, len(posts), pageNumber, batchCount)

		// 4. Xử lý từng post
		foundNewPost := false
		for _, post := range posts {
			postMap, ok := post.(map[string]interface{})
			if !ok {
				continue
			}

			// Parse inserted_at từ Pancake
			insertedAtStr, ok := postMap["inserted_at"].(string)
			if !ok {
				continue
			}

			insertedAtSeconds, err := parsePostInsertedAt(insertedAtStr)
			if err != nil {
				continue
			}

			// ⚠️ LOGIC DỪNG: Nếu post mới hơn until → vượt quá mốc
			if insertedAtSeconds > until {
				foundNewPost = true
				log.Printf("[BridgeV2] Page %s - Gặp post mới hơn until (%d > %d), dừng sync", pageId, insertedAtSeconds, until)
				break // Dừng xử lý batch này
			}

			// ✅ Upsert post (tự động xử lý duplicate)
			_, err = FolkForm_CreateFbPost(post)
			if err != nil {
				logError("[BridgeV2] Lỗi khi upsert post: %v", err)
			}
		}

		// 5. Kiểm tra điều kiện dừng
		if foundNewPost {
			log.Printf("[BridgeV2] Page %s - Đã sync hết posts cũ (gặp post mới hơn until)", pageId)
			break
		}

		if len(posts) < pageSize {
			log.Printf("[BridgeV2] Page %s - Đã lấy hết posts (len=%d < page_size=%d)", pageId, len(posts), pageSize)
			break
		}

		// Tiếp tục với page tiếp theo
		pageNumber++
	}

	log.Printf("[BridgeV2] ✅ Hoàn thành sync posts cũ cho page %s", pageId)
	return nil
}

// Helper function: Parse updated_at từ Pancake (ISO 8601 string) sang Unix timestamp (seconds)
// Format: "2019-08-24T14:15:22.000000" hoặc "2019-08-24T14:15:22Z"
func parseCustomerUpdatedAt(updatedAtStr string) (int64, error) {
	layouts := []string{
		"2006-01-02T15:04:05.000000",
		"2006-01-02T15:04:05",
		time.RFC3339,
		time.RFC3339Nano,
	}

	for _, layout := range layouts {
		t, err := time.Parse(layout, updatedAtStr)
		if err == nil {
			return t.Unix(), nil
		}
	}

	return 0, errors.New("Không thể parse updated_at: " + updatedAtStr)
}

// BridgeV2_SyncNewCustomers sync customers đã cập nhật gần đây (incremental sync) cho tất cả pages
func BridgeV2_SyncNewCustomers() error {
	log.Println("[BridgeV2] Bắt đầu sync customers đã cập nhật gần đây (incremental sync)")

	// Lấy tất cả pages từ FolkForm
	limit := 50
	page := 1

	for {
		// Lấy danh sách các pages từ server FolkForm
		resultPages, err := FolkForm_GetFbPages(page, limit)
		if err != nil {
			logError("[BridgeV2] Lỗi khi lấy danh sách trang Facebook: %v", err)
			return errors.New("Lỗi khi lấy danh sách trang Facebook")
		}

		// Xử lý response - có thể là pagination object hoặc array trực tiếp
		items, itemCount, err := parseResponseData(resultPages)
		if err != nil {
			logError("[BridgeV2] LỖI khi parse response: %v", err)
			return err
		}

		if itemCount == 0 || len(items) == 0 {
			log.Printf("[BridgeV2] Không còn pages nào, dừng sync")
			break
		}

		log.Printf("[BridgeV2] Nhận được %d pages (page=%d, limit=%d)", len(items), page, limit)

		// Với mỗi page
		for _, item := range items {
			pageMap, ok := item.(map[string]interface{})
			if !ok {
				logError("[BridgeV2] Page không phải là map, bỏ qua")
				continue
			}

			pageId, ok := pageMap["pageId"].(string)
			if !ok {
				logError("[BridgeV2] Page không có pageId, bỏ qua")
				continue
			}

			// Kiểm tra isSync
			isSync, ok := pageMap["isSync"].(bool)
			if !ok || !isSync {
				log.Printf("[BridgeV2] Page %s - isSync=false, bỏ qua", pageId)
				continue
			}

			// Sync customers mới cho page này
			err = bridgeV2_SyncNewCustomersOfPage(pageId)
			if err != nil {
				logError("[BridgeV2] Lỗi khi sync customers mới cho page %s: %v", pageId, err)
				// Tiếp tục với page tiếp theo, không dừng toàn bộ job
			}
		}

		// Kiểm tra xem còn pages không
		if len(items) < limit {
			break
		}

		page++
	}

	log.Println("[BridgeV2] ✅ Hoàn thành sync customers đã cập nhật gần đây")
	return nil
}

// bridgeV2_SyncNewCustomersOfPage sync customers đã cập nhật gần đây (incremental sync) cho một page
func bridgeV2_SyncNewCustomersOfPage(pageId string) error {
	log.Printf("[BridgeV2] Bắt đầu sync customers đã cập nhật gần đây cho page %s", pageId)

	// 1. Lấy mốc từ FolkForm (FB customer collection)
	lastUpdatedAt, err := FolkForm_GetLastFbCustomerUpdatedAt(pageId)
	if err != nil {
		logError("[BridgeV2] Lỗi khi lấy lastUpdatedAt cho page %s: %v", pageId, err)
		return err
	}

	// 2. Tính khoảng thời gian sync
	var since, until int64
	if lastUpdatedAt == 0 {
		// Chưa có customers → sync 30 ngày gần nhất
		until = time.Now().Unix()
		since = until - (30 * 24 * 60 * 60) // 30 ngày trước
		log.Printf("[BridgeV2] Page %s - Chưa có customers, sync 30 ngày gần nhất", pageId)
	} else {
		since = lastUpdatedAt
		until = time.Now().Unix()
		log.Printf("[BridgeV2] Page %s - Sync customers từ %d đến %d", pageId, since, until)
	}

	// 3. Pagination loop
	pageNumber := 1
	pageSize := 100
	rateLimiter := apputility.GetPancakeRateLimiter()

	for {
		// Rate limiter
		rateLimiter.Wait()

		// Gọi Pancake API với order_by="updated_at"
		result, err := Pancake_GetCustomers(pageId, pageNumber, pageSize, since, until, "updated_at")
		if err != nil {
			logError("[BridgeV2] Lỗi khi lấy customers cho page %s: %v", pageId, err)
			break
		}

		// Parse customers
		customers, ok := result["customers"].([]interface{})
		if !ok || len(customers) == 0 {
			log.Printf("[BridgeV2] Page %s - Không còn customers nào, dừng sync", pageId)
			break
		}

		log.Printf("[BridgeV2] Page %s - Lấy được %d customers (page_number=%d)", pageId, len(customers), pageNumber)

		// 4. Xử lý từng customer
		foundOldCustomer := false
		for _, customer := range customers {
			customerMap, ok := customer.(map[string]interface{})
			if !ok {
				continue
			}

			// Parse updated_at từ Pancake
			updatedAtStr, ok := customerMap["updated_at"].(string)
			if !ok {
				log.Printf("[BridgeV2] Customer không có updated_at, bỏ qua")
				continue
			}

			// Convert ISO 8601 → Unix timestamp (seconds)
			updatedAtSeconds, err := parseCustomerUpdatedAt(updatedAtStr)
			if err != nil {
				log.Printf("[BridgeV2] Lỗi khi parse updated_at: %v", err)
				continue
			}

			// ⚠️ LOGIC DỪNG: Nếu customer cũ hơn since → đã sync hết
			if updatedAtSeconds < since {
				foundOldCustomer = true
				log.Printf("[BridgeV2] Page %s - Gặp customer cũ hơn since (%d < %d), dừng sync", pageId, updatedAtSeconds, since)
				break // Dừng xử lý batch này
			}

			// Đảm bảo page_id có trong customer data (Pancake API có thể không trả về)
			if _, ok := customerMap["page_id"]; !ok {
				customerMap["page_id"] = pageId
			}

			// ✅ Upsert FB customer (tự động xử lý duplicate theo customerId)
			_, err = FolkForm_UpsertFbCustomer(customer)
			if err != nil {
				logError("[BridgeV2] Lỗi khi upsert FB customer: %v", err)
				// Tiếp tục với customer tiếp theo
			}
		}

		// 5. Kiểm tra điều kiện dừng
		if foundOldCustomer {
			log.Printf("[BridgeV2] Page %s - Đã sync hết customers đã cập nhật gần đây (gặp customer cũ hơn since)", pageId)
			break
		}

		if len(customers) < pageSize {
			log.Printf("[BridgeV2] Page %s - Đã lấy hết customers (len=%d < page_size=%d)", pageId, len(customers), pageSize)
			break
		}

		// Tiếp tục với page tiếp theo
		pageNumber++
	}

	log.Printf("[BridgeV2] ✅ Hoàn thành sync customers đã cập nhật gần đây cho page %s", pageId)
	return nil
}

// BridgeV2_SyncAllCustomers sync customers cập nhật cũ (backfill sync) cho tất cả pages
func BridgeV2_SyncAllCustomers() error {
	log.Println("[BridgeV2] Bắt đầu sync customers cập nhật cũ (backfill sync)")

	// Lấy tất cả pages từ FolkForm
	limit := 50
	page := 1

	for {
		// Lấy danh sách các pages từ server FolkForm
		resultPages, err := FolkForm_GetFbPages(page, limit)
		if err != nil {
			logError("[BridgeV2] Lỗi khi lấy danh sách trang Facebook: %v", err)
			return errors.New("Lỗi khi lấy danh sách trang Facebook")
		}

		// Xử lý response - có thể là pagination object hoặc array trực tiếp
		items, itemCount, err := parseResponseData(resultPages)
		if err != nil {
			logError("[BridgeV2] LỖI khi parse response: %v", err)
			return err
		}

		if itemCount == 0 || len(items) == 0 {
			log.Printf("[BridgeV2] Không còn pages nào, dừng sync")
			break
		}

		log.Printf("[BridgeV2] Nhận được %d pages (page=%d, limit=%d)", len(items), page, limit)

		// Với mỗi page
		for _, item := range items {
			pageMap, ok := item.(map[string]interface{})
			if !ok {
				logError("[BridgeV2] Page không phải là map, bỏ qua")
				continue
			}

			pageId, ok := pageMap["pageId"].(string)
			if !ok {
				logError("[BridgeV2] Page không có pageId, bỏ qua")
				continue
			}

			// Kiểm tra isSync
			isSync, ok := pageMap["isSync"].(bool)
			if !ok || !isSync {
				log.Printf("[BridgeV2] Page %s - isSync=false, bỏ qua", pageId)
				continue
			}

			// Sync customers cũ cho page này
			err = bridgeV2_SyncAllCustomersOfPage(pageId)
			if err != nil {
				logError("[BridgeV2] Lỗi khi sync customers cũ cho page %s: %v", pageId, err)
				// Tiếp tục với page tiếp theo, không dừng toàn bộ job
			}
		}

		// Kiểm tra xem còn pages không
		if len(items) < limit {
			break
		}

		page++
	}

	log.Println("[BridgeV2] ✅ Hoàn thành sync customers cập nhật cũ")
	return nil
}

// bridgeV2_SyncAllCustomersOfPage sync customers cập nhật cũ (backfill sync) cho một page
func bridgeV2_SyncAllCustomersOfPage(pageId string) error {
	log.Printf("[BridgeV2] Bắt đầu sync customers cập nhật cũ cho page %s", pageId)

	// 1. Lấy mốc từ FolkForm (FB customer collection)
	oldestUpdatedAt, err := FolkForm_GetOldestFbCustomerUpdatedAt(pageId)
	if err != nil {
		logError("[BridgeV2] Lỗi khi lấy oldestUpdatedAt cho page %s: %v", pageId, err)
		return err
	}

	// 2. Tính khoảng thời gian sync
	var since, until int64
	if oldestUpdatedAt == 0 {
		// Chưa có customers → sync toàn bộ
		since = 0
		until = time.Now().Unix()
		log.Printf("[BridgeV2] Page %s - Chưa có customers, sync toàn bộ", pageId)
	} else {
		since = 0
		until = oldestUpdatedAt
		log.Printf("[BridgeV2] Page %s - Sync customers cập nhật cũ từ 0 đến %d", pageId, until)
	}

	// 3. Pagination loop
	pageNumber := 1
	pageSize := 100
	batchCount := 0
	const REFRESH_OLDEST_AFTER_BATCHES = 10
	rateLimiter := apputility.GetPancakeRateLimiter()

	for {
		// Refresh oldestUpdatedAt sau mỗi N batches
		if batchCount > 0 && batchCount%REFRESH_OLDEST_AFTER_BATCHES == 0 {
			newOldest, _ := FolkForm_GetOldestFbCustomerUpdatedAt(pageId)
			if newOldest > 0 && newOldest < until {
				// Có customer cũ hơn → cập nhật until
				log.Printf("[BridgeV2] Page %s - Cập nhật until: %d -> %d (có customer cũ hơn)", pageId, until, newOldest)
				until = newOldest
				oldestUpdatedAt = newOldest
			}
		}

		batchCount++

		// Rate limiter
		rateLimiter.Wait()

		// Gọi Pancake API với order_by="updated_at"
		result, err := Pancake_GetCustomers(pageId, pageNumber, pageSize, since, until, "updated_at")
		if err != nil {
			logError("[BridgeV2] Lỗi khi lấy customers cho page %s: %v", pageId, err)
			break
		}

		// Parse customers
		customers, ok := result["customers"].([]interface{})
		if !ok || len(customers) == 0 {
			log.Printf("[BridgeV2] Page %s - Không còn customers nào, dừng sync", pageId)
			break
		}

		log.Printf("[BridgeV2] Page %s - Lấy được %d customers cũ (page_number=%d, batch=%d)", pageId, len(customers), pageNumber, batchCount)

		// 4. Xử lý từng customer
		skippedCount := 0
		for _, customer := range customers {
			customerMap, ok := customer.(map[string]interface{})
			if !ok {
				continue
			}

			// Parse updated_at từ Pancake
			updatedAtStr, ok := customerMap["updated_at"].(string)
			if !ok {
				log.Printf("[BridgeV2] Customer không có updated_at, bỏ qua")
				continue
			}

			// Convert ISO 8601 → Unix timestamp (seconds)
			updatedAtSeconds, err := parseCustomerUpdatedAt(updatedAtStr)
			if err != nil {
				log.Printf("[BridgeV2] Lỗi khi parse updated_at: %v", err)
				continue
			}

			// ⚠️ LOGIC BỎ QUA: Nếu customer mới hơn until → bỏ qua (tiếp tục pagination)
			if updatedAtSeconds > until {
				skippedCount++
				continue // Bỏ qua, tiếp tục với customer tiếp theo
			}

			// Đảm bảo page_id có trong customer data (Pancake API có thể không trả về)
			if _, ok := customerMap["page_id"]; !ok {
				customerMap["page_id"] = pageId
			}

			// ✅ Upsert FB customer (tự động xử lý duplicate theo customerId)
			_, err = FolkForm_UpsertFbCustomer(customer)
			if err != nil {
				logError("[BridgeV2] Lỗi khi upsert FB customer: %v", err)
				// Tiếp tục với customer tiếp theo
			}
		}

		// 5. Kiểm tra điều kiện dừng
		if len(customers) < pageSize {
			log.Printf("[BridgeV2] Page %s - Đã lấy hết customers (len=%d < page_size=%d)", pageId, len(customers), pageSize)
			break
		}

		// Nếu tất cả customers đều bị bỏ qua → có thể đã hết customers cũ
		if skippedCount == len(customers) && len(customers) == pageSize {
			// Tiếp tục với page tiếp theo để kiểm tra
			log.Printf("[BridgeV2] Page %s - Tất cả customers đều bị bỏ qua (mới hơn until), tiếp tục pagination", pageId)
		}

		// Tiếp tục với page tiếp theo
		pageNumber++
	}

	log.Printf("[BridgeV2] ✅ Hoàn thành sync customers cập nhật cũ cho page %s", pageId)
	return nil
}

// BridgeV2_SyncNewCustomersFromPos đồng bộ customers mới từ POS về FolkForm (incremental sync)
func BridgeV2_SyncNewCustomersFromPos() error {
	log.Println("[BridgeV2] Bắt đầu sync customers mới từ POS (incremental sync)")

	// Lấy danh sách tokens từ FolkForm với filter system: "Pancake POS"
	filter := `{"system":"Pancake POS"}`
	page := 1
	limit := 50

	for {
		// Dừng nửa giây trước khi tiếp tục
		time.Sleep(100 * time.Millisecond)

		// Lấy danh sách access token với filter system: "Pancake POS"
		accessTokens, err := FolkForm_GetAccessTokens(page, limit, filter)
		if err != nil {
			logError("[BridgeV2] Lỗi khi lấy danh sách access token: %v", err)
			return errors.New("Lỗi khi lấy danh sách access token")
		}

		// Xử lý response - có thể là pagination object hoặc array trực tiếp
		items, itemCount, err := parseResponseData(accessTokens)
		if err != nil {
			logError("[BridgeV2] LỖI khi parse response: %v", err)
			return err
		}
		log.Printf("[BridgeV2] Nhận được %d access tokens (system: Pancake POS, page=%d, limit=%d)", len(items), page, limit)

		if itemCount > 0 && len(items) > 0 {
			// Với mỗi token
			for _, item := range items {
				// Dừng nửa giây trước khi tiếp tục
				time.Sleep(100 * time.Millisecond)

				// Chuyển item từ interface{} sang dạng map[string]interface{}
				itemMap, ok := item.(map[string]interface{})
				if !ok {
					logError("[BridgeV2] LỖI: Item không phải là map: %T", item)
					continue
				}

				// Lấy api_key từ item (đã được filter ở server, chỉ còn tokens có system: "Pancake POS")
				apiKey, ok := itemMap["value"].(string)
				if !ok {
					logError("[BridgeV2] LỖI: Không tìm thấy field 'value' trong item")
					continue
				}

				log.Printf("[BridgeV2] Đang đồng bộ customers mới với API key (system: Pancake POS, length: %d)", len(apiKey))

				// 1. Lấy danh sách shops
				shops, err := PancakePos_GetShops(apiKey)
				if err != nil {
					logError("[BridgeV2] Lỗi khi lấy danh sách shops: %v", err)
					continue
				}

				// 2. Với mỗi shop
				for _, shop := range shops {
					// Dừng nửa giây trước khi tiếp tục
					time.Sleep(100 * time.Millisecond)

					shopMap, ok := shop.(map[string]interface{})
					if !ok {
						logError("[BridgeV2] LỖI: Shop không phải là map: %T", shop)
						continue
					}

					// Lấy shopId từ shop
					var shopId int
					if shopIdRaw, ok := shopMap["id"]; ok {
						switch v := shopIdRaw.(type) {
						case float64:
							shopId = int(v)
						case int:
							shopId = v
						case int64:
							shopId = int(v)
						default:
							logError("[BridgeV2] LỖI: shopId không phải là số: %T", shopIdRaw)
							continue
						}
					} else {
						logError("[BridgeV2] LỖI: Không tìm thấy field 'id' trong shop")
						continue
					}

					// 3. Đồng bộ customers mới cho shop này
					err = bridgeV2_SyncNewCustomersFromPosForShop(apiKey, shopId)
					if err != nil {
						logError("[BridgeV2] Lỗi khi đồng bộ customers mới cho shop %d: %v", shopId, err)
						// Tiếp tục với shop tiếp theo
						continue
					}
				}

				log.Printf("[BridgeV2] Đã hoàn thành đồng bộ customers mới cho API key (length: %d)", len(apiKey))
			}
		} else {
			log.Println("[BridgeV2] Không còn access token nào. Kết thúc.")
			break
		}

		// Kiểm tra xem còn tokens không
		if len(items) < limit {
			break
		}

		page++
	}

	log.Println("[BridgeV2] ✅ Hoàn thành sync customers mới từ POS")
	return nil
}

// bridgeV2_SyncNewCustomersFromPosForShop đồng bộ customers mới từ POS cho một shop (incremental sync)
func bridgeV2_SyncNewCustomersFromPosForShop(apiKey string, shopId int) error {
	log.Printf("[BridgeV2] Bắt đầu đồng bộ customers mới từ POS cho shop %d (incremental sync)", shopId)

	// 1. Lấy mốc từ FolkForm
	// Filter: customers có posCustomerId (từ POS) và thuộc shop này
	// Sort theo updatedAt desc, limit 1 → lấy customer mới nhất
	lastUpdatedAt, err := FolkForm_GetLastPosCustomerUpdatedAt(shopId)
	if err != nil {
		logError("[BridgeV2] Lỗi khi lấy lastUpdatedAt cho shop %d: %v", shopId, err)
		return err
	}

	// 2. Tính khoảng thời gian sync
	var startTime, endTime int64
	if lastUpdatedAt == 0 {
		// Chưa có customers → sync 30 ngày gần nhất
		endTime = time.Now().Unix()
		startTime = endTime - (30 * 24 * 60 * 60) // 30 ngày trước
		log.Printf("[BridgeV2] Shop %d - Chưa có customers, sync 30 ngày gần nhất", shopId)
	} else {
		startTime = lastUpdatedAt
		endTime = time.Now().Unix()
		log.Printf("[BridgeV2] Shop %d - Sync customers từ %d đến %d", shopId, startTime, endTime)
	}

	// 3. Pagination loop
	pageNumber := 1
	pageSize := 100
	rateLimiter := apputility.GetPancakeRateLimiter()

	for {
		// Rate limiter
		rateLimiter.Wait()

		// Lấy customers từ POS với filter theo thời gian
		customers, err := PancakePos_GetCustomers(apiKey, shopId, pageNumber, pageSize, startTime, endTime)
		if err != nil {
			logError("[BridgeV2] Lỗi khi lấy customers cho shop %d: %v", shopId, err)
			break
		}

		if len(customers) == 0 {
			log.Printf("[BridgeV2] Shop %d - Không còn customers nào, dừng sync", shopId)
			break
		}

		log.Printf("[BridgeV2] Shop %d - Lấy được %d customers (page=%d)", shopId, len(customers), pageNumber)

		// 4. Xử lý từng customer
		foundOldCustomer := false
		for _, customer := range customers {
			customerMap, ok := customer.(map[string]interface{})
			if !ok {
				continue
			}

			// Parse updated_at từ POS (có thể là string hoặc number)
			var updatedAtSeconds int64 = 0
			if updatedAtStr, ok := customerMap["updated_at"].(string); ok {
				// Convert ISO 8601 → Unix timestamp (seconds)
				updatedAtSeconds, err = parseCustomerUpdatedAt(updatedAtStr)
				if err != nil {
					log.Printf("[BridgeV2] Lỗi khi parse updated_at: %v", err)
					continue
				}
			} else if updatedAtNum, ok := customerMap["updated_at"].(float64); ok {
				// Nếu là number (Unix timestamp)
				updatedAtSeconds = int64(updatedAtNum)
			} else {
				log.Printf("[BridgeV2] Customer không có updated_at, bỏ qua")
				continue
			}

			// ⚠️ LOGIC DỪNG: Nếu customer cũ hơn startTime → đã sync hết
			if updatedAtSeconds < startTime {
				foundOldCustomer = true
				log.Printf("[BridgeV2] Shop %d - Gặp customer cũ hơn startTime (%d < %d), dừng sync", shopId, updatedAtSeconds, startTime)
				break // Dừng xử lý batch này
			}

			// Đảm bảo shop_id có trong customer data (để lấy mốc sau này)
			if _, ok := customerMap["shop_id"]; !ok {
				customerMap["shop_id"] = shopId
			}

			// ✅ Upsert customer từ POS (tự động xử lý duplicate theo posCustomerId hoặc phone/email)
			_, err = FolkForm_UpsertCustomerFromPos(customerMap)
			if err != nil {
				logError("[BridgeV2] Lỗi khi upsert customer từ POS: %v", err)
				// Tiếp tục với customer tiếp theo
				continue
			}
		}

		// 5. Kiểm tra điều kiện dừng
		if foundOldCustomer {
			log.Printf("[BridgeV2] Shop %d - Đã sync hết customers mới (gặp customer cũ hơn startTime)", shopId)
			break
		}

		if len(customers) < pageSize {
			log.Printf("[BridgeV2] Shop %d - Đã lấy hết customers (len=%d < page_size=%d)", shopId, len(customers), pageSize)
			break
		}

		// Tiếp tục với page tiếp theo
		pageNumber++
	}

	log.Printf("[BridgeV2] ✅ Hoàn thành đồng bộ customers mới từ POS cho shop %d", shopId)
	return nil
}

// BridgeV2_SyncAllCustomersFromPos đồng bộ customers cũ từ POS về FolkForm (backfill sync)
func BridgeV2_SyncAllCustomersFromPos() error {
	log.Println("[BridgeV2] Bắt đầu sync customers cũ từ POS (backfill sync)")

	// Lấy danh sách tokens từ FolkForm với filter system: "Pancake POS"
	filter := `{"system":"Pancake POS"}`
	page := 1
	limit := 50

	for {
		// Dừng nửa giây trước khi tiếp tục
		time.Sleep(100 * time.Millisecond)

		// Lấy danh sách access token với filter system: "Pancake POS"
		accessTokens, err := FolkForm_GetAccessTokens(page, limit, filter)
		if err != nil {
			logError("[BridgeV2] Lỗi khi lấy danh sách access token: %v", err)
			return errors.New("Lỗi khi lấy danh sách access token")
		}

		// Xử lý response - có thể là pagination object hoặc array trực tiếp
		items, itemCount, err := parseResponseData(accessTokens)
		if err != nil {
			logError("[BridgeV2] LỖI khi parse response: %v", err)
			return err
		}
		log.Printf("[BridgeV2] Nhận được %d access tokens (system: Pancake POS, page=%d, limit=%d)", len(items), page, limit)

		if itemCount > 0 && len(items) > 0 {
			// Với mỗi token
			for _, item := range items {
				// Dừng nửa giây trước khi tiếp tục
				time.Sleep(100 * time.Millisecond)

				// Chuyển item từ interface{} sang dạng map[string]interface{}
				itemMap, ok := item.(map[string]interface{})
				if !ok {
					logError("[BridgeV2] LỖI: Item không phải là map: %T", item)
					continue
				}

				// Lấy api_key từ item (đã được filter ở server, chỉ còn tokens có system: "Pancake POS")
				apiKey, ok := itemMap["value"].(string)
				if !ok {
					logError("[BridgeV2] LỖI: Không tìm thấy field 'value' trong item")
					continue
				}

				log.Printf("[BridgeV2] Đang đồng bộ customers cũ với API key (system: Pancake POS, length: %d)", len(apiKey))

				// 1. Lấy danh sách shops
				shops, err := PancakePos_GetShops(apiKey)
				if err != nil {
					logError("[BridgeV2] Lỗi khi lấy danh sách shops: %v", err)
					continue
				}

				// 2. Với mỗi shop
				for _, shop := range shops {
					// Dừng nửa giây trước khi tiếp tục
					time.Sleep(100 * time.Millisecond)

					shopMap, ok := shop.(map[string]interface{})
					if !ok {
						logError("[BridgeV2] LỖI: Shop không phải là map: %T", shop)
						continue
					}

					// Lấy shopId từ shop
					var shopId int
					if shopIdRaw, ok := shopMap["id"]; ok {
						switch v := shopIdRaw.(type) {
						case float64:
							shopId = int(v)
						case int:
							shopId = v
						case int64:
							shopId = int(v)
						default:
							logError("[BridgeV2] LỖI: shopId không phải là số: %T", shopIdRaw)
							continue
						}
					} else {
						logError("[BridgeV2] LỖI: Không tìm thấy field 'id' trong shop")
						continue
					}

					// 3. Đồng bộ customers cũ cho shop này
					err = bridgeV2_SyncAllCustomersFromPosForShop(apiKey, shopId)
					if err != nil {
						logError("[BridgeV2] Lỗi khi đồng bộ customers cũ cho shop %d: %v", shopId, err)
						// Tiếp tục với shop tiếp theo
						continue
					}
				}

				log.Printf("[BridgeV2] Đã hoàn thành đồng bộ customers cũ cho API key (length: %d)", len(apiKey))
			}
		} else {
			log.Println("[BridgeV2] Không còn access token nào. Kết thúc.")
			break
		}

		// Kiểm tra xem còn tokens không
		if len(items) < limit {
			break
		}

		page++
	}

	log.Println("[BridgeV2] ✅ Hoàn thành sync customers cũ từ POS")
	return nil
}

// bridgeV2_SyncAllCustomersFromPosForShop đồng bộ customers cũ từ POS cho một shop (backfill sync)
func bridgeV2_SyncAllCustomersFromPosForShop(apiKey string, shopId int) error {
	log.Printf("[BridgeV2] Bắt đầu đồng bộ customers cũ từ POS cho shop %d (backfill sync)", shopId)

	// 1. Lấy mốc từ FolkForm
	// Filter: customers có posCustomerId (từ POS) và thuộc shop này
	// Sort theo updatedAt asc, limit 1 → lấy customer cũ nhất
	oldestUpdatedAt, err := FolkForm_GetOldestPosCustomerUpdatedAt(shopId)
	if err != nil {
		logError("[BridgeV2] Lỗi khi lấy oldestUpdatedAt cho shop %d: %v", shopId, err)
		return err
	}

	// 2. Tính khoảng thời gian sync
	var startTime, endTime int64
	if oldestUpdatedAt == 0 {
		// Chưa có customers → sync toàn bộ
		startTime = 0
		endTime = time.Now().Unix()
		log.Printf("[BridgeV2] Shop %d - Chưa có customers, sync toàn bộ", shopId)
	} else {
		startTime = 0
		endTime = oldestUpdatedAt
		log.Printf("[BridgeV2] Shop %d - Sync customers cũ từ 0 đến %d", shopId, endTime)
	}

	// 3. Pagination loop với refresh mốc
	pageNumber := 1
	pageSize := 100
	batchCount := 0
	const REFRESH_OLDEST_AFTER_BATCHES = 10
	rateLimiter := apputility.GetPancakeRateLimiter()

	for {
		// Refresh oldestUpdatedAt sau mỗi N batches
		if batchCount > 0 && batchCount%REFRESH_OLDEST_AFTER_BATCHES == 0 {
			newOldest, _ := FolkForm_GetOldestPosCustomerUpdatedAt(shopId)
			if newOldest > 0 && newOldest < endTime {
				// Có customer cũ hơn → cập nhật endTime
				log.Printf("[BridgeV2] Shop %d - Cập nhật endTime: %d -> %d (có customer cũ hơn)", shopId, endTime, newOldest)
				endTime = newOldest
				oldestUpdatedAt = newOldest
			}
		}

		batchCount++

		// Rate limiter
		rateLimiter.Wait()

		// Lấy customers từ POS với filter theo thời gian
		customers, err := PancakePos_GetCustomers(apiKey, shopId, pageNumber, pageSize, startTime, endTime)
		if err != nil {
			logError("[BridgeV2] Lỗi khi lấy customers cho shop %d: %v", shopId, err)
			break
		}

		if len(customers) == 0 {
			log.Printf("[BridgeV2] Shop %d - Không còn customers nào, dừng sync", shopId)
			break
		}

		log.Printf("[BridgeV2] Shop %d - Lấy được %d customers cũ (page_number=%d, batch=%d)", shopId, len(customers), pageNumber, batchCount)

		// 4. Xử lý từng customer
		skippedCount := 0
		for _, customer := range customers {
			customerMap, ok := customer.(map[string]interface{})
			if !ok {
				continue
			}

			// Parse updated_at từ POS (có thể là string hoặc number)
			var updatedAtSeconds int64 = 0
			if updatedAtStr, ok := customerMap["updated_at"].(string); ok {
				// Convert ISO 8601 → Unix timestamp (seconds)
				updatedAtSeconds, err = parseCustomerUpdatedAt(updatedAtStr)
				if err != nil {
					log.Printf("[BridgeV2] Lỗi khi parse updated_at: %v", err)
					continue
				}
			} else if updatedAtNum, ok := customerMap["updated_at"].(float64); ok {
				// Nếu là number (Unix timestamp)
				updatedAtSeconds = int64(updatedAtNum)
			} else {
				log.Printf("[BridgeV2] Customer không có updated_at, bỏ qua")
				continue
			}

			// ⚠️ LOGIC BỎ QUA: Nếu customer mới hơn endTime → bỏ qua (tiếp tục pagination)
			if updatedAtSeconds > endTime {
				skippedCount++
				continue // Bỏ qua, tiếp tục với customer tiếp theo
			}

			// Đảm bảo shop_id có trong customer data (để lấy mốc sau này)
			if _, ok := customerMap["shop_id"]; !ok {
				customerMap["shop_id"] = shopId
			}

			// ✅ Upsert customer từ POS (tự động xử lý duplicate theo posCustomerId hoặc phone/email)
			_, err = FolkForm_UpsertCustomerFromPos(customerMap)
			if err != nil {
				logError("[BridgeV2] Lỗi khi upsert customer từ POS: %v", err)
				// Tiếp tục với customer tiếp theo
				continue
			}
		}

		// 5. Kiểm tra điều kiện dừng
		if len(customers) < pageSize {
			log.Printf("[BridgeV2] Shop %d - Đã lấy hết customers (len=%d < page_size=%d)", shopId, len(customers), pageSize)
			break
		}

		// Nếu tất cả customers đều bị bỏ qua → có thể đã hết customers cũ
		if skippedCount == len(customers) && len(customers) == pageSize {
			// Tiếp tục với page tiếp theo để kiểm tra
			log.Printf("[BridgeV2] Shop %d - Tất cả customers đều bị bỏ qua (mới hơn endTime), tiếp tục pagination", shopId)
		}

		// Tiếp tục với page tiếp theo
		pageNumber++
	}

	log.Printf("[BridgeV2] ✅ Hoàn thành đồng bộ customers cũ từ POS cho shop %d", shopId)
	return nil
}

// parseOrderUpdatedAt parse updated_at từ Pancake POS (ISO 8601) sang Unix timestamp (seconds)
func parseOrderUpdatedAt(updatedAtStr string) (int64, error) {
	layouts := []string{
		"2006-01-02T15:04:05.000000Z",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05.000000",
		"2006-01-02T15:04:05",
		time.RFC3339,
		time.RFC3339Nano,
	}

	for _, layout := range layouts {
		t, err := time.Parse(layout, updatedAtStr)
		if err == nil {
			return t.Unix(), nil
		}
	}

	return 0, errors.New("Không thể parse updated_at: " + updatedAtStr)
}

// BridgeV2_SyncNewOrders đồng bộ orders mới từ POS về FolkForm (incremental sync)
func BridgeV2_SyncNewOrders() error {
	log.Println("[BridgeV2] Bắt đầu sync orders mới từ POS (incremental sync)")

	// Lấy danh sách tokens từ FolkForm với filter system: "Pancake POS"
	filter := `{"system":"Pancake POS"}`
	page := 1
	limit := 50

	for {
		// Dừng nửa giây trước khi tiếp tục
		time.Sleep(100 * time.Millisecond)

		// Lấy danh sách access token với filter system: "Pancake POS"
		accessTokens, err := FolkForm_GetAccessTokens(page, limit, filter)
		if err != nil {
			logError("[BridgeV2] Lỗi khi lấy danh sách access token: %v", err)
			return errors.New("Lỗi khi lấy danh sách access token")
		}

		// Xử lý response - có thể là pagination object hoặc array trực tiếp
		items, itemCount, err := parseResponseData(accessTokens)
		if err != nil {
			logError("[BridgeV2] LỖI khi parse response: %v", err)
			return err
		}
		log.Printf("[BridgeV2] Nhận được %d access tokens (system: Pancake POS, page=%d, limit=%d)", len(items), page, limit)

		if itemCount > 0 && len(items) > 0 {
			// Với mỗi token
			for _, item := range items {
				// Dừng nửa giây trước khi tiếp tục
				time.Sleep(100 * time.Millisecond)

				// Chuyển item từ interface{} sang dạng map[string]interface{}
				itemMap, ok := item.(map[string]interface{})
				if !ok {
					logError("[BridgeV2] LỖI: Item không phải là map: %T", item)
					continue
				}

				// Lấy api_key từ item (đã được filter ở server, chỉ còn tokens có system: "Pancake POS")
				apiKey, ok := itemMap["value"].(string)
				if !ok {
					logError("[BridgeV2] LỖI: Không tìm thấy field 'value' trong item")
					continue
				}

				log.Printf("[BridgeV2] Đang đồng bộ orders mới với API key (system: Pancake POS, length: %d)", len(apiKey))

				// 1. Lấy danh sách shops
				shops, err := PancakePos_GetShops(apiKey)
				if err != nil {
					logError("[BridgeV2] Lỗi khi lấy danh sách shops: %v", err)
					continue
				}

				// 2. Với mỗi shop
				for _, shop := range shops {
					// Dừng nửa giây trước khi tiếp tục
					time.Sleep(100 * time.Millisecond)

					shopMap, ok := shop.(map[string]interface{})
					if !ok {
						logError("[BridgeV2] LỖI: Shop không phải là map: %T", shop)
						continue
					}

					// Lấy shopId từ shop
					var shopId int
					if shopIdRaw, ok := shopMap["id"]; ok {
						switch v := shopIdRaw.(type) {
						case float64:
							shopId = int(v)
						case int:
							shopId = v
						case int64:
							shopId = int(v)
						default:
							logError("[BridgeV2] LỖI: shopId không phải là số: %T", shopIdRaw)
							continue
						}
					} else {
						logError("[BridgeV2] LỖI: Không tìm thấy field 'id' trong shop")
						continue
					}

					// 3. Đồng bộ orders mới cho shop này
					err = bridgeV2_SyncNewOrdersForShop(apiKey, shopId)
					if err != nil {
						logError("[BridgeV2] Lỗi khi đồng bộ orders mới cho shop %d: %v", shopId, err)
						// Tiếp tục với shop tiếp theo
						continue
					}
				}

				log.Printf("[BridgeV2] Đã hoàn thành đồng bộ orders mới cho API key (length: %d)", len(apiKey))
			}
		} else {
			log.Println("[BridgeV2] Không còn access token nào. Kết thúc.")
			break
		}

		// Kiểm tra xem còn tokens không
		if len(items) < limit {
			break
		}

		page++
	}

	log.Println("[BridgeV2] ✅ Hoàn thành sync orders mới từ POS")
	return nil
}

// bridgeV2_SyncNewOrdersForShop sync orders đã cập nhật gần đây (incremental sync) cho một shop
func bridgeV2_SyncNewOrdersForShop(apiKey string, shopId int) error {
	log.Printf("[BridgeV2] Bắt đầu sync orders đã cập nhật gần đây cho shop %d", shopId)

	// 1. Lấy mốc từ FolkForm
	lastUpdatedAt, err := FolkForm_GetLastOrderUpdatedAt(shopId)
	if err != nil {
		logError("[BridgeV2] Lỗi khi lấy lastUpdatedAt cho shop %d: %v", shopId, err)
		return err
	}

	// 2. Tính khoảng thời gian sync
	var since, until int64
	if lastUpdatedAt == 0 {
		// Chưa có orders → sync 30 ngày gần nhất
		until = time.Now().Unix()
		since = until - (30 * 24 * 60 * 60) // 30 ngày trước
		log.Printf("[BridgeV2] Shop %d - Chưa có orders, sync 30 ngày gần nhất", shopId)
	} else {
		since = lastUpdatedAt
		until = time.Now().Unix()
		log.Printf("[BridgeV2] Shop %d - Sync orders từ %d đến %d", shopId, since, until)
	}

	// 3. Pagination loop
	pageNumber := 1
	pageSize := 100
	rateLimiter := apputility.GetPancakeRateLimiter()

	for {
		// Rate limiter
		rateLimiter.Wait()

		// Gọi Pancake POS API với updateStatus="updated_at"
		result, err := PancakePos_GetOrders(apiKey, shopId, pageNumber, pageSize, "updated_at")
		if err != nil {
			logError("[BridgeV2] Lỗi khi lấy orders cho shop %d: %v", shopId, err)
			break
		}

		// Parse orders
		orders, ok := result["orders"].([]interface{})
		if !ok || len(orders) == 0 {
			log.Printf("[BridgeV2] Shop %d - Không còn orders nào, dừng sync", shopId)
			break
		}

		log.Printf("[BridgeV2] Shop %d - Lấy được %d orders (page_number=%d)", shopId, len(orders), pageNumber)

		// 4. Xử lý từng order
		foundOldOrder := false
		for _, order := range orders {
			orderMap, ok := order.(map[string]interface{})
			if !ok {
				continue
			}

			// Parse updated_at từ Pancake POS
			updatedAtStr, ok := orderMap["updated_at"].(string)
			if !ok {
				log.Printf("[BridgeV2] Order không có updated_at, bỏ qua")
				continue
			}

			// Convert ISO 8601 → Unix timestamp (seconds)
			updatedAtSeconds, err := parseOrderUpdatedAt(updatedAtStr)
			if err != nil {
				log.Printf("[BridgeV2] Lỗi khi parse updated_at: %v", err)
				continue
			}

			// ⚠️ LOGIC DỪNG: Nếu order cũ hơn since → đã sync hết
			if updatedAtSeconds < since {
				foundOldOrder = true
				log.Printf("[BridgeV2] Shop %d - Gặp order cũ hơn since (%d < %d), dừng sync", shopId, updatedAtSeconds, since)
				break // Dừng xử lý batch này
			}

			// ✅ Upsert order (tự động xử lý duplicate theo orderId + shopId)
			_, err = FolkForm_CreatePcPosOrder(order)
			if err != nil {
				logError("[BridgeV2] Lỗi khi upsert order: %v", err)
				// Tiếp tục với order tiếp theo
			}
		}

		// 5. Kiểm tra điều kiện dừng
		if foundOldOrder {
			log.Printf("[BridgeV2] Shop %d - Đã sync hết orders đã cập nhật gần đây (gặp order cũ hơn since)", shopId)
			break
		}

		if len(orders) < pageSize {
			log.Printf("[BridgeV2] Shop %d - Đã lấy hết orders (len=%d < page_size=%d)", shopId, len(orders), pageSize)
			break
		}

		// Tiếp tục với page tiếp theo
		pageNumber++
	}

	log.Printf("[BridgeV2] ✅ Hoàn thành sync orders đã cập nhật gần đây cho shop %d", shopId)
	return nil
}

// BridgeV2_SyncAllOrders đồng bộ orders cũ từ POS về FolkForm (backfill sync)
func BridgeV2_SyncAllOrders() error {
	log.Println("[BridgeV2] Bắt đầu sync orders cũ từ POS (backfill sync)")

	// Lấy danh sách tokens từ FolkForm với filter system: "Pancake POS"
	filter := `{"system":"Pancake POS"}`
	page := 1
	limit := 50

	for {
		// Dừng nửa giây trước khi tiếp tục
		time.Sleep(100 * time.Millisecond)

		// Lấy danh sách access token với filter system: "Pancake POS"
		accessTokens, err := FolkForm_GetAccessTokens(page, limit, filter)
		if err != nil {
			logError("[BridgeV2] Lỗi khi lấy danh sách access token: %v", err)
			return errors.New("Lỗi khi lấy danh sách access token")
		}

		// Xử lý response - có thể là pagination object hoặc array trực tiếp
		items, itemCount, err := parseResponseData(accessTokens)
		if err != nil {
			logError("[BridgeV2] LỖI khi parse response: %v", err)
			return err
		}
		log.Printf("[BridgeV2] Nhận được %d access tokens (system: Pancake POS, page=%d, limit=%d)", len(items), page, limit)

		if itemCount > 0 && len(items) > 0 {
			// Với mỗi token
			for _, item := range items {
				// Dừng nửa giây trước khi tiếp tục
				time.Sleep(100 * time.Millisecond)

				// Chuyển item từ interface{} sang dạng map[string]interface{}
				itemMap, ok := item.(map[string]interface{})
				if !ok {
					logError("[BridgeV2] LỖI: Item không phải là map: %T", item)
					continue
				}

				// Lấy api_key từ item (đã được filter ở server, chỉ còn tokens có system: "Pancake POS")
				apiKey, ok := itemMap["value"].(string)
				if !ok {
					logError("[BridgeV2] LỖI: Không tìm thấy field 'value' trong item")
					continue
				}

				log.Printf("[BridgeV2] Đang đồng bộ orders cũ với API key (system: Pancake POS, length: %d)", len(apiKey))

				// 1. Lấy danh sách shops
				shops, err := PancakePos_GetShops(apiKey)
				if err != nil {
					logError("[BridgeV2] Lỗi khi lấy danh sách shops: %v", err)
					continue
				}

				// 2. Với mỗi shop
				for _, shop := range shops {
					// Dừng nửa giây trước khi tiếp tục
					time.Sleep(100 * time.Millisecond)

					shopMap, ok := shop.(map[string]interface{})
					if !ok {
						logError("[BridgeV2] LỖI: Shop không phải là map: %T", shop)
						continue
					}

					// Lấy shopId từ shop
					var shopId int
					if shopIdRaw, ok := shopMap["id"]; ok {
						switch v := shopIdRaw.(type) {
						case float64:
							shopId = int(v)
						case int:
							shopId = v
						case int64:
							shopId = int(v)
						default:
							logError("[BridgeV2] LỖI: shopId không phải là số: %T", shopIdRaw)
							continue
						}
					} else {
						logError("[BridgeV2] LỖI: Không tìm thấy field 'id' trong shop")
						continue
					}

					// 3. Đồng bộ orders cũ cho shop này
					err = bridgeV2_SyncAllOrdersForShop(apiKey, shopId)
					if err != nil {
						logError("[BridgeV2] Lỗi khi đồng bộ orders cũ cho shop %d: %v", shopId, err)
						// Tiếp tục với shop tiếp theo
						continue
					}
				}

				log.Printf("[BridgeV2] Đã hoàn thành đồng bộ orders cũ cho API key (length: %d)", len(apiKey))
			}
		} else {
			log.Println("[BridgeV2] Không còn access token nào. Kết thúc.")
			break
		}

		// Kiểm tra xem còn tokens không
		if len(items) < limit {
			break
		}

		page++
	}

	log.Println("[BridgeV2] ✅ Hoàn thành sync orders cũ từ POS")
	return nil
}

// bridgeV2_SyncAllOrdersForShop sync orders cập nhật cũ (backfill sync) cho một shop
func bridgeV2_SyncAllOrdersForShop(apiKey string, shopId int) error {
	log.Printf("[BridgeV2] Bắt đầu sync orders cập nhật cũ cho shop %d", shopId)

	// 1. Lấy mốc từ FolkForm
	oldestUpdatedAt, err := FolkForm_GetOldestOrderUpdatedAt(shopId)
	if err != nil {
		logError("[BridgeV2] Lỗi khi lấy oldestUpdatedAt cho shop %d: %v", shopId, err)
		return err
	}

	// 2. Tính khoảng thời gian sync
	var since, endTime int64
	if oldestUpdatedAt == 0 {
		// Chưa có orders → sync toàn bộ
		since = 0
		endTime = time.Now().Unix()
		log.Printf("[BridgeV2] Shop %d - Chưa có orders, sync toàn bộ", shopId)
	} else {
		since = 0
		endTime = oldestUpdatedAt
		log.Printf("[BridgeV2] Shop %d - Sync orders từ %d đến %d", shopId, since, endTime)
	}

	// 3. Pagination loop
	pageNumber := 1
	pageSize := 100
	rateLimiter := apputility.GetPancakeRateLimiter()
	batchCount := 0

	for {
		// Rate limiter
		rateLimiter.Wait()

		// Refresh oldestUpdatedAt sau mỗi 10 batches
		if batchCount > 0 && batchCount%10 == 0 {
			newOldestUpdatedAt, err := FolkForm_GetOldestOrderUpdatedAt(shopId)
			if err == nil && newOldestUpdatedAt > 0 && newOldestUpdatedAt < endTime {
				endTime = newOldestUpdatedAt
				log.Printf("[BridgeV2] Shop %d - Đã refresh oldestUpdatedAt: %d", shopId, endTime)
			}
		}

		// Gọi Pancake POS API với updateStatus="updated_at"
		result, err := PancakePos_GetOrders(apiKey, shopId, pageNumber, pageSize, "updated_at")
		if err != nil {
			logError("[BridgeV2] Lỗi khi lấy orders cho shop %d: %v", shopId, err)
			break
		}

		// Parse orders
		orders, ok := result["orders"].([]interface{})
		if !ok || len(orders) == 0 {
			log.Printf("[BridgeV2] Shop %d - Không còn orders nào, dừng sync", shopId)
			break
		}

		log.Printf("[BridgeV2] Shop %d - Lấy được %d orders (page_number=%d)", shopId, len(orders), pageNumber)

		// 4. Xử lý từng order
		skippedCount := 0
		for _, order := range orders {
			orderMap, ok := order.(map[string]interface{})
			if !ok {
				continue
			}

			// Parse updated_at từ Pancake POS
			var updatedAtSeconds int64
			if updatedAtStr, ok := orderMap["updated_at"].(string); ok {
				// Convert ISO 8601 → Unix timestamp (seconds)
				updatedAtSeconds, err = parseOrderUpdatedAt(updatedAtStr)
				if err != nil {
					log.Printf("[BridgeV2] Lỗi khi parse updated_at: %v", err)
					continue
				}
			} else if updatedAtNum, ok := orderMap["updated_at"].(float64); ok {
				// Nếu là number (Unix timestamp)
				updatedAtSeconds = int64(updatedAtNum)
			} else {
				log.Printf("[BridgeV2] Order không có updated_at, bỏ qua")
				continue
			}

			// ⚠️ LOGIC BỎ QUA: Nếu order mới hơn endTime → bỏ qua (tiếp tục pagination)
			if updatedAtSeconds > endTime {
				skippedCount++
				continue // Bỏ qua, tiếp tục với order tiếp theo
			}

			// ✅ Upsert order (tự động xử lý duplicate theo orderId + shopId)
			_, err = FolkForm_CreatePcPosOrder(order)
			if err != nil {
				logError("[BridgeV2] Lỗi khi upsert order: %v", err)
				// Tiếp tục với order tiếp theo
				continue
			}
		}

		// 5. Kiểm tra điều kiện dừng
		if len(orders) < pageSize {
			log.Printf("[BridgeV2] Shop %d - Đã lấy hết orders (len=%d < page_size=%d)", shopId, len(orders), pageSize)
			break
		}

		// Nếu tất cả orders đều bị bỏ qua → có thể đã hết orders cũ
		if skippedCount == len(orders) && len(orders) == pageSize {
			// Tiếp tục với page tiếp theo để kiểm tra
			log.Printf("[BridgeV2] Shop %d - Tất cả orders đều bị bỏ qua (mới hơn endTime), tiếp tục pagination", shopId)
		}

		// Tiếp tục với page tiếp theo
		pageNumber++
		batchCount++
	}

	log.Printf("[BridgeV2] ✅ Hoàn thành đồng bộ orders cũ từ POS cho shop %d", shopId)
	return nil
}
