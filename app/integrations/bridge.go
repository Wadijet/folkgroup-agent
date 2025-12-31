package integrations

import (
	"errors"
	"fmt"
	"log"
	"time"

	apputility "agent_pancake/app/utility"
	"agent_pancake/global"

	"go.mongodb.org/mongo-driver/bson"
)

// ANSI color codes cho terminal
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
	colorGreen  = "\033[32m"
)

// logError in log lỗi với màu đỏ để dễ theo dõi
func logError(format string, v ...interface{}) {
	message := fmt.Sprintf(format, v...)
	log.Printf("%s%s%s", colorRed, message, colorReset)
}

// Helper function: Xử lý response data an toàn - hỗ trợ cả array và pagination object
// Trả về items ([]interface{}) và itemCount (float64)
func parseResponseData(response map[string]interface{}) (items []interface{}, itemCount float64, err error) {
	dataRaw, ok := response["data"]
	if !ok {
		return nil, 0, errors.New("Response không có field 'data'")
	}

	switch v := dataRaw.(type) {
	case []interface{}:
		// Data là array trực tiếp
		items = v
		itemCount = float64(len(items))
		return items, itemCount, nil
	case map[string]interface{}:
		// Data là object có pagination
		data := v
		if itemCountRaw, ok := data["itemCount"]; ok {
			if count, ok := itemCountRaw.(float64); ok {
				itemCount = count
			} else if count, ok := itemCountRaw.(int); ok {
				itemCount = float64(count)
			}
		}
		if itemsRaw, ok := data["items"]; ok {
			if itemsArray, ok := itemsRaw.([]interface{}); ok {
				items = itemsArray
			}
		}
		return items, itemCount, nil
	default:
		return nil, 0, errors.New("Kiểu dữ liệu response không hợp lệ")
	}
}

// ========================================================================================================
// Hàm xử lý logic trên server FolkForm
// ========================================================================================================

// Hàm Bridge_SyncPages(access_token string) sẽ đồng bộ danh sách trang Facebook từ server Pancake về server FolkForm
// - Lấy danh sách trang từ server Pancake
// - Đẩy danh sách trang vào server FolkForm
func bridge_SyncPagesOfAccessToken(access_token string) (resultErr error) {

	log.Println("Đang đồng bộ trang với access token:", access_token)

	// Lấy danh sách trang từ server Pancake
	resultPages, err := PanCake_GetFbPages(access_token)
	if err != nil {
		logError("Lỗi khi lấy danh sách trang Facebook: %v", err)
		return errors.New("Lỗi khi lấy danh sách trang Facebook")
	}

	// Lấy data lưu trong resultPages dạng []interface{} ở categorizedactivated
	activePages := resultPages["categorized"].(map[string]interface{})["activated"].([]interface{})
	for _, page := range activePages {

		// Dừng nửa giây trước khi tiếp tục
		time.Sleep(100 * time.Millisecond)

		log.Println("Đang tạo trang trên FolkForm với access token:", access_token)

		FolkForm_CreateFbPage(access_token, page)

	}

	log.Println("Đồng bộ trang với access token thành công:", access_token)

	return nil
}

// Hàm Bridge_SyncPages sẽ đồng bộ danh sách trang Facebook từ server Pancake về server FolkForm
// - Lấy danh sách access token từ server FolkForm
// - Gọi hàm Bridge_SyncPagesOfAccessToken để đồng bộ trang của từng access token
func Bridge_SyncPages() (resultErr error) {

	log.Println("Bắt đầu đồng bộ trang Facebook từ server Pancake về server FolkForm...")

	limit := 50
	page := 1

	for {

		// Dừng nửa giây trước khi tiếp tục
		time.Sleep(100 * time.Millisecond)

		// Lấy danh sách access token với filter system: "Pancake"
		// Filter được xử lý ở server để chỉ lấy tokens có system: "Pancake"
		filter := `{"system":"Pancake"}`
		accessTokens, err := FolkForm_GetAccessTokens(page, limit, filter)
		if err != nil {
			logError("Lỗi khi lấy danh sách access token: %v", err)
			return errors.New("Lỗi khi lấy danh sách access token")
		}

		// Xử lý response - có thể là pagination object hoặc array trực tiếp
		items, itemCount, err := parseResponseData(accessTokens)
		if err != nil {
			logError("[Bridge_SyncPages] LỖI khi parse response: %v", err)
			return err
		}
		log.Printf("[Bridge_SyncPages] Nhận được %d access tokens (system: Pancake, page=%d, limit=%d)", len(items), page, limit)

		if itemCount > 0 && len(items) > 0 {
			for _, item := range items {
				// Dừng nửa giây trước khi tiếp tục
				time.Sleep(100 * time.Millisecond)

				// chuyển item từ interface{} sang dạng map[string]interface{}
				itemMap, ok := item.(map[string]interface{})
				if !ok {
					logError("[Bridge_SyncPages] LỖI: Item không phải là map: %T", item)
					continue
				}

				// Lấy access_token từ item (đã được filter ở server, chỉ còn tokens có system: "Pancake")
				access_token, ok := itemMap["value"].(string)
				if !ok {
					logError("[Bridge_SyncPages] LỖI: Không tìm thấy field 'value' trong item")
					continue
				}

				// Gọi hàm bridge_SyncPagesOfAccessToken để đồng bộ trang với access token
				log.Printf("[Bridge_SyncPages] Đang đồng bộ trang với access token (system: Pancake): %s", access_token)
				bridge_SyncPagesOfAccessToken(access_token)
			}

		} else {
			log.Println("[Bridge_SyncPages] Không còn access token nào. Kết thúc.")
			break
		}

		page++
		continue
	}

	log.Println("Đồng bộ trang Facebook từ server Pancake về server FolkForm thành công")

	return nil
}

// Hàm FolkForm_UpdarePageAccessToken sẽ cập nhật page_access_token của trang Facebook trên server FolkForm bằng cách:
// - Gửi yêu cầu tạo page_access_token lên server PanCake
// - Lấy page_access_token từ phản hồi và cập nhật lên server FolkForm
func Bridge_UpdatePagesAccessToken_toFolkForm() (resultErr error) {

	limit := 50
	page := 1

	for {
		// Dừng nửa giây trước khi tiếp tục
		time.Sleep(100 * time.Millisecond)

		// Lấy danh sách các pages từ server FolkForm
		resultPages, err := FolkForm_GetFbPages(page, limit)
		if err != nil {
			logError("Lỗi khi lấy danh sách trang Facebook: %v", err)
			return errors.New("Lỗi khi lấy danh sách trang Facebook")
		}

		data := resultPages["data"].(map[string]interface{})
		itemCount := data["itemCount"].(float64)

		if itemCount > 0 {
			items := data["items"].([]interface{})

			if len(items) > 0 {
				for _, item := range items {

					// Dừng nửa giây trước khi tiếp tục
					time.Sleep(100 * time.Millisecond)

					// chuyển item từ interface{} sang dạng map[string]interface{}
					page := item.(map[string]interface{})
					page_id := page["pageId"].(string)
					access_token := page["accessToken"].(string)

					log.Println("Đang kiểm tra page_access_token cho trang:", page_id)

					// Gọi hàm Pancake_GetConversations_v2 để test page_access_token có hợp lệ không
					// Truyền 0, 0 cho since/until vì chỉ cần test token
					// unread_first=false vì chỉ test token, không cần ưu tiên unread
					_, err := Pancake_GetConversations_v2(page_id, "", 0, 0, "", false)
					if err == nil {
						log.Println("Page_access_token vẫn còn hiệu lực cho trang:", page_id)
						continue
					}

					// Gọi hàm PanCake_GeneratePageAccessToken để lấy page_access_token
					resultGeneratePageAccessToken, err := PanCake_GeneratePageAccessToken(page_id, access_token)
					if err != nil {
						logError("Lỗi khi lấy page access token: %v", err)
						continue
					}

					// chuyển resultGeneratePageAccessToken từ interface{} sang dạng map[string]interface{}
					page_access_token := resultGeneratePageAccessToken["page_access_token"].(string)
					// Gọi hàm FolkForm_UpdatePageAccessToken để cập nhật page_access_token
					_, err = FolkForm_UpdatePageAccessToken(page_id, page_access_token)
					if err != nil {
						logError("Lỗi khi cập nhật page access token: %v", err)
						continue
					}
					log.Println("Cập nhật page access token thành công cho trang:", page_id)
				}
			}

			page++
			continue
		} else {
			break
		}
	}

	log.Println("Cập nhật page access token cho tất cả các trang thành công")

	return nil
}

// Hàm Bridge_SyncPagesFolkformToLocal sẽ đồng bộ danh sách trang Facebook từ server FolkForm về server local
// - Lấy danh sách trang từ server FolkForm
// - Đẩy danh sách trang vào server local
func Bridge_SyncPagesFolkformToLocal() (resultErr error) {
	limit := 50
	page := 1

	for {
		// Dừng nửa giây trước khi tiếp tục
		time.Sleep(100 * time.Millisecond)

		// Lấy danh sách các pages từ server FolkForm
		resultPages, err := FolkForm_GetFbPages(page, limit)
		if err != nil {
			logError("Lỗi khi lấy danh sách trang Facebook: %v", err)
			return errors.New("Lỗi khi lấy danh sách trang Facebook")
		}

		data := resultPages["data"].(map[string]interface{})
		itemCount := data["itemCount"].(float64)

		if itemCount > 0 {
			items := data["items"].([]interface{})

			if len(items) > 0 {
				// Clear all data in global.PanCake_FbPages (với mutex để tránh race condition)
				global.PanCake_FbPagesMu.Lock()
				global.PanCake_FbPages = nil

				for _, item := range items {

					// chuyển item từ interface{} sang dạng global.FbPage
					var cloudFbPage global.FbPage
					bsonBytes, err := bson.Marshal(item)
					if err != nil {
						global.PanCake_FbPagesMu.Unlock()
						logError("Lỗi khi chuyển đổi dữ liệu trang: %v", err)
						return err
					}

					err = bson.Unmarshal(bsonBytes, &cloudFbPage)
					if err != nil {
						global.PanCake_FbPagesMu.Unlock()
						logError("Lỗi khi chuyển đổi dữ liệu trang: %v", err)
						return err
					}

					// Append cloudFbPage to global.PanCake_FbPages
					global.PanCake_FbPages = append(global.PanCake_FbPages, cloudFbPage)
				}
				global.PanCake_FbPagesMu.Unlock()
			}
			log.Println("Đồng bộ danh sách trang từ FolkForm về local thành công")
		}

	}

	return nil
}

// Hàm bridge_SyncConversationsOfPage sẽ đồng bộ danh sách hội thoại của trang Facebook từ server Pancake về server FolkForm
// - Lấy danh sách hội thoại của page từ server Pancake
// - Đẩy danh sách hội thoại vào server FolkForm
func bridge_SyncConversationsOfPage(page_id string, page_username string) (resultErr error) {

	last_conversation_id := ""
	conversationCount := 0
	batchCount := 0
	// Sync tất cả → không dùng since/until (truyền 0, 0)
	// Sử dụng adaptive rate limiter để tránh rate limit
	rateLimiter := apputility.GetPancakeRateLimiter()

	for {
		// Sử dụng rate limiter trước khi gọi API
		rateLimiter.Wait()

		batchCount++
		log.Printf("[Bridge] [Batch %d] Lấy conversations cho page_id=%s (last_conversation_id=%s)", batchCount, page_id, last_conversation_id)

		// Sử dụng unread_first=true để ưu tiên lấy conversations chưa đọc trước
		resultGetConversations, err := Pancake_GetConversations_v2(page_id, last_conversation_id, 0, 0, "", true)
		if err != nil {
			logError("Lỗi khi lấy danh sách hội thoại: %v", err)
			break
		}

		if resultGetConversations["conversations"] != nil {
			conversations := resultGetConversations["conversations"].([]interface{})
			if len(conversations) > 0 {
				log.Printf("[Bridge] [Batch %d] Lấy được %d conversations từ Pancake", batchCount, len(conversations))

				for _, conversation := range conversations {
					_, err = FolkForm_CreateConversation(page_id, page_username, conversation)
					if err != nil {
						logError("[Bridge] Lỗi khi tạo hội thoại (batch=%d): %v", batchCount, err)
						continue
					}
					conversationCount++
				}

				log.Printf("[Bridge] [Batch %d] Đã sync %d conversations vào FolkForm (tổng: %d conversations)", batchCount, len(conversations), conversationCount)

				new_last_conversation_id := conversations[len(conversations)-1].(map[string]interface{})["id"].(string)
				if new_last_conversation_id != last_conversation_id {
					last_conversation_id = new_last_conversation_id
					log.Printf("[Bridge] [Batch %d] Tiếp tục pagination (last_conversation_id=%s)", batchCount, last_conversation_id)
					continue
				} else {
					log.Printf("[Bridge] [Batch %d] Không còn conversations mới, dừng pagination (last_conversation_id không đổi)", batchCount)
					break
				}
			} else {
				log.Printf("[Bridge] [Batch %d] Không có conversations nào trong response", batchCount)
				break
			}
		} else {
			log.Printf("[Bridge] [Batch %d] Response không có field 'conversations'", batchCount)
			break
		}
	}

	log.Printf("[Bridge] ✅ Đồng bộ hội thoại cho trang %s thành công (tổng: %d conversations trong %d batches)", page_id, conversationCount, batchCount)

	return nil
}

// Hàm Bridge_SyncConversations sẽ đồng bộ danh sách hội thoại của trang Facebook từ server Pancake về server FolkForm
// - Lấy danh sách trang từ server FolkForm
// - Gọi hàm bridge_SyncConversationsOfPage để đồng bộ hội thoại của từng trang
func Bridge_SyncConversationsFromCloud() (resultErr error) {

	limit := 50
	page := 1

	for {

		// Dừng nửa giây trước khi tiếp tục
		time.Sleep(100 * time.Millisecond)

		// Lấy danh sách các pages từ server FolkForm
		resultPages, err := FolkForm_GetFbPages(page, limit)
		if err != nil {
			logError("Lỗi khi lấy danh sách trang Facebook: %v", err)
			return errors.New("Lỗi khi lấy danh sách trang Facebook")
		}

		data := resultPages["data"].(map[string]interface{})
		itemCount := data["itemCount"].(float64)

		if itemCount > 0 {
			items := data["items"].([]interface{})

			if len(items) > 0 {
				for _, item := range items {

					// Dừng nửa giây trước khi tiếp tục
					time.Sleep(100 * time.Millisecond)

					// chuyển item từ interface{} sang dạng map[string]interface{}
					page := item.(map[string]interface{})
					page_id := page["pageId"].(string)
					page_access_token := page["pageAccessToken"].(string)
					page_username := page["pageUsername"].(string)
					is_sync := page["isSync"].(bool)
					if page_access_token != "" && is_sync == true {
						// Gọi hàm bridge_SyncConversationsOfPage để đồng bộ hội thoại của từng trang
						err = bridge_SyncConversationsOfPage(page_id, page_username)
						if err != nil {
							logError("Lỗi khi đồng bộ hội thoại: %v", err)
							continue
						}
					}
				}
			}

			page++
			continue
		} else {
			break
		}
	}

	log.Println("Đồng bộ hội thoại từ server Pancake về server FolkForm thành công")

	return nil
}

// Hàm bridge_SyncMessageOfConversation sẽ đồng bộ danh sách tin nhắn của hội thoại từ server Pancake về server FolkForm
// Sử dụng pagination với current_count để lấy hết messages (không chỉ 30 đầu tiên)
// Tối ưu: Chỉ sync messages mới hơn message mới nhất đã có trong FolkForm
func bridge_SyncMessageOfConversation(page_id string, page_username string, conversation_id string, customer_id string) (resultErr error) {
	log.Printf("[Bridge] Bắt đầu sync messages cho conversation: conversation_id=%s, page_id=%s, customer_id=%s", conversation_id, page_id, customer_id)

	// Lấy message mới nhất từ FolkForm để so sánh insertedAt
	// Pancake messages được sắp xếp theo thời gian (mới nhất trước, index 0 là mới nhất)
	// So sánh insertedAt: nếu message từ Pancake cũ hơn message mới nhất trong FolkForm → dừng
	latestInsertedAt, err := FolkForm_GetLatestMessageItem(conversation_id)
	if err != nil {
		log.Printf("[Bridge] CẢNH BÁO: Không thể lấy latest message từ FolkForm, sẽ sync từ đầu - conversation_id=%s, error=%v", conversation_id, err)
		latestInsertedAt = 0 // Fallback: sync từ đầu
	}

	if latestInsertedAt > 0 {
		log.Printf("[Bridge] Đã có message mới nhất trong FolkForm với insertedAt=%d, sẽ chỉ sync messages mới hơn", latestInsertedAt)
	} else {
		log.Printf("[Bridge] Chưa có messages trong FolkForm, sẽ sync từ đầu")
	}

	// Bắt đầu từ current_count = 0 để lấy messages mới nhất
	current_count := 0

	// Sử dụng adaptive rate limiter để tránh rate limit
	rateLimiter := apputility.GetPancakeRateLimiter()

	totalMessagesSynced := 0 // Số messages đã sync trong lần này
	batchCount := 0
	const maxMessagesPerBatch = 30 // Pancake API trả về tối đa 30 messages mỗi lần

	for {
		// Sử dụng rate limiter trước khi gọi API
		rateLimiter.Wait()

		batchCount++
		log.Printf("[Bridge] [Batch %d] Lấy messages cho conversation %s (current_count=%d)", batchCount, conversation_id, current_count)

		resultGetMessages, err := Pancake_GetMessages(page_id, conversation_id, customer_id, current_count)
		if err != nil {
			logError("[Bridge] Lỗi khi lấy danh sách tin nhắn từ server Pancake (conversation_id=%s, current_count=%d, batch=%d): %v", conversation_id, current_count, batchCount, err)
			return fmt.Errorf("Lỗi khi lấy danh sách tin nhắn từ server Pancake: %v", err)
		}

		// Kiểm tra xem có messages không
		if resultGetMessages["messages"] == nil {
			log.Printf("[Bridge] Không có messages trong response cho conversation %s (batch=%d)", conversation_id, batchCount)
			break
		}

		messages := resultGetMessages["messages"].([]interface{})
		if len(messages) == 0 {
			log.Printf("[Bridge] Không còn messages nào cho conversation %s (current_count=%d, batch=%d)", conversation_id, current_count, batchCount)
			break
		}

		log.Printf("[Bridge] [Batch %d] Lấy được %d messages từ Pancake cho conversation %s (current_count=%d)", batchCount, len(messages), conversation_id, current_count)

		// So sánh insertedAt của messages với message mới nhất trong FolkForm
		// Nếu có message cũ hơn → dừng, chỉ sync messages mới hơn
		messagesToSync := []interface{}{}
		shouldStop := false

		for _, messageItem := range messages {
			messageMap, ok := messageItem.(map[string]interface{})
			if !ok {
				log.Printf("[Bridge] [Batch %d] CẢNH BÁO: Message không phải là map, bỏ qua", batchCount)
				continue
			}

			// Lấy inserted_at từ message (format: "2025-05-10T05:04:53.000000")
			var messageInsertedAt int64 = 0
			if insertedAtStr, ok := messageMap["inserted_at"].(string); ok && insertedAtStr != "" {
				// Parse ISO 8601 string sang Unix timestamp
				if t, err := time.Parse("2006-01-02T15:04:05.000000", insertedAtStr); err == nil {
					messageInsertedAt = t.Unix()
				} else {
					log.Printf("[Bridge] [Batch %d] CẢNH BÁO: Không thể parse inserted_at: %s, error: %v", batchCount, insertedAtStr, err)
				}
			}

			// Nếu đã có message mới nhất trong FolkForm
			if latestInsertedAt > 0 {
				// Nếu message này cũ hơn hoặc bằng message mới nhất → dừng
				if messageInsertedAt <= latestInsertedAt {
					log.Printf("[Bridge] [Batch %d] Gặp message cũ hơn hoặc bằng message mới nhất (insertedAt: %d <= %d), dừng sync", batchCount, messageInsertedAt, latestInsertedAt)
					shouldStop = true
					break // Dừng vòng lặp, không thêm messages cũ hơn
				}
			}

			// Chỉ thêm messages mới hơn
			messagesToSync = append(messagesToSync, messageItem)
		}

		// Nếu không có messages nào mới hơn → dừng
		if len(messagesToSync) == 0 {
			log.Printf("[Bridge] [Batch %d] Không có messages mới hơn message mới nhất trong FolkForm, dừng sync", batchCount)
			break
		}

		log.Printf("[Bridge] [Batch %d] Có %d messages mới hơn (tổng: %d messages từ Pancake), sẽ sync %d messages", batchCount, len(messagesToSync), len(messages), len(messagesToSync))

		// Tạo panCakeData chỉ với messages mới hơn
		panCakeData := map[string]interface{}{
			"messages": messagesToSync,
		}

		// Xác định hasMore: nếu lấy được đủ 30 messages và không dừng → có thể còn messages để sync
		hasMore := len(messages) >= maxMessagesPerBatch && !shouldStop

		// Gọi endpoint mới /upsert-messages với dữ liệu nguyên gốc từ Pancake
		_, err = FolkForm_UpsertMessages(page_id, page_username, conversation_id, customer_id, panCakeData, hasMore)
		if err != nil {
			logError("[Bridge] Lỗi khi upsert messages lên server FolkForm (conversation_id=%s, batch=%d): %v", conversation_id, batchCount, err)
			return fmt.Errorf("Lỗi khi upsert messages lên server FolkForm: %v", err)
		}

		totalMessagesSynced += len(messagesToSync)
		log.Printf("[Bridge] [Batch %d] Đã upsert %d messages mới vào FolkForm (đã sync: %d messages mới trong lần này, hasMore: %v)", batchCount, len(messagesToSync), totalMessagesSynced, hasMore)

		// Nếu gặp message cũ hơn → dừng
		if shouldStop {
			log.Printf("[Bridge] [Batch %d] Đã gặp message cũ hơn, dừng sync", batchCount)
			break
		}

		// Nếu lấy được ít hơn maxMessagesPerBatch → đã lấy hết messages mới
		if len(messages) < maxMessagesPerBatch {
			log.Printf("[Bridge] Đã lấy hết messages mới cho conversation %s (đã sync: %d messages mới trong %d batches)", conversation_id, totalMessagesSynced, batchCount)
			break
		}

		// Cập nhật current_count để lấy tiếp batch sau
		// current_count là vị trí index để lấy 30 messages trước đó
		current_count += len(messages)
		log.Printf("[Bridge] [Batch %d] Tiếp tục lấy messages mới cho conversation %s (current_count=%d, đã sync: %d messages mới)", batchCount, conversation_id, current_count, totalMessagesSynced)
	}

	if totalMessagesSynced > 0 {
		log.Printf("[Bridge] ✅ Đồng bộ tin nhắn cho hội thoại %s thành công (đã sync: %d messages mới trong %d batches)", conversation_id, totalMessagesSynced, batchCount)
	} else {
		if latestInsertedAt > 0 {
			log.Printf("[Bridge] ✅ Không có messages mới để sync cho conversation %s (message mới nhất có insertedAt=%d)", conversation_id, latestInsertedAt)
		} else {
			log.Printf("[Bridge] ✅ Không có messages mới để sync cho conversation %s", conversation_id)
		}
	}

	return nil
}

// Hàm Bridge_SyncMessages sẽ đồng bộ danh sách tin nhắn của trang Facebook từ server Pancake về server FolkForm
func Bridge_SyncMessages() (resultErr error) {

	limit := 50
	page := 1

	for {

		// Dừng nửa giây trước khi tiếp tục
		time.Sleep(100 * time.Millisecond)

		// Lấy danh sách các Conversations từ server FolkForm
		resultGetConversations, err := FolkForm_GetConversations(page, limit)
		if err != nil {
			logError("Lỗi khi lấy danh sách trang Facebook: %v", err)
			return errors.New("Lỗi khi lấy danh sách trang Facebook")
		}

		data := resultGetConversations["data"].(map[string]interface{})
		itemCount := data["itemCount"].(float64)

		log.Printf("[Bridge] Lấy được %d conversations từ FolkForm (page: %d)", int(itemCount), page)

		if itemCount > 0 {
			items := data["items"].([]interface{})

			if len(items) > 0 {
				log.Printf("[Bridge] Bắt đầu xử lý %d conversations", len(items))
				processedCount := 0
				skippedCount := 0

				for _, item := range items {
					// chuyển item từ interface{} sang dạng map[string]interface{}
					conversation := item.(map[string]interface{})
					pageId := ""
					pageUsername := ""
					conversationId := ""
					customerId := ""

					if pid, ok := conversation["pageId"].(string); ok {
						pageId = pid
					}
					if pusername, ok := conversation["pageUsername"].(string); ok {
						pageUsername = pusername
					}
					if cid, ok := conversation["conversationId"].(string); ok {
						conversationId = cid
					}
					if custId, ok := conversation["customerId"].(string); ok {
						customerId = custId
					}

					if pageId == "" || conversationId == "" {
						log.Printf("[Bridge] CẢNH BÁO: Conversation thiếu pageId hoặc conversationId, bỏ qua")
						skippedCount++
						continue
					}

					log.Printf("[Bridge] Xử lý conversation: conversationId=%s, pageId=%s, customerId=%s", conversationId, pageId, customerId)

					resultGetPageByPageId, err := FolkForm_GetFbPageByPageId(pageId)
					if err != nil {
						logError("[Bridge] Lỗi khi lấy trang theo pageId (%s): %v", pageId, err)
						skippedCount++
						continue
					}

					pageData := resultGetPageByPageId["data"].(map[string]interface{})
					page_access_token := ""
					if pat, ok := pageData["pageAccessToken"].(string); ok {
						page_access_token = pat
					}

					if page_access_token == "" {
						log.Printf("[Bridge] CẢNH BÁO: Page %s không có pageAccessToken, bỏ qua conversation %s", pageId, conversationId)
						skippedCount++
						continue
					}

					// Gọi hàm bridge_SyncMessageOfConversation để đồng bộ tin nhắn
					err = bridge_SyncMessageOfConversation(pageId, pageUsername, conversationId, customerId)
					if err != nil {
						logError("[Bridge] Lỗi khi đồng bộ tin nhắn (conversationId=%s): %v", conversationId, err)
						skippedCount++
						continue
					}

					processedCount++
					log.Printf("[Bridge] Đã xử lý conversation %s thành công (%d/%d)", conversationId, processedCount, len(items))
				}

				log.Printf("[Bridge] Hoàn thành xử lý: %d thành công, %d bỏ qua", processedCount, skippedCount)
			}

			page++
			continue
		} else {
			log.Printf("[Bridge] Không còn conversations nào, dừng pagination")
			break
		}
	}

	log.Println("Đồng bộ tin nhắn từ server Pancake về server FolkForm thành công")

	return nil
}

// getLastPanCakeUpdatedAt lấy panCakeUpdatedAt cuối cùng từ FolkForm cho một page
// Trả về Unix timestamp (giây), hoặc 0 nếu không tìm thấy
func getLastPanCakeUpdatedAt(page_id string) int64 {
	log.Printf("[Bridge] Lấy panCakeUpdatedAt cuối cùng từ FolkForm cho page_id: %s", page_id)

	// Lấy conversations từ FolkForm (sắp xếp theo panCakeUpdatedAt giảm dần với -1)
	// Có thể dùng limit=1 vì items[0] đã là conversation mới nhất
	resultGetConversations, err := FolkForm_GetConversationsWithPageId(1, 1, page_id)
	if err != nil {
		logError("[Bridge] Lỗi khi lấy conversations từ FolkForm: %v", err)
		return 0
	}

	// Parse response
	var items []interface{}
	if dataMap, ok := resultGetConversations["data"].(map[string]interface{}); ok {
		if itemCount, ok := dataMap["itemCount"].(float64); ok && itemCount > 0 {
			if itemsArray, ok := dataMap["items"].([]interface{}); ok {
				items = itemsArray
			}
		}
	} else if dataArray, ok := resultGetConversations["data"].([]interface{}); ok {
		items = dataArray
	}

	if len(items) == 0 {
		log.Printf("[Bridge] Không tìm thấy conversation nào trong FolkForm cho page_id: %s", page_id)
		return 0
	}

	// Lấy item đầu tiên (mới nhất) vì API sắp xếp giảm dần (panCakeUpdatedAt: -1)
	// items[0] = conversation mới nhất (panCakeUpdatedAt lớn nhất)
	firstItem := items[0]
	if conversation, ok := firstItem.(map[string]interface{}); ok {
		// panCakeUpdatedAt có thể là number (float64) hoặc int64
		if panCakeUpdatedAt, ok := conversation["panCakeUpdatedAt"].(float64); ok {
			result := int64(panCakeUpdatedAt)
			log.Printf("[Bridge] Tìm thấy panCakeUpdatedAt mới nhất: %d (Unix timestamp)", result)
			// Convert sang time để log dễ đọc
			lastUpdatedTime := time.Unix(result, 0)
			log.Printf("[Bridge] Thời gian tương ứng: %s", lastUpdatedTime.Format("2006-01-02 15:04:05"))
			return result
		} else if panCakeUpdatedAt, ok := conversation["panCakeUpdatedAt"].(int64); ok {
			log.Printf("[Bridge] Tìm thấy panCakeUpdatedAt mới nhất: %d (Unix timestamp)", panCakeUpdatedAt)
			lastUpdatedTime := time.Unix(panCakeUpdatedAt, 0)
			log.Printf("[Bridge] Thời gian tương ứng: %s", lastUpdatedTime.Format("2006-01-02 15:04:05"))
			return panCakeUpdatedAt
		} else {
			log.Printf("[Bridge] CẢNH BÁO: Không tìm thấy panCakeUpdatedAt trong conversation")
			// Debug: log toàn bộ conversation để xem structure
			log.Printf("[Bridge] Conversation structure: %+v", conversation)
		}
	}

	log.Printf("[Bridge] Không thể parse conversation từ FolkForm")
	return 0
}

// ========================================================================================================
// Hàm đồng bộ dữ liệu mới nhất từ server Pancake về server FolkForm của 1 trang Facebook
func Sync_NewMessagesOfPage(page_id string, page_username string) (resultErr error) {
	log.Printf("[Bridge] Bắt đầu sync conversations mới cho page_id: %s", page_id)

	// Bước 1: Lấy panCakeUpdatedAt cuối cùng từ FolkForm
	lastUpdatedAt := getLastPanCakeUpdatedAt(page_id)

	// Bước 2: Tính since và until
	var since int64
	until := time.Now().Unix()

	if lastUpdatedAt == 0 {
		// Trường hợp chưa có conversation nào trong FolkForm
		// Giới hạn sync về 30 ngày gần nhất để tránh lấy quá nhiều dữ liệu cũ
		// Có thể thay đổi số ngày này nếu cần sync xa hơn
		maxDaysBack := int64(30) // Sync tối đa 30 ngày gần nhất
		since = until - (maxDaysBack * 24 * 60 * 60)

		sinceTime := time.Unix(since, 0)
		untilTime := time.Unix(until, 0)
		log.Printf("[Bridge] Không tìm thấy conversation nào trong FolkForm")
		log.Printf("[Bridge] Sẽ sync conversations từ %s đến %s (30 ngày gần nhất)",
			sinceTime.Format("2006-01-02 15:04:05"),
			untilTime.Format("2006-01-02 15:04:05"))
		log.Printf("[Bridge] Lưu ý: Nếu cần sync toàn bộ lịch sử, hãy dùng hàm bridge_SyncConversationsOfPage()")
	} else {
		// Trường hợp đã có conversations → sync incremental từ lastUpdatedAt
		since = lastUpdatedAt
		log.Printf("[Bridge] Tìm thấy panCakeUpdatedAt cuối cùng: %d (Unix timestamp)", lastUpdatedAt)
		// Convert sang time để log dễ đọc
		lastUpdatedTime := time.Unix(lastUpdatedAt, 0)
		untilTime := time.Unix(until, 0)
		log.Printf("[Bridge] Thời gian tương ứng: %s", lastUpdatedTime.Format("2006-01-02 15:04:05"))
		log.Printf("[Bridge] Sẽ sync conversations từ %s đến %s",
			lastUpdatedTime.Format("2006-01-02 15:04:05"),
			untilTime.Format("2006-01-02 15:04:05"))
	}

	// Edge case: since >= until (không nên xảy ra nhưng kiểm tra để an toàn)
	if since >= until {
		log.Printf("[Bridge] since (%d) >= until (%d), không có conversations mới", since, until)
		return nil
	}

	// Log thông tin khoảng thời gian sync
	timeWindow := until - since
	days := timeWindow / (24 * 60 * 60)
	hours := (timeWindow % (24 * 60 * 60)) / (60 * 60)
	log.Printf("[Bridge] Sync conversations từ timestamp %d đến %d", since, until)
	log.Printf("[Bridge] Khoảng thời gian: %d ngày %d giờ (%d giây)", days, hours, timeWindow)

	// Bước 3: Sync conversations trong khoảng thời gian
	// Sử dụng adaptive rate limiter để tránh rate limit
	rateLimiter := apputility.GetPancakeRateLimiter()

	last_conversation_id := ""
	conversationCount := 0
	batchCount := 0

	for {
		// Sử dụng rate limiter trước khi gọi API
		rateLimiter.Wait()

		batchCount++
		log.Printf("[Bridge] [Batch %d] Lấy conversations cho page_id=%s (last_conversation_id=%s)", batchCount, page_id, last_conversation_id)

		// Gọi API với since/until, sử dụng unread_first=true để ưu tiên lấy conversations chưa đọc trước
		resultGetConversations, err := Pancake_GetConversations_v2(page_id, last_conversation_id, since, until, "", true)
		if err != nil {
			logError("[Bridge] Lỗi khi lấy danh sách hội thoại: %v", err)
			break
		}

		if resultGetConversations["conversations"] != nil {
			conversations := resultGetConversations["conversations"].([]interface{})
			if len(conversations) == 0 {
				log.Printf("[Bridge] Không còn conversations nào trong khoảng thời gian")
				break
			}

			log.Printf("[Bridge] [Batch %d] Lấy được %d conversations từ Pancake", batchCount, len(conversations))

			// Xử lý từng conversation
			for _, conversation := range conversations {
				conversationMap := conversation.(map[string]interface{})
				conversation_id := conversationMap["id"].(string)
				customerId := ""
				if cid, ok := conversationMap["customer_id"].(string); ok {
					customerId = cid
				}

				// Tạo/update conversation trong FolkForm
				_, err = FolkForm_CreateConversation(page_id, page_username, conversation)
				if err != nil {
					logError("[Bridge] Lỗi khi tạo/cập nhật hội thoại: %v", err)
					continue
				}

				conversationCount++

				// Sync messages của conversation này
				err = bridge_SyncMessageOfConversation(page_id, page_username, conversation_id, customerId)
				if err != nil {
					logError("[Bridge] Lỗi khi đồng bộ tin nhắn: %v", err)
					continue
				}

				// Sử dụng adaptive rate limiter để nghỉ trước khi tiếp tục (có thể gọi Pancake API tiếp theo)
				rateLimiter := apputility.GetPancakeRateLimiter()
				rateLimiter.Wait()
			}

			// Cập nhật last_conversation_id để pagination
			new_last_conversation_id := conversations[len(conversations)-1].(map[string]interface{})["id"].(string)
			if new_last_conversation_id != last_conversation_id {
				last_conversation_id = new_last_conversation_id
				log.Printf("[Bridge] [Batch %d] Tiếp tục pagination (last_conversation_id=%s, đã sync: %d conversations)", batchCount, last_conversation_id, conversationCount)
				continue
			} else {
				log.Printf("[Bridge] [Batch %d] Không còn conversations mới, dừng pagination (last_conversation_id không đổi)", batchCount)
				break
			}
		} else {
			log.Printf("[Bridge] Không có conversations nào trong response")
			break
		}
	}

	log.Printf("[Bridge] ✅ Đồng bộ conversations mới thành công cho page_id: %s, tổng cộng: %d conversations trong %d batches",
		page_id, conversationCount, batchCount)

	return nil
}

// Hàm Sync_NewMessages sẽ đồng bộ dữ liệu mới nhất từ server Pancake về server FolkForm
func Sync_NewMessagesOfAllPages() (resultErr error) {

	limit := 50
	page := 1

	for {

		// Dừng nửa giây trước khi tiếp tục
		time.Sleep(100 * time.Millisecond)

		// Lấy danh sách các pages từ server FolkForm
		resultPages, err := FolkForm_GetFbPages(page, limit)
		if err != nil {
			logError("Lỗi khi lấy danh sách trang Facebook: %v", err)
			return errors.New("Lỗi khi lấy danh sách trang Facebook")
		}

		// Xử lý response - có thể là pagination object hoặc array trực tiếp
		items, itemCount, err := parseResponseData(resultPages)
		if err != nil {
			logError("[Sync_NewMessagesOfAllPages] LỖI khi parse response: %v", err)
			return err
		}
		log.Printf("[Sync_NewMessagesOfAllPages] Nhận được %d pages (page=%d, limit=%d)", len(items), page, limit)

		if itemCount > 0 && len(items) > 0 {
			for _, item := range items {

				// chuyển item từ interface{} sang dạng map[string]interface{}
				itemMap, ok := item.(map[string]interface{})
				if !ok {
					logError("[Sync_NewMessagesOfAllPages] LỖI: Item không phải là map: %T", item)
					continue
				}

				// Lấy pageId
				page_id, ok := itemMap["pageId"].(string)
				if !ok {
					logError("[Sync_NewMessagesOfAllPages] LỖI: Không tìm thấy field 'pageId' trong item hoặc không phải string")
					continue
				}

				// Lấy pageUsername
				page_username, ok := itemMap["pageUsername"].(string)
				if !ok {
					logError("[Sync_NewMessagesOfAllPages] LỖI: Không tìm thấy field 'pageUsername' trong item hoặc không phải string")
					continue
				}

				// Lấy isSync
				is_sync, ok := itemMap["isSync"].(bool)
				if !ok {
					logError("[Sync_NewMessagesOfAllPages] LỖI: Không tìm thấy field 'isSync' trong item hoặc không phải bool")
					continue
				}

				if is_sync == true {
					// Gọi hàm Sync_NewMessagesOfPage để đồng bộ tin nhắn của từng trang
					err = Sync_NewMessagesOfPage(page_id, page_username)
					if err != nil {
						logError("Lỗi khi đồng bộ tin nhắn: %v", err)
						continue
					}
				}
			}

			page++
			continue
		} else {
			log.Println("[Sync_NewMessagesOfAllPages] Không còn pages nào. Kết thúc.")
			break
		}
	}

	log.Println("Đồng bộ tin nhắn mới nhất từ server Pancake về server FolkForm thành công")

	return nil
}
