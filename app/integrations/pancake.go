/*
Package integrations chứa các hàm tích hợp với các hệ thống bên ngoài.
File pancake.go chứa các hàm gọi API từ Pancake để lấy dữ liệu:
- Pages (trang Facebook)
- Conversations (hội thoại)
- Messages (tin nhắn)
- Posts (bài đăng)
- Customers (khách hàng)
Tất cả các hàm đều có retry logic và sử dụng adaptive rate limiter để tránh rate limit.
*/
package integrations

import (
	apputility "agent_pancake/app/utility"
	"agent_pancake/global"
	"agent_pancake/utility/httpclient"
	"encoding/json"
	"errors"
	"io"
	"log"
	"strconv"
	"time"
)

// PanCake_GetFbPages lấy danh sách pages từ server Pancake
// Tham số:
//   - access_token: Access token của user để truy cập Pancake API
//
// Trả về:
//   - result: Map chứa danh sách pages với format: {"success": true, "data": {"categorized": {"activated": [...]}}}
//   - err: Lỗi nếu có (sau khi đã retry tối đa 5 lần)
func PanCake_GetFbPages(access_token string) (result map[string]interface{}, err error) {
	log.Printf("[Pancake] Bắt đầu lấy danh sách pages từ Pancake")
	log.Printf("[Pancake] Pancake Base URL: %s", global.GlobalConfig.PancakeBaseUrl)

	// Khởi tạo client
	client := httpclient.NewHttpClient(global.GlobalConfig.PancakeBaseUrl, 60*time.Second)

	// Thiết lập header
	params := map[string]string{
		"access_token": access_token,
	}

	// Số lần thử request
	requestCount := 0
	for {
		requestCount++
		log.Printf("[Pancake] [Lần thử %d/5] Bắt đầu lấy danh sách pages", requestCount)

		// Nếu số lần thử vượt quá 5 lần thì thoát vòng lặp
		if requestCount > 5 {
			logError("[Pancake] LỖI: Đã thử quá nhiều lần (%d/5). Thoát vòng lặp.", requestCount)
			return nil, errors.New("Đã thử quá nhiều lần. Thoát vòng lặp.")
		}

		// Sử dụng adaptive rate limiter để nghỉ trước khi gửi request
		rateLimiter := apputility.GetPancakeRateLimiter()
		rateLimiter.Wait()

		log.Printf("[Pancake] [Lần thử %d/5] Gửi GET request đến endpoint: /v1/pages", requestCount)
		log.Printf("[Pancake] [Lần thử %d/5] Request params: access_token (length: %d)", requestCount, len(access_token))

		// Gửi yêu cầu GET
		resp, err := client.GET("/v1/pages", params)
		if err != nil {
			logError("[Pancake] [Lần thử %d/5] ❌ LỖI khi gọi API GET: %v", requestCount, err)
			log.Printf("[Pancake] [Lần thử %d/5] Request endpoint: /v1/pages", requestCount)
			log.Printf("[Pancake] [Lần thử %d/5] 📝 Chi tiết lỗi: %s", requestCount, err.Error())
			continue
		}

		statusCode := resp.StatusCode
		log.Printf("[Pancake] [Lần thử %d/5] Response Status Code: %d", requestCount, statusCode)

		// Kiểm tra mã trạng thái, nếu không phải 200 thì thử lại
		if statusCode != 200 {
			// Đọc response body để log lỗi
			bodyBytes, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			var errorCode interface{}
			if readErr == nil {
				logError("[Pancake] [Lần thử %d/5] ❌ LỖI: Response Body (raw): %s", requestCount, string(bodyBytes))
				var errorResult map[string]interface{}
				if err := json.Unmarshal(bodyBytes, &errorResult); err == nil {
					logError("[Pancake] [Lần thử %d/5] ❌ LỖI: Response Body (parsed): %+v", requestCount, errorResult)
					// In message lỗi nếu có
					if message, ok := errorResult["message"].(string); ok {
						log.Printf("[Pancake] [Lần thử %d/5] 📝 Message lỗi từ Pancake: %s", requestCount, message)
					}
					if ec, ok := errorResult["error_code"]; ok {
						errorCode = ec
						log.Printf("[Pancake] [Lần thử %d/5] 🔢 Error Code: %v", requestCount, errorCode)
					}
				}
			} else {
				log.Printf("[Pancake] [Lần thử %d/5] ❌ Không thể đọc response body: %v", requestCount, readErr)
			}
			// Ghi nhận lỗi để điều chỉnh rate limiter
			rateLimiter.RecordFailure(statusCode, errorCode)
			log.Printf("[Pancake] [Lần thử %d/5] ⚠️ Status Code: %d - Lấy danh sách trang Facebook thất bại. Thử lại", requestCount, statusCode)
			continue
		}

		// Đọc dữ liệu từ phản hồi
		// Đọc body trước để có thể log khi parse lỗi
		bodyBytes, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			log.Printf("[Pancake] [Lần thử %d/5] ❌ Không thể đọc response body: %v", requestCount, readErr)
			continue
		}

		var result map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &result); err != nil {
			logError("[Pancake] [Lần thử %d/5] ❌ LỖI khi phân tích phản hồi JSON: %v", requestCount, err)
			log.Printf("[Pancake] [Lần thử %d/5] 📝 Response Body (raw): %s", requestCount, string(bodyBytes))
			continue
		}

		// Ghi nhận kết quả response để điều chỉnh rate limiter
		var errorCode interface{}
		if ec, ok := result["error_code"]; ok {
			errorCode = ec
		}
		success := result["success"] == true
		rateLimiter.RecordResponse(statusCode, success, errorCode)

		if result["success"] == true {
			log.Printf("[Pancake] Lấy danh sách pages thành công")
			return result, nil
		}

		log.Printf("[Pancake] [Lần thử %d/5] ❌ Response success không phải true: %v", requestCount, result["success"])
		if message, ok := result["message"].(string); ok {
			log.Printf("[Pancake] [Lần thử %d/5] 📝 Message lỗi từ Pancake: %s", requestCount, message)
		}
		if errorCode, ok := result["error_code"]; ok {
			log.Printf("[Pancake] [Lần thử %d/5] 🔢 Error Code: %v", requestCount, errorCode)
		}
		log.Printf("[Pancake] [Lần thử %d/5] Response Body: %+v", requestCount, result)

		// Nếu số lần thử vượt quá 5 lần thì thoát vòng lặp
		if requestCount > 5 {
			return result, errors.New("Đã thử quá nhiều lần. Thoát vòng lặp.")
		}
	}
}

// PanCake_GeneratePageAccessToken tạo page_access_token từ server Pancake
// Hàm này gọi Pancake API để generate page_access_token mới cho một page
// Tham số:
//   - page_id: ID của page cần generate token
//   - access_token: Access token của user để truy cập Pancake API
//
// Trả về:
//   - result: Map chứa page_access_token với format: {"success": true, "page_access_token": "..."}
//   - err: Lỗi nếu có (sau khi đã retry tối đa 5 lần)
func PanCake_GeneratePageAccessToken(page_id string, access_token string) (result map[string]interface{}, err error) {
	log.Printf("[Pancake] Bắt đầu tạo page_access_token - page_id: %s", page_id)
	log.Printf("[Pancake] Pancake Base URL: %s", global.GlobalConfig.PancakeBaseUrl)

	// Khởi tạo client
	client := httpclient.NewHttpClient(global.GlobalConfig.PancakeBaseUrl, 10*time.Second)

	// Chuẩn bị dữ liệu cần gửi
	params := map[string]string{
		"access_token": access_token,
	}

	// Số lần thử request
	requestCount := 0
	for {
		requestCount++
		log.Printf("[Pancake] [Lần thử %d/5] Bắt đầu tạo page_access_token", requestCount)

		// Nếu số lần thử vượt quá 5 lần thì thoát vòng lặp
		if requestCount > 5 {
			logError("[Pancake] LỖI: Đã thử quá nhiều lần (%d/5). Thoát vòng lặp.", requestCount)
			return nil, errors.New("Đã thử quá nhiều lần. Thoát vòng lặp.")
		}

		// Sử dụng adaptive rate limiter để nghỉ trước khi gửi request
		rateLimiter := apputility.GetPancakeRateLimiter()
		rateLimiter.Wait()

		endpoint := "/v1/pages/" + page_id + "/generate_page_access_token"
		log.Printf("[Pancake] [Lần thử %d/5] Gửi POST request đến endpoint: %s", requestCount, endpoint)
		log.Printf("[Pancake] [Lần thử %d/5] Request params: access_token (length: %d)", requestCount, len(access_token))

		// Gửi yêu cầu POST
		resp, err := client.POST(endpoint, nil, params)
		if err != nil {
			logError("[Pancake] [Lần thử %d/5] ❌ LỖI khi gọi API POST: %v", requestCount, err)
			log.Printf("[Pancake] [Lần thử %d/5] Request endpoint: %s", requestCount, endpoint)
			log.Printf("[Pancake] [Lần thử %d/5] 📝 Chi tiết lỗi: %s", requestCount, err.Error())
			continue
		}

		statusCode := resp.StatusCode
		log.Printf("[Pancake] [Lần thử %d/5] Response Status Code: %d", requestCount, statusCode)

		// Kiểm tra mã trạng thái, nếu không phải 200 thì thử lại
		if statusCode != 200 {
			// Đọc response body để log lỗi
			bodyBytes, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			var errorCode interface{}
			if readErr == nil {
				logError("[Pancake] [Lần thử %d/5] ❌ LỖI: Response Body (raw): %s", requestCount, string(bodyBytes))
				var errorResult map[string]interface{}
				if err := json.Unmarshal(bodyBytes, &errorResult); err == nil {
					logError("[Pancake] [Lần thử %d/5] ❌ LỖI: Response Body (parsed): %+v", requestCount, errorResult)
					// In message lỗi nếu có
					if message, ok := errorResult["message"].(string); ok {
						log.Printf("[Pancake] [Lần thử %d/5] 📝 Message lỗi từ Pancake: %s", requestCount, message)
					}
					if ec, ok := errorResult["error_code"]; ok {
						errorCode = ec
						log.Printf("[Pancake] [Lần thử %d/5] 🔢 Error Code: %v", requestCount, errorCode)
					}
				}
			} else {
				log.Printf("[Pancake] [Lần thử %d/5] ❌ Không thể đọc response body: %v", requestCount, readErr)
			}
			// Ghi nhận lỗi để điều chỉnh rate limiter
			rateLimiter.RecordFailure(statusCode, errorCode)
			log.Printf("[Pancake] [Lần thử %d/5] ⚠️ Status Code: %d - Lấy page_access_token thất bại. Thử lại", requestCount, statusCode)
			continue
		}

		// Đọc dữ liệu từ phản hồi
		// Đọc body trước để có thể log khi parse lỗi
		bodyBytes, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			log.Printf("[Pancake] [Lần thử %d/5] ❌ Không thể đọc response body: %v", requestCount, readErr)
			continue
		}

		var result map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &result); err != nil {
			logError("[Pancake] [Lần thử %d/5] ❌ LỖI khi phân tích phản hồi JSON: %v", requestCount, err)
			log.Printf("[Pancake] [Lần thử %d/5] 📝 Response Body (raw): %s", requestCount, string(bodyBytes))
			continue
		}

		// Ghi nhận kết quả response để điều chỉnh rate limiter
		var errorCode interface{}
		if ec, ok := result["error_code"]; ok {
			errorCode = ec
		}
		success := result["success"] == true
		rateLimiter.RecordResponse(statusCode, success, errorCode)

		if result["success"] == true {
			log.Printf("[Pancake] Tạo page_access_token thành công - page_id: %s", page_id)
			return result, nil
		} else {
			// Nếu lỗi 105 thì cập nhật lại page_access_token
			errCode, _ := result["error_code"].(float64)
			if errCode == 103 { // 103: access_token hết hạn, cần báo cho user cập nhật lại access_token
				log.Printf("[Pancake] [Lần thử %d/5] ⚠️ Lỗi 103: access_token hết hạn", requestCount)
			}
			if message, ok := result["message"].(string); ok {
				log.Printf("[Pancake] [Lần thử %d/5] ❌ Lấy page_access_token thất bại: %s", requestCount, message)
			} else {
				log.Printf("[Pancake] [Lần thử %d/5] ❌ Lấy page_access_token thất bại: %v", requestCount, result["message"])
			}
			if errorCode, ok := result["error_code"]; ok {
				log.Printf("[Pancake] [Lần thử %d/5] 🔢 Error Code: %v", requestCount, errorCode)
			}
			log.Printf("[Pancake] [Lần thử %d/5] Response Body: %+v", requestCount, result)
		}

		// Nếu số lần thử vượt quá 5 lần thì thoát vòng lặp
		if requestCount > 5 {
			return result, errors.New("Đã thử quá nhiều lần. Thoát vòng lặp.")
		}
	}
}

// Hàm Pancake_GetConversations_v2 lấy danh sách Conversations từ server Pancake
// since và until là Unix timestamp (giây), nếu <= 0 thì không thêm param (optional)
// unread_first: nếu true, ưu tiên lấy các conversations chưa đọc trước
func Pancake_GetConversations_v2(page_id string, last_conversation_id string, since int64, until int64, order_by string, unread_first bool) (result map[string]interface{}, err error) {
	log.Printf("[Pancake] Bắt đầu lấy danh sách conversations - page_id: %s, last_conversation_id: %s, since: %d, until: %d, order_by: %s, unread_first: %v", page_id, last_conversation_id, since, until, order_by, unread_first)
	log.Printf("[Pancake] Pancake Base URL: %s", global.GlobalConfig.PancakeBaseUrl)

	// Khởi tạo client
	client := httpclient.NewHttpClient(global.GlobalConfig.PancakeBaseUrl, 60*time.Second)

	// Số lần thử request
	requestCount := 0
	for {
		requestCount++
		log.Printf("[Pancake] [Lần thử %d/5] Bắt đầu lấy danh sách conversations", requestCount)

		// Nếu số lần thử vượt quá 5 lần thì thoát vòng lặp
		if requestCount > 5 {
			logError("[Pancake] LỖI: Đã thử quá nhiều lần (%d/5). Thoát vòng lặp.", requestCount)
			return nil, errors.New("Đã thử quá nhiều lần. Thoát vòng lặp.")
		}

	Start:

		// Lấy page_access_token
		log.Printf("[Pancake] [Lần thử %d/5] Lấy page_access_token từ local...", requestCount)
		page_access_token, err := Local_GetPageAccessToken(page_id)
		if err != nil {
			logError("[Pancake] [Lần thử %d/5] LỖI khi lấy page_access_token: %v", requestCount, err)
			return nil, err
		}
		if page_access_token == "" {
			log.Printf("[Pancake] [Lần thử %d/5] Không tìm thấy page_access_token trong biến local. Đang cập nhật...", requestCount)
			Local_UpdatePagesAccessToken(page_id)
			goto Start
		}
		log.Printf("[Pancake] [Lần thử %d/5] Đã lấy được page_access_token (length: %d)", requestCount, len(page_access_token))

		// Thiết lập params
		params := map[string]string{
			"page_access_token":    page_access_token,
			"last_conversation_id": last_conversation_id,
		}

		// Thêm since/until nếu có
		if since > 0 {
			params["since"] = strconv.FormatInt(since, 10)
			log.Printf("[Pancake] [Lần thử %d/5] Thêm param since: %d", requestCount, since)
		}
		if until > 0 {
			params["until"] = strconv.FormatInt(until, 10)
			log.Printf("[Pancake] [Lần thử %d/5] Thêm param until: %d", requestCount, until)
		}
		// Thêm order_by nếu có
		if order_by != "" {
			params["order_by"] = order_by
			log.Printf("[Pancake] [Lần thử %d/5] Thêm param order_by: %s", requestCount, order_by)
		}
		// Thêm unread_first nếu true
		if unread_first {
			params["unread_first"] = "true"
			log.Printf("[Pancake] [Lần thử %d/5] Thêm param unread_first: true", requestCount)
		}

		// Nếu số lần thử vượt quá 5 lần thì thoát vòng lặp
		if requestCount > 5 {
			return nil, errors.New("Đã thử quá nhiều lần. Thoát vòng lặp.")
		}

		// Sử dụng adaptive rate limiter để nghỉ trước khi gửi request
		rateLimiter := apputility.GetPancakeRateLimiter()
		rateLimiter.Wait()

		endpoint := "/public_api/v2/pages/" + page_id + "/conversations"
		log.Printf("[Pancake] [Lần thử %d/5] Gửi GET request đến endpoint: %s", requestCount, endpoint)
		log.Printf("[Pancake] [Lần thử %d/5] Request params: page_access_token (length: %d), last_conversation_id: %s", requestCount, len(page_access_token), last_conversation_id)

		// Gửi yêu cầu GET
		resp, err := client.GET(endpoint, params)
		if err != nil {
			logError("[Pancake] [Lần thử %d/5] ❌ LỖI khi gọi API GET: %v", requestCount, err)
			log.Printf("[Pancake] [Lần thử %d/5] Request endpoint: %s", requestCount, endpoint)
			log.Printf("[Pancake] [Lần thử %d/5] 📝 Chi tiết lỗi: %s", requestCount, err.Error())
			continue
		}

		statusCode := resp.StatusCode
		log.Printf("[Pancake] [Lần thử %d/5] Response Status Code: %d", requestCount, statusCode)

		// Kiểm tra mã trạng thái, nếu không phải 200 thì thử lại
		if statusCode != 200 {
			// Đọc response body để log lỗi
			bodyBytes, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			var errorCode interface{}
			if readErr == nil {
				logError("[Pancake] [Lần thử %d/5] ❌ LỖI: Response Body (raw): %s", requestCount, string(bodyBytes))
				var errorResult map[string]interface{}
				if err := json.Unmarshal(bodyBytes, &errorResult); err == nil {
					logError("[Pancake] [Lần thử %d/5] ❌ LỖI: Response Body (parsed): %+v", requestCount, errorResult)
					// In message lỗi nếu có
					if message, ok := errorResult["message"].(string); ok {
						log.Printf("[Pancake] [Lần thử %d/5] 📝 Message lỗi từ Pancake: %s", requestCount, message)
					}
					if ec, ok := errorResult["error_code"]; ok {
						errorCode = ec
						log.Printf("[Pancake] [Lần thử %d/5] 🔢 Error Code: %v", requestCount, errorCode)
					}
				}
			} else {
				log.Printf("[Pancake] [Lần thử %d/5] ❌ Không thể đọc response body: %v", requestCount, readErr)
			}
			// Ghi nhận lỗi để điều chỉnh rate limiter
			rateLimiter.RecordFailure(statusCode, errorCode)
			log.Printf("[Pancake] [Lần thử %d/5] ⚠️ Status Code: %d - Lấy danh sách cuộc trò chuyện thất bại. Thử lại", requestCount, statusCode)
			continue
		}

		// Đọc dữ liệu từ phản hồi
		// Đọc body trước để có thể log khi parse lỗi
		bodyBytes, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			log.Printf("[Pancake] [Lần thử %d/5] ❌ Không thể đọc response body: %v", requestCount, readErr)
			continue
		}

		var result map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &result); err != nil {
			logError("[Pancake] [Lần thử %d/5] ❌ LỖI khi phân tích phản hồi JSON: %v", requestCount, err)
			log.Printf("[Pancake] [Lần thử %d/5] 📝 Response Body (raw): %s", requestCount, string(bodyBytes))
			continue
		}

		// Ghi nhận kết quả response để điều chỉnh rate limiter
		var errorCode interface{}
		if ec, ok := result["error_code"]; ok {
			errorCode = ec
		}
		success := result["success"] == true
		rateLimiter.RecordResponse(statusCode, success, errorCode)

		if result["success"] == true {
			log.Printf("[Pancake] Lấy danh sách conversations thành công - page_id: %s", page_id)
			return result, nil
		} else {
			// Nếu lỗi 105 thì cập nhật lại page_access_token
			errCode, _ := result["error_code"].(float64)
			if errCode == 105 || errCode == 102 { // 105: page_access_token hết hạn, cần cập nhật lại page_access_token
				log.Printf("[Pancake] [Lần thử %d/5] Lỗi %v: page_access_token hết hạn. Đang cập nhật...", requestCount, errCode)
				err = Local_UpdatePagesAccessToken(page_id)
				if err != nil {
					log.Printf("[Pancake] [Lần thử %d/5] LỖI khi cập nhật page_access_token: %v", requestCount, err)
				} else {
					log.Printf("[Pancake] [Lần thử %d/5] Đã cập nhật page_access_token thành công", requestCount)
				}
				goto Start
			}

			if message, ok := result["message"].(string); ok {
				log.Printf("[Pancake] [Lần thử %d/5] ❌ Lấy danh sách cuộc trò chuyện thất bại: %s", requestCount, message)
			} else {
				log.Printf("[Pancake] [Lần thử %d/5] ❌ Lấy danh sách cuộc trò chuyện thất bại: message = %v", requestCount, result["message"])
			}
			if errorCode, ok := result["error_code"]; ok {
				log.Printf("[Pancake] [Lần thử %d/5] 🔢 Error Code: %v", requestCount, errorCode)
			}
			log.Printf("[Pancake] [Lần thử %d/5] Response Body: %+v", requestCount, result)
		}

		// Nếu số lần thử vượt quá 5 lần thì thoát vòng lặp
		if requestCount > 5 {
			return result, errors.New("Đã thử quá nhiều lần. Thoát vòng lặp.")
		}

	}
}

// Hàm Pancake_GetMessages lấy danh sách Messages từ server Pancake
// current_count là vị trí index để lấy 30 tin nhắn trước đó (pagination)
// Nếu current_count = 0, lấy 30 messages mới nhất
func Pancake_GetMessages(page_id string, conversation_id string, customer_id string, current_count int) (result map[string]interface{}, err error) {
	log.Printf("[Pancake] Bắt đầu lấy danh sách messages - page_id: %s, conversation_id: %s, customer_id: %s, current_count: %d", page_id, conversation_id, customer_id, current_count)
	log.Printf("[Pancake] Pancake Base URL: %s", global.GlobalConfig.PancakeBaseUrl)

	// Khởi tạo client
	client := httpclient.NewHttpClient(global.GlobalConfig.PancakeBaseUrl, 60*time.Second)

	// Số lần thử request
	requestCount := 0
	for {
		requestCount++
		log.Printf("[Pancake] [Lần thử %d/5] Bắt đầu lấy danh sách messages", requestCount)

		// Nếu số lần thử vượt quá 5 lần thì thoát vòng lặp
		if requestCount > 5 {
			logError("[Pancake] LỖI: Đã thử quá nhiều lần (%d/5). Thoát vòng lặp.", requestCount)
			return nil, errors.New("Đã thử quá nhiều lần. Thoát vòng lặp.")
		}

	Start:

		// Sử dụng adaptive rate limiter để nghỉ trước khi gửi request
		rateLimiter := apputility.GetPancakeRateLimiter()
		rateLimiter.Wait()

		log.Printf("[Pancake] [Lần thử %d/5] Lấy page_access_token từ local...", requestCount)
		page_access_token, err := Local_GetPageAccessToken(page_id)
		if err != nil {
			logError("[Pancake] [Lần thử %d/5] LỖI khi lấy page_access_token: %v", requestCount, err)
			return nil, err
		}
		if page_access_token == "" {
			log.Printf("[Pancake] [Lần thử %d/5] Không tìm thấy page_access_token trong biến local. Đang cập nhật...", requestCount)
			Local_UpdatePagesAccessToken(page_id)
			goto Start
		}
		log.Printf("[Pancake] [Lần thử %d/5] Đã lấy được page_access_token (length: %d)", requestCount, len(page_access_token))

		// Thiết lập params
		params := map[string]string{
			"page_access_token": page_access_token,
			"customer_id":       customer_id,
		}

		// Thêm current_count nếu > 0 (pagination)
		if current_count > 0 {
			params["current_count"] = strconv.Itoa(current_count)
			log.Printf("[Pancake] [Lần thử %d/5] Thêm param current_count: %d", requestCount, current_count)
		}

		endpoint := "/public_api/v1/pages/" + page_id + "/conversations/" + conversation_id + "/messages"
		log.Printf("[Pancake] [Lần thử %d/5] Gửi GET request đến endpoint: %s", requestCount, endpoint)
		log.Printf("[Pancake] [Lần thử %d/5] Request params: page_access_token (length: %d), customer_id: %s", requestCount, len(page_access_token), customer_id)

		// Gửi yêu cầu GET
		resp, err := client.GET(endpoint, params)
		if err != nil {
			logError("[Pancake] [Lần thử %d/5] ❌ LỖI khi gọi API GET: %v", requestCount, err)
			log.Printf("[Pancake] [Lần thử %d/5] Request endpoint: %s", requestCount, endpoint)
			log.Printf("[Pancake] [Lần thử %d/5] 📝 Chi tiết lỗi: %s", requestCount, err.Error())
			continue
		}

		statusCode := resp.StatusCode
		log.Printf("[Pancake] [Lần thử %d/5] Response Status Code: %d", requestCount, statusCode)

		// Kiểm tra mã trạng thái, nếu không phải 200 thì thử lại
		if statusCode != 200 {
			// Đọc response body để log lỗi
			bodyBytes, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			var errorCode interface{}
			if readErr == nil {
				logError("[Pancake] [Lần thử %d/5] ❌ LỖI: Response Body (raw): %s", requestCount, string(bodyBytes))
				var errorResult map[string]interface{}
				if err := json.Unmarshal(bodyBytes, &errorResult); err == nil {
					logError("[Pancake] [Lần thử %d/5] ❌ LỖI: Response Body (parsed): %+v", requestCount, errorResult)
					// In message lỗi nếu có
					if message, ok := errorResult["message"].(string); ok {
						log.Printf("[Pancake] [Lần thử %d/5] 📝 Message lỗi từ Pancake: %s", requestCount, message)
					}
					if ec, ok := errorResult["error_code"]; ok {
						errorCode = ec
						log.Printf("[Pancake] [Lần thử %d/5] 🔢 Error Code: %v", requestCount, errorCode)
					}
				}
			} else {
				log.Printf("[Pancake] [Lần thử %d/5] ❌ Không thể đọc response body: %v", requestCount, readErr)
			}
			// Ghi nhận lỗi để điều chỉnh rate limiter
			rateLimiter.RecordFailure(statusCode, errorCode)
			log.Printf("[Pancake] [Lần thử %d/5] ⚠️ Status Code: %d - Lấy danh sách tin nhắn thất bại. Thử lại", requestCount, statusCode)
			continue
		}

		// Đọc dữ liệu từ phản hồi
		// Đọc body trước để có thể log khi parse lỗi
		bodyBytes, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			log.Printf("[Pancake] [Lần thử %d/5] ❌ Không thể đọc response body: %v", requestCount, readErr)
			continue
		}

		var result map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &result); err != nil {
			logError("[Pancake] [Lần thử %d/5] ❌ LỖI khi phân tích phản hồi JSON: %v", requestCount, err)
			log.Printf("[Pancake] [Lần thử %d/5] 📝 Response Body (raw): %s", requestCount, string(bodyBytes))
			continue
		}

		// Ghi nhận kết quả response để điều chỉnh rate limiter
		var errorCode interface{}
		if ec, ok := result["error_code"]; ok {
			errorCode = ec
		}
		success := result["success"] == true
		rateLimiter.RecordResponse(statusCode, success, errorCode)

		if result["success"] == true {
			log.Printf("[Pancake] Lấy danh sách messages thành công - page_id: %s, conversation_id: %s", page_id, conversation_id)
			return result, nil
		} else {
			errCode, _ := result["error_code"].(float64)
			if errCode == 105 {
				log.Printf("[Pancake] [Lần thử %d/5] Lỗi 105: page_access_token hết hạn. Đang cập nhật...", requestCount)
				err = Local_UpdatePagesAccessToken(page_id)
				if err != nil {
					log.Printf("[Pancake] [Lần thử %d/5] LỖI khi cập nhật page_access_token: %v", requestCount, err)
				} else {
					log.Printf("[Pancake] [Lần thử %d/5] Đã cập nhật page_access_token thành công", requestCount)
				}
				goto Start
			}
			if message, ok := result["message"].(string); ok {
				log.Printf("[Pancake] [Lần thử %d/5] ❌ Lấy danh sách tin nhắn thất bại: %s", requestCount, message)
			} else {
				log.Printf("[Pancake] [Lần thử %d/5] ❌ Lấy danh sách tin nhắn thất bại: %v", requestCount, result["message"])
			}
			if errorCode, ok := result["error_code"]; ok {
				log.Printf("[Pancake] [Lần thử %d/5] 🔢 Error Code: %v", requestCount, errorCode)
			}
			log.Printf("[Pancake] [Lần thử %d/5] Response Body: %+v", requestCount, result)
		}

		// Nếu số lần thử vượt quá 5 lần thì thoát vòng lặp
		if requestCount > 5 {
			return result, errors.New("Đã thử quá nhiều lần. Thoát vòng lặp.")
		}

	}
}

// Hàm Pancake_GetPosts lấy danh sách Posts từ server Pancake
// page_number: Số trang (bắt đầu từ 1)
// page_size: Kích thước trang (tối đa 30)
// since: Thời gian bắt đầu (Unix timestamp giây, UTC+0) - REQUIRED
// until: Thời gian kết thúc (Unix timestamp giây, UTC+0) - REQUIRED
// post_type: Loại post (optional): "video", "photo", "text", "livestream"
func Pancake_GetPosts(page_id string, page_number int, page_size int, since int64, until int64, post_type string) (result map[string]interface{}, err error) {
	log.Printf("[Pancake] Bắt đầu lấy danh sách posts - page_id: %s, page_number: %d, page_size: %d, since: %d, until: %d, type: %s", page_id, page_number, page_size, since, until, post_type)
	log.Printf("[Pancake] Pancake Base URL: %s", global.GlobalConfig.PancakeBaseUrl)

	// Khởi tạo client
	client := httpclient.NewHttpClient(global.GlobalConfig.PancakeBaseUrl, 60*time.Second)

	// Số lần thử request
	requestCount := 0
	for {
		requestCount++
		log.Printf("[Pancake] [Lần thử %d/5] Bắt đầu lấy danh sách posts", requestCount)

		// Nếu số lần thử vượt quá 5 lần thì thoát vòng lặp
		if requestCount > 5 {
			logError("[Pancake] LỖI: Đã thử quá nhiều lần (%d/5). Thoát vòng lặp.", requestCount)
			return nil, errors.New("Đã thử quá nhiều lần. Thoát vòng lặp.")
		}

	Start:
		// Lấy page_access_token
		log.Printf("[Pancake] [Lần thử %d/5] Lấy page_access_token từ local...", requestCount)
		page_access_token, err := Local_GetPageAccessToken(page_id)
		if err != nil {
			logError("[Pancake] [Lần thử %d/5] LỖI khi lấy page_access_token: %v", requestCount, err)
			return nil, err
		}
		if page_access_token == "" {
			log.Printf("[Pancake] [Lần thử %d/5] Không tìm thấy page_access_token trong biến local. Đang cập nhật...", requestCount)
			Local_UpdatePagesAccessToken(page_id)
			goto Start
		}
		log.Printf("[Pancake] [Lần thử %d/5] Đã lấy được page_access_token (length: %d)", requestCount, len(page_access_token))

		// Thiết lập params (since và until là REQUIRED)
		params := map[string]string{
			"page_access_token": page_access_token,
			"page_number":       strconv.Itoa(page_number),
			"page_size":         strconv.Itoa(page_size),
			"since":             strconv.FormatInt(since, 10),
			"until":             strconv.FormatInt(until, 10),
		}

		// Thêm type nếu có
		if post_type != "" {
			params["type"] = post_type
			log.Printf("[Pancake] [Lần thử %d/5] Thêm param type: %s", requestCount, post_type)
		}

		// Sử dụng adaptive rate limiter để nghỉ trước khi gửi request
		rateLimiter := apputility.GetPancakeRateLimiter()
		rateLimiter.Wait()

		endpoint := "/public_api/v1/pages/" + page_id + "/posts"
		log.Printf("[Pancake] [Lần thử %d/5] Gửi GET request đến endpoint: %s", requestCount, endpoint)
		log.Printf("[Pancake] [Lần thử %d/5] Request params: page_number=%d, page_size=%d, since=%d, until=%d", requestCount, page_number, page_size, since, until)

		// Gửi yêu cầu GET
		resp, err := client.GET(endpoint, params)
		if err != nil {
			logError("[Pancake] [Lần thử %d/5] ❌ LỖI khi gọi API GET: %v", requestCount, err)
			log.Printf("[Pancake] [Lần thử %d/5] Request endpoint: %s", requestCount, endpoint)
			log.Printf("[Pancake] [Lần thử %d/5] 📝 Chi tiết lỗi: %s", requestCount, err.Error())
			continue
		}

		statusCode := resp.StatusCode
		log.Printf("[Pancake] [Lần thử %d/5] Response Status Code: %d", requestCount, statusCode)

		// Kiểm tra mã trạng thái, nếu không phải 200 thì thử lại
		if statusCode != 200 {
			// Đọc response body để log lỗi
			bodyBytes, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			var errorCode interface{}
			if readErr == nil {
				logError("[Pancake] [Lần thử %d/5] ❌ LỖI: Response Body (raw): %s", requestCount, string(bodyBytes))
				var errorResult map[string]interface{}
				if err := json.Unmarshal(bodyBytes, &errorResult); err == nil {
					logError("[Pancake] [Lần thử %d/5] ❌ LỖI: Response Body (parsed): %+v", requestCount, errorResult)
					// In message lỗi nếu có
					if message, ok := errorResult["message"].(string); ok {
						log.Printf("[Pancake] [Lần thử %d/5] 📝 Message lỗi từ Pancake: %s", requestCount, message)
					}
					if ec, ok := errorResult["error_code"]; ok {
						errorCode = ec
						log.Printf("[Pancake] [Lần thử %d/5] 🔢 Error Code: %v", requestCount, errorCode)
					}
				}
			} else {
				log.Printf("[Pancake] [Lần thử %d/5] ❌ Không thể đọc response body: %v", requestCount, readErr)
			}
			// Ghi nhận lỗi để điều chỉnh rate limiter
			rateLimiter.RecordFailure(statusCode, errorCode)
			log.Printf("[Pancake] [Lần thử %d/5] ⚠️ Status Code: %d - Lấy danh sách posts thất bại. Thử lại", requestCount, statusCode)
			continue
		}

		// Đọc dữ liệu từ phản hồi
		bodyBytes, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			log.Printf("[Pancake] [Lần thử %d/5] ❌ Không thể đọc response body: %v", requestCount, readErr)
			continue
		}

		var result map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &result); err != nil {
			logError("[Pancake] [Lần thử %d/5] ❌ LỖI khi phân tích phản hồi JSON: %v", requestCount, err)
			log.Printf("[Pancake] [Lần thử %d/5] 📝 Response Body (raw): %s", requestCount, string(bodyBytes))
			continue
		}

		// Kiểm tra success
		if success, ok := result["success"].(bool); ok && success {
			// Ghi nhận thành công để điều chỉnh rate limiter
			rateLimiter.RecordSuccess()
			log.Printf("[Pancake] [Lần thử %d/5] ✅ Lấy danh sách posts thành công", requestCount)
			if total, ok := result["total"].(float64); ok {
				log.Printf("[Pancake] [Lần thử %d/5] Tổng số posts trong khoảng: %d", requestCount, int(total))
			}
			if posts, ok := result["posts"].([]interface{}); ok {
				log.Printf("[Pancake] [Lần thử %d/5] Số posts trong response: %d", requestCount, len(posts))
			}
			return result, nil
		} else {
			logError("[Pancake] [Lần thử %d/5] ❌ Response không thành công: %+v", requestCount, result)
			continue
		}
	}
}

// Hàm Pancake_GetCustomers lấy danh sách Customers từ server Pancake
// page_number: Số trang (bắt đầu từ 1)
// page_size: Kích thước trang (tối đa 100)
// since: Thời gian bắt đầu (Unix timestamp giây, UTC+0) - REQUIRED
// until: Thời gian kết thúc (Unix timestamp giây, UTC+0) - REQUIRED
// order_by: Sắp xếp (optional): "inserted_at" hoặc "updated_at" (default: "inserted_at")
func Pancake_GetCustomers(page_id string, page_number int, page_size int, since int64, until int64, order_by string) (result map[string]interface{}, err error) {
	log.Printf("[Pancake] Bắt đầu lấy danh sách customers - page_id: %s, page_number: %d, page_size: %d, since: %d, until: %d, order_by: %s", page_id, page_number, page_size, since, until, order_by)
	log.Printf("[Pancake] Pancake Base URL: %s", global.GlobalConfig.PancakeBaseUrl)

	// Khởi tạo client
	client := httpclient.NewHttpClient(global.GlobalConfig.PancakeBaseUrl, 60*time.Second)

	// Số lần thử request
	requestCount := 0
	for {
		requestCount++
		log.Printf("[Pancake] [Lần thử %d/5] Bắt đầu lấy danh sách customers", requestCount)

		// Nếu số lần thử vượt quá 5 lần thì thoát vòng lặp
		if requestCount > 5 {
			logError("[Pancake] LỖI: Đã thử quá nhiều lần (%d/5). Thoát vòng lặp.", requestCount)
			return nil, errors.New("Đã thử quá nhiều lần. Thoát vòng lặp.")
		}

	Start:
		// Lấy page_access_token
		log.Printf("[Pancake] [Lần thử %d/5] Lấy page_access_token từ local...", requestCount)
		page_access_token, err := Local_GetPageAccessToken(page_id)
		if err != nil {
			logError("[Pancake] [Lần thử %d/5] LỖI khi lấy page_access_token: %v", requestCount, err)
			return nil, err
		}
		if page_access_token == "" {
			log.Printf("[Pancake] [Lần thử %d/5] Không tìm thấy page_access_token trong biến local. Đang cập nhật...", requestCount)
			Local_UpdatePagesAccessToken(page_id)
			goto Start
		}
		log.Printf("[Pancake] [Lần thử %d/5] Đã lấy được page_access_token (length: %d)", requestCount, len(page_access_token))

		// Thiết lập params (since và until là REQUIRED)
		params := map[string]string{
			"page_access_token": page_access_token,
			"page_number":       strconv.Itoa(page_number),
			"page_size":         strconv.Itoa(page_size),
			"since":             strconv.FormatInt(since, 10),
			"until":             strconv.FormatInt(until, 10),
		}

		// Thêm order_by nếu có
		if order_by != "" {
			params["order_by"] = order_by
			log.Printf("[Pancake] [Lần thử %d/5] Thêm param order_by: %s", requestCount, order_by)
		}

		// Sử dụng adaptive rate limiter để nghỉ trước khi gửi request
		rateLimiter := apputility.GetPancakeRateLimiter()
		rateLimiter.Wait()

		endpoint := "/public_api/v1/pages/" + page_id + "/page_customers"
		log.Printf("[Pancake] [Lần thử %d/5] Gửi GET request đến endpoint: %s", requestCount, endpoint)
		log.Printf("[Pancake] [Lần thử %d/5] Request params: page_number=%d, page_size=%d, since=%d, until=%d, order_by=%s", requestCount, page_number, page_size, since, until, order_by)

		// Gửi yêu cầu GET
		resp, err := client.GET(endpoint, params)
		if err != nil {
			logError("[Pancake] [Lần thử %d/5] ❌ LỖI khi gọi API GET: %v", requestCount, err)
			log.Printf("[Pancake] [Lần thử %d/5] Request endpoint: %s", requestCount, endpoint)
			log.Printf("[Pancake] [Lần thử %d/5] 📝 Chi tiết lỗi: %s", requestCount, err.Error())
			continue
		}

		statusCode := resp.StatusCode
		log.Printf("[Pancake] [Lần thử %d/5] Response Status Code: %d", requestCount, statusCode)

		// Kiểm tra mã trạng thái, nếu không phải 200 thì thử lại
		if statusCode != 200 {
			// Đọc response body để log lỗi
			bodyBytes, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			var errorCode interface{}
			if readErr == nil {
				logError("[Pancake] [Lần thử %d/5] ❌ LỖI: Response Body (raw): %s", requestCount, string(bodyBytes))
				var errorResult map[string]interface{}
				if err := json.Unmarshal(bodyBytes, &errorResult); err == nil {
					logError("[Pancake] [Lần thử %d/5] ❌ LỖI: Response Body (parsed): %+v", requestCount, errorResult)
					// In message lỗi nếu có
					if message, ok := errorResult["message"].(string); ok {
						log.Printf("[Pancake] [Lần thử %d/5] 📝 Message lỗi từ Pancake: %s", requestCount, message)
					}
					if ec, ok := errorResult["error_code"]; ok {
						errorCode = ec
						log.Printf("[Pancake] [Lần thử %d/5] 🔢 Error Code: %v", requestCount, errorCode)
					}
				}
			} else {
				log.Printf("[Pancake] [Lần thử %d/5] ❌ Không thể đọc response body: %v", requestCount, readErr)
			}
			// Ghi nhận lỗi để điều chỉnh rate limiter
			rateLimiter.RecordFailure(statusCode, errorCode)
			log.Printf("[Pancake] [Lần thử %d/5] ⚠️ Status Code: %d - Lấy danh sách customers thất bại. Thử lại", requestCount, statusCode)
			continue
		}

		// Đọc dữ liệu từ phản hồi
		bodyBytes, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			log.Printf("[Pancake] [Lần thử %d/5] ❌ Không thể đọc response body: %v", requestCount, readErr)
			continue
		}

		var result map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &result); err != nil {
			logError("[Pancake] [Lần thử %d/5] ❌ LỖI khi phân tích phản hồi JSON: %v", requestCount, err)
			log.Printf("[Pancake] [Lần thử %d/5] 📝 Response Body (raw): %s", requestCount, string(bodyBytes))
			continue
		}

		// Kiểm tra success
		if success, ok := result["success"].(bool); ok && success {
			// Ghi nhận thành công để điều chỉnh rate limiter
			rateLimiter.RecordSuccess()
			log.Printf("[Pancake] [Lần thử %d/5] ✅ Lấy danh sách customers thành công", requestCount)
			if total, ok := result["total"].(float64); ok {
				log.Printf("[Pancake] [Lần thử %d/5] Tổng số customers trong khoảng: %d", requestCount, int(total))
			}
			if customers, ok := result["customers"].([]interface{}); ok {
				log.Printf("[Pancake] [Lần thử %d/5] Số customers trong response: %d", requestCount, len(customers))
			}
			return result, nil
		} else {
			logError("[Pancake] [Lần thử %d/5] ❌ Response không thành công: %+v", requestCount, result)
			continue
		}
	}
}
