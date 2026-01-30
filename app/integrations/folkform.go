package integrations

import (
	apputility "agent_pancake/app/utility"
	"agent_pancake/global"
	"agent_pancake/utility/httpclient"
	"agent_pancake/utility/hwid"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Các hằng số dùng chung
const (
	maxRetries         = 5
	retryDelay         = 100 * time.Millisecond
	defaultTimeout     = 10 * time.Second
	longTimeout        = 60 * time.Second
	quotaExceededWait  = 10 * time.Minute // Đợi 10 phút khi gặp QUOTA_EXCEEDED
	firebaseRetryDelay = 5 * time.Second  // Đợi 5 giây giữa các lần retry Firebase thông thường
)

// Helper function: Kiểm tra ApiToken
func checkApiToken() error {
	if global.ApiToken == "" {
		return errors.New("Chưa đăng nhập. Thoát vòng lặp.")
	}
	return nil
}

// Helper function: Tạo HTTP client với authorization header và organization context
// Thêm header X-Active-Role-ID để xác định context làm việc (Organization Context System - Version 3.2)
// Tự động lấy role đầu tiên nếu chưa có ActiveRoleId (backend yêu cầu header này bắt buộc)
func createAuthorizedClient(timeout time.Duration) *httpclient.HttpClient {
	client := httpclient.NewHttpClient(global.GlobalConfig.ApiBaseUrl, timeout)
	client.SetHeader("Authorization", "Bearer "+global.ApiToken)

	// Đảm bảo có ActiveRoleId trước khi gọi API (backend yêu cầu header X-Active-Role-ID bắt buộc)
	if global.ActiveRoleId == "" {
		// Tự động lấy role đầu tiên nếu chưa có
		ensureActiveRoleId()
	}

	// Thêm header X-Active-Role-ID (bắt buộc theo API v3.2+)
	if global.ActiveRoleId != "" {
		client.SetHeader("X-Active-Role-ID", global.ActiveRoleId)
	} else {
		// Nếu vẫn không có role sau khi thử lấy → log warning
		// Backend sẽ trả về lỗi AUTH_003 nếu không có header này
		log.Printf("[FolkForm] ⚠️ CẢNH BÁO: Không có Active Role ID, request có thể bị từ chối")
	}

	return client
}

// ensureActiveRoleId đảm bảo có ActiveRoleId bằng cách lấy role đầu tiên từ backend
// Hàm này được gọi tự động trong createAuthorizedClient nếu chưa có ActiveRoleId
// Lưu ý: Phải tạo client trực tiếp để tránh vòng lặp đệ quy với createAuthorizedClient
func ensureActiveRoleId() {
	if global.ActiveRoleId != "" {
		return // Đã có rồi, không cần làm gì
	}

	// Kiểm tra xem đã đăng nhập chưa
	if global.ApiToken == "" {
		log.Printf("[FolkForm] Chưa đăng nhập, không thể lấy Active Role ID")
		return
	}

	log.Printf("[FolkForm] Chưa có Active Role ID, đang lấy roles từ backend...")

	// Tạo client trực tiếp (KHÔNG dùng createAuthorizedClient để tránh vòng lặp đệ quy)
	// Endpoint /v1/auth/roles có thể không yêu cầu X-Active-Role-ID
	tempClient := httpclient.NewHttpClient(global.GlobalConfig.ApiBaseUrl, defaultTimeout)
	tempClient.SetHeader("Authorization", "Bearer "+global.ApiToken)

	// Gọi API lấy roles trực tiếp (không qua executeGetRequest để tránh vòng lặp)
	systemName := "[FolkForm]"
	log.Printf("%s [ensureActiveRoleId] Gửi GET request đến endpoint: /v1/auth/roles", systemName)

	// Sử dụng adaptive rate limiter
	rateLimiter := apputility.GetFolkFormRateLimiter()
	rateLimiter.Wait()

	resp, err := tempClient.GET("/v1/auth/roles", nil)
	if err != nil {
		log.Printf("[FolkForm] ⚠️ Không thể lấy roles: %v", err)
		return
	}

	statusCode := resp.StatusCode
	if statusCode != http.StatusOK {
		log.Printf("[FolkForm] ⚠️ Lỗi khi lấy roles, status code: %d", statusCode)
		resp.Body.Close()
		return
	}

	var result map[string]interface{}
	if err := httpclient.ParseJSONResponse(resp, &result); err != nil {
		log.Printf("[FolkForm] ⚠️ Lỗi khi parse response: %v", err)
		resp.Body.Close()
		return
	}
	resp.Body.Close()

	// Parse roles từ response
	var roles []interface{}
	if data, ok := result["data"].(map[string]interface{}); ok {
		if rolesArray, ok := data["roles"].([]interface{}); ok {
			roles = rolesArray
		} else if rolesArray, ok := data["items"].([]interface{}); ok {
			roles = rolesArray
		} else if rolesArray, ok := data["data"].([]interface{}); ok {
			roles = rolesArray
		}
	} else if rolesArray, ok := result["data"].([]interface{}); ok {
		roles = rolesArray
	}

	if len(roles) > 0 {
		if firstRole, ok := roles[0].(map[string]interface{}); ok {
			// Thử lấy roleId từ các field có thể có
			if roleId, ok := firstRole["id"].(string); ok && roleId != "" {
				global.ActiveRoleId = roleId
				log.Printf("[FolkForm] ✅ Đã lấy Active Role ID: %s", roleId)
				return
			} else if roleId, ok := firstRole["roleId"].(string); ok && roleId != "" {
				global.ActiveRoleId = roleId
				log.Printf("[FolkForm] ✅ Đã lấy Active Role ID: %s", roleId)
				return
			} else if roleId, ok := firstRole["_id"].(string); ok && roleId != "" {
				global.ActiveRoleId = roleId
				log.Printf("[FolkForm] ✅ Đã lấy Active Role ID: %s", roleId)
				return
			}
		}
	}

	log.Printf("[FolkForm] ⚠️ Không tìm thấy role ID trong response")
}

// executeGetRequest thực hiện GET request với retry logic và adaptive rate limiting
// Hàm này tự động retry tối đa maxRetries lần nếu gặp lỗi
// Sử dụng adaptive rate limiter để tránh rate limit từ server
// Tham số:
//   - client: HTTP client đã được cấu hình (có authorization header)
//   - endpoint: Endpoint path (ví dụ: "/v1/conversations")
//   - params: Query parameters (sẽ được thêm vào URL)
//   - logMessage: Message log khi thành công (optional)
//
// Trả về:
//   - map[string]interface{}: Response từ server (đã parse JSON)
//   - error: Lỗi nếu có (sau khi đã retry tối đa maxRetries lần)
func executeGetRequest(client *httpclient.HttpClient, endpoint string, params map[string]string, logMessage string) (map[string]interface{}, error) {
	systemName := "[FolkForm]"
	requestCount := 0
	for {
		requestCount++
		if requestCount > maxRetries {
			log.Printf("%s LỖI: Đã thử quá nhiều lần (%d/%d). Thoát vòng lặp.", systemName, requestCount, maxRetries)
			return nil, errors.New("Đã thử quá nhiều lần. Thoát vòng lặp.")
		}

		// Sử dụng adaptive rate limiter cho FolkForm
		rateLimiter := apputility.GetFolkFormRateLimiter()
		rateLimiter.Wait()

		resp, err := client.GET(endpoint, params)
		if err != nil {
			if requestCount >= 3 {
				log.Printf("%s ❌ LỖI khi gọi API GET (lần thử %d/%d): %v", systemName, requestCount, maxRetries, err)
			}
			continue
		}

		statusCode := resp.StatusCode

		if statusCode != http.StatusOK {
			// Đọc response body để log lỗi
			bodyBytes, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			var errorCode interface{}

			// Luôn log endpoint và status code khi có lỗi
			log.Printf("%s ❌ Lỗi (lần thử %d/%d): Cannot GET %s (status: %d)", systemName, requestCount, maxRetries, endpoint, statusCode)

			if readErr == nil && len(bodyBytes) > 0 {
				// Luôn log response body (raw) để xem server trả về gì
				bodyStr := string(bodyBytes)
				// Giới hạn độ dài log để tránh quá dài
				if len(bodyStr) > 500 {
					bodyStr = bodyStr[:500] + "...[truncated]"
				}
				log.Printf("%s 📝 Response Body (raw): %s", systemName, bodyStr)

				// Thử parse JSON để lấy thông tin chi tiết
				var errorResult map[string]interface{}
				if err := json.Unmarshal(bodyBytes, &errorResult); err == nil {
					// Lấy error code nếu có
					if ec, ok := errorResult["error_code"]; ok {
						errorCode = ec
					} else if code, ok := errorResult["code"]; ok {
						errorCode = code
					}
					// Log thông tin chi tiết nếu parse được JSON
					if message, ok := errorResult["message"].(string); ok {
						log.Printf("%s 📝 Error Message: %s", systemName, message)
					}
					if errorCode != nil {
						log.Printf("%s 📝 Error Code: %v", systemName, errorCode)
					}
					log.Printf("%s 📝 Response Body (parsed): %+v", systemName, errorResult)
				} else {
					// Nếu không parse được JSON, có thể là plain text hoặc HTML
					log.Printf("%s ⚠️  Response không phải JSON format (có thể là plain text hoặc HTML)", systemName)
				}
			} else if readErr != nil {
				log.Printf("%s ❌ Không thể đọc response body: %v", systemName, readErr)
			}

			// Ghi nhận lỗi để điều chỉnh rate limiter
			rateLimiter.RecordFailure(statusCode, errorCode)
			continue
		}

		var result map[string]interface{}
		if err := httpclient.ParseJSONResponse(resp, &result); err != nil {
			if requestCount >= 3 {
				log.Printf("%s ❌ LỖI khi phân tích phản hồi JSON (lần thử %d/%d): %v", systemName, requestCount, maxRetries, err)
			}
			continue
		}

		if result["status"] == "success" {
			// Chỉ log khi có logMessage và thử lần đầu
			if logMessage != "" && requestCount == 1 {
				log.Printf("%s %s", systemName, logMessage)
			}
			return result, nil
		}

		// Chỉ log lỗi khi thử nhiều lần
		if requestCount >= 3 {
			if message, ok := result["message"].(string); ok {
				log.Printf("%s ❌ Response không thành công (lần thử %d/%d): %s", systemName, requestCount, maxRetries, message)
			} else {
				log.Printf("%s ❌ Response không thành công (lần thử %d/%d): status %v", systemName, requestCount, maxRetries, result["status"])
			}
		}

		// Kiểm tra lại ở cuối vòng lặp (không cần thiết nhưng giữ để tương thích)
		if requestCount > maxRetries {
			return result, errors.New("Đã thử quá nhiều lần. Thoát vòng lặp.")
		}
	}
}

// executePostRequest thực hiện POST request với retry logic và adaptive rate limiting
// Hàm này tự động retry tối đa maxRetries lần nếu gặp lỗi
// Sử dụng adaptive rate limiter để tránh rate limit từ server
// Tham số:
//   - client: HTTP client đã được cấu hình (có authorization header)
//   - endpoint: Endpoint path (ví dụ: "/v1/conversations")
//   - data: Request body (sẽ được marshal thành JSON)
//   - params: Query parameters (sẽ được thêm vào URL)
//   - logMessage: Message log khi thành công (optional)
//   - errorLogMessage: Message log khi lỗi (optional, sẽ thêm số lần thử)
//   - withSleep: Có sleep giữa các lần retry không (true = có sleep)
//
// Trả về:
//   - map[string]interface{}: Response từ server (đã parse JSON)
//   - error: Lỗi nếu có (sau khi đã retry tối đa maxRetries lần)
func executePostRequest(client *httpclient.HttpClient, endpoint string, data interface{}, params map[string]string, logMessage string, errorLogMessage string, withSleep bool) (map[string]interface{}, error) {
	systemName := "[FolkForm]"
	requestCount := 0
	for {
		requestCount++
		if requestCount > maxRetries {
			log.Printf("%s LỖI: Đã thử quá nhiều lần (%d/%d). Thoát vòng lặp.", systemName, requestCount, maxRetries)
			return nil, errors.New("Đã thử quá nhiều lần. Thoát vòng lặp.")
		}

		// Sử dụng adaptive rate limiter cho FolkForm
		rateLimiter := apputility.GetFolkFormRateLimiter()
		if withSleep {
			rateLimiter.Wait()
		}

		resp, err := client.POST(endpoint, data, params)
		if err != nil {
			if requestCount >= 3 {
				log.Printf("%s ❌ LỖI khi gọi API POST (lần thử %d/%d): %v", systemName, requestCount, maxRetries, err)
			}
			continue
		}

		statusCode := resp.StatusCode

		// Kiểm tra mã trạng thái, nếu không phải 200 thì thử lại
		if statusCode != http.StatusOK {
			// Đọc response body để log lỗi
			bodyBytes, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			var errorCode interface{}

			// Luôn log endpoint và status code khi có lỗi
			log.Printf("%s ❌ Lỗi (lần thử %d/%d): Cannot POST %s (status: %d)", systemName, requestCount, maxRetries, endpoint, statusCode)

			if readErr == nil && len(bodyBytes) > 0 {
				// Luôn log response body (raw) để xem server trả về gì
				bodyStr := string(bodyBytes)
				// Giới hạn độ dài log để tránh quá dài
				if len(bodyStr) > 500 {
					bodyStr = bodyStr[:500] + "...[truncated]"
				}
				log.Printf("%s 📝 Response Body (raw): %s", systemName, bodyStr)

				// Thử parse JSON để lấy thông tin chi tiết
				var errorResult map[string]interface{}
				if err := json.Unmarshal(bodyBytes, &errorResult); err == nil {
					// Lấy error code nếu có
					if ec, ok := errorResult["error_code"]; ok {
						errorCode = ec
					} else if code, ok := errorResult["code"]; ok {
						errorCode = code
					}
					// Log thông tin chi tiết nếu parse được JSON
					if message, ok := errorResult["message"].(string); ok {
						log.Printf("%s 📝 Error Message: %s", systemName, message)
					}
					if errorCode != nil {
						log.Printf("%s 📝 Error Code: %v", systemName, errorCode)
					}
					log.Printf("%s 📝 Response Body (parsed): %+v", systemName, errorResult)
				} else {
					// Nếu không parse được JSON, có thể là plain text hoặc HTML
					log.Printf("%s ⚠️  Response không phải JSON format (có thể là plain text hoặc HTML)", systemName)
				}
			} else if readErr != nil {
				log.Printf("%s ❌ Không thể đọc response body: %v", systemName, readErr)
			}

			// Ghi nhận lỗi để điều chỉnh rate limiter
			rateLimiter.RecordFailure(statusCode, errorCode)
			continue
		}

		var result map[string]interface{}
		if err := httpclient.ParseJSONResponse(resp, &result); err != nil {
			if requestCount >= 3 {
				log.Printf("%s ❌ LỖI khi phân tích phản hồi JSON (lần thử %d/%d): %v", systemName, requestCount, maxRetries, err)
			}
			continue
		}

		// Ghi nhận kết quả response để điều chỉnh rate limiter
		success := result["status"] == "success"
		var errorCode interface{}
		if ec, ok := result["error_code"]; ok {
			errorCode = ec
		} else if code, ok := result["code"]; ok {
			errorCode = code
		}
		rateLimiter.RecordResponse(statusCode, success, errorCode)

		if result["status"] == "success" {
			if logMessage != "" {
				log.Printf("%s [Bước %d/%d] %s", systemName, requestCount, maxRetries, logMessage)
			} else {
				log.Printf("%s [Bước %d/%d] Request thành công", systemName, requestCount, maxRetries)
			}
			return result, nil
		}

		log.Printf("%s [Bước %d/%d] Response status không phải 'success': %v", systemName, requestCount, maxRetries, result["status"])
		if result["message"] != nil {
			log.Printf("%s [Bước %d/%d] Response message: %v", systemName, requestCount, maxRetries, result["message"])
		}
		log.Printf("%s [Bước %d/%d] Response Body: %+v", systemName, requestCount, maxRetries, result)

		// Kiểm tra lại ở cuối vòng lặp (không cần thiết nhưng giữ để tương thích)
		if requestCount > maxRetries {
			return result, errors.New("Đã thử quá nhiều lần. Thoát vòng lặp.")
		}
	}
}

// Helper function: Thực hiện PUT request với retry logic và kiểm tra status code
func executePutRequest(client *httpclient.HttpClient, endpoint string, data interface{}, params map[string]string, logMessage string, errorLogMessage string, withSleep bool) (map[string]interface{}, error) {
	systemName := "[FolkForm]"
	requestCount := 0
	for {
		requestCount++
		if requestCount > maxRetries {
			log.Printf("%s LỖI: Đã thử quá nhiều lần (%d/%d). Thoát vòng lặp.", systemName, requestCount, maxRetries)
			return nil, errors.New("Đã thử quá nhiều lần. Thoát vòng lặp.")
		}

		log.Printf("%s [Bước %d/%d] Gửi PUT request đến endpoint: %s", systemName, requestCount, maxRetries, endpoint)
		if data != nil {
			// Log data nhưng ẩn thông tin nhạy cảm
			if dataMap, ok := data.(map[string]interface{}); ok {
				safeData := make(map[string]interface{})
				for k, v := range dataMap {
					if k == "accessToken" || k == "pageAccessToken" {
						if str, ok := v.(string); ok && len(str) > 0 {
							safeData[k] = str[:min(10, len(str))] + "...[đã ẩn]"
						} else {
							safeData[k] = v
						}
					} else {
						safeData[k] = v
					}
				}
				log.Printf("%s [Bước %d/%d] Request data: %+v", systemName, requestCount, maxRetries, safeData)
			} else {
				log.Printf("%s [Bước %d/%d] Request data: [non-map data]", systemName, requestCount, maxRetries)
			}
		}
		if len(params) > 0 {
			log.Printf("%s [Bước %d/%d] Request params: %+v", systemName, requestCount, maxRetries, params)
		}

		// Sử dụng adaptive rate limiter cho FolkForm
		rateLimiter := apputility.GetFolkFormRateLimiter()
		if withSleep {
			rateLimiter.Wait()
		}

		resp, err := client.PUT(endpoint, data, params)
		if err != nil {
			log.Printf("%s [Bước %d/%d] LỖI khi gọi API PUT: %v", systemName, requestCount, maxRetries, err)
			log.Printf("%s [Bước %d/%d] Request endpoint: %s", systemName, requestCount, maxRetries, endpoint)
			continue
		}

		statusCode := resp.StatusCode
		log.Printf("%s [Bước %d/%d] Response Status Code: %d", systemName, requestCount, maxRetries, statusCode)

		// Kiểm tra mã trạng thái, nếu không phải 200 thì thử lại
		if statusCode != http.StatusOK {
			// Đọc response body để log lỗi
			bodyBytes, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			var errorCode interface{}

			// Luôn log endpoint và status code khi có lỗi
			log.Printf("%s ❌ Lỗi (lần thử %d/%d): Cannot PUT %s (status: %d)", systemName, requestCount, maxRetries, endpoint, statusCode)

			if readErr == nil && len(bodyBytes) > 0 {
				// Luôn log response body (raw) để xem server trả về gì
				bodyStr := string(bodyBytes)
				// Giới hạn độ dài log để tránh quá dài
				if len(bodyStr) > 500 {
					bodyStr = bodyStr[:500] + "...[truncated]"
				}
				log.Printf("%s 📝 Response Body (raw): %s", systemName, bodyStr)

				// Thử parse JSON để lấy thông tin chi tiết
				var errorResult map[string]interface{}
				if err := json.Unmarshal(bodyBytes, &errorResult); err == nil {
					// Lấy error code nếu có
					if ec, ok := errorResult["error_code"]; ok {
						errorCode = ec
					} else if code, ok := errorResult["code"]; ok {
						errorCode = code
					}
					// Log thông tin chi tiết nếu parse được JSON
					if message, ok := errorResult["message"].(string); ok {
						log.Printf("%s 📝 Error Message: %s", systemName, message)
					}
					if errorCode != nil {
						log.Printf("%s 📝 Error Code: %v", systemName, errorCode)
					}
					log.Printf("%s 📝 Response Body (parsed): %+v", systemName, errorResult)
				} else {
					// Nếu không parse được JSON, có thể là plain text hoặc HTML
					log.Printf("%s ⚠️  Response không phải JSON format (có thể là plain text hoặc HTML)", systemName)
				}
			} else if readErr != nil {
				log.Printf("%s ❌ Không thể đọc response body: %v", systemName, readErr)
			}

			// Ghi nhận lỗi để điều chỉnh rate limiter
			rateLimiter.RecordFailure(statusCode, errorCode)
			if errorLogMessage != "" {
				log.Printf("%s [Bước %d/%d] %s %d", systemName, requestCount, maxRetries, errorLogMessage, requestCount)
			}
			continue
		}

		var result map[string]interface{}
		if err := httpclient.ParseJSONResponse(resp, &result); err != nil {
			log.Printf("%s [Bước %d/%d] LỖI khi phân tích phản hồi JSON: %v", systemName, requestCount, maxRetries, err)
			// Đọc lại response body để log
			bodyBytes, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			if readErr == nil {
				log.Printf("%s [Bước %d/%d] Response Body (raw): %s", systemName, requestCount, maxRetries, string(bodyBytes))
			}
			continue
		}

		// Ghi nhận kết quả response để điều chỉnh rate limiter
		success := result["status"] == "success"
		var errorCode interface{}
		if ec, ok := result["error_code"]; ok {
			errorCode = ec
		} else if code, ok := result["code"]; ok {
			errorCode = code
		}
		rateLimiter.RecordResponse(statusCode, success, errorCode)

		if result["status"] == "success" {
			if logMessage != "" {
				log.Printf("%s [Bước %d/%d] %s", systemName, requestCount, maxRetries, logMessage)
			} else {
				log.Printf("%s [Bước %d/%d] Request thành công", systemName, requestCount, maxRetries)
			}
			return result, nil
		}

		log.Printf("%s [Bước %d/%d] Response status không phải 'success': %v", systemName, requestCount, maxRetries, result["status"])
		if result["message"] != nil {
			log.Printf("%s [Bước %d/%d] Response message: %v", systemName, requestCount, maxRetries, result["message"])
		}
		log.Printf("%s [Bước %d/%d] Response Body: %+v", systemName, requestCount, maxRetries, result)

		// Kiểm tra lại ở cuối vòng lặp (không cần thiết nhưng giữ để tương thích)
		if requestCount > maxRetries {
			return result, errors.New("Đã thử quá nhiều lần. Thoát vòng lặp.")
		}
	}
}

// Hàm FolkForm_GetLatestMessageItem lấy message_item mới nhất từ FolkForm theo conversationId
// Sử dụng endpoint /facebook/message-item/find-by-conversation/:conversationId với page=1, limit=1
// Backend sẽ tự động sort theo insertedAt desc để lấy message mới nhất
// Trả về insertedAt (Unix timestamp) của message mới nhất, hoặc 0 nếu chưa có messages
func FolkForm_GetLatestMessageItem(conversationId string) (latestInsertedAt int64, err error) {
	log.Printf("[FolkForm] Bắt đầu lấy message_item mới nhất - conversationId: %s", conversationId)

	if err := checkApiToken(); err != nil {
		log.Printf("[FolkForm] LỖI: %v", err)
		return 0, err
	}

	client := createAuthorizedClient(defaultTimeout)

	// Sử dụng endpoint đặc biệt /find-by-conversation với page=1, limit=1
	// Backend sẽ tự động sort theo insertedAt desc để lấy message mới nhất
	params := map[string]string{
		"page":  "1", // Page đầu tiên
		"limit": "1", // Chỉ lấy 1 message mới nhất
	}

	endpoint := "/v1/facebook/message-item/find-by-conversation/" + conversationId
	log.Printf("[FolkForm] Đang gửi request GET latest message_item đến FolkForm backend...")
	log.Printf("[FolkForm] Endpoint: %s với page=1, limit=1", endpoint)

	result, err := executeGetRequest(client, endpoint, params, "")
	if err != nil {
		log.Printf("[FolkForm] LỖI khi lấy latest message_item: %v", err)
		return 0, err
	}

	// Extract insertedAt từ message mới nhất
	// Response format: { data: FbMessageItem[], pagination: { page, limit, total } }
	if result != nil {
		var items []interface{}

		// Kiểm tra xem có data không
		if data, ok := result["data"].(map[string]interface{}); ok {
			if itemsArray, ok := data["data"].([]interface{}); ok {
				items = itemsArray
			} else if itemsArray, ok := data["items"].([]interface{}); ok {
				items = itemsArray
			}
		} else if itemsArray, ok := result["data"].([]interface{}); ok {
			items = itemsArray
		}

		if len(items) > 0 {
			// Lấy message đầu tiên (mới nhất - backend đã sort theo insertedAt desc)
			if firstItem, ok := items[0].(map[string]interface{}); ok {
				// Kiểm tra insertedAt (có thể là number hoặc numberLong)
				if insertedAt, ok := firstItem["insertedAt"].(float64); ok {
					latestInsertedAt = int64(insertedAt)
					log.Printf("[FolkForm] Tìm thấy message_item mới nhất - conversationId: %s, insertedAt: %d", conversationId, latestInsertedAt)
					return latestInsertedAt, nil
				} else if insertedAtMap, ok := firstItem["insertedAt"].(map[string]interface{}); ok {
					// Xử lý trường hợp MongoDB numberLong format
					if numberLong, ok := insertedAtMap["$numberLong"].(string); ok {
						if parsed, err := strconv.ParseInt(numberLong, 10, 64); err == nil {
							latestInsertedAt = parsed
							log.Printf("[FolkForm] Tìm thấy message_item mới nhất - conversationId: %s, insertedAt: %d", conversationId, latestInsertedAt)
							return latestInsertedAt, nil
						}
					}
				}
			}
		}
	}

	log.Printf("[FolkForm] Không tìm thấy message_item hoặc insertedAt = 0 - conversationId: %s", conversationId)
	return 0, nil // Không có messages → trả về 0, không phải lỗi
}

// Hàm FolkForm_UpsertMessages sẽ gửi yêu cầu upsert messages lên server sử dụng endpoint đặc biệt /upsert-messages
// Endpoint này tự động tách messages[] ra khỏi panCakeData và lưu vào 2 collections:
// - fb_messages: Metadata (không có messages[])
// - fb_message_items: Từng message riêng lẻ (mỗi message là 1 document)
// Tự động tránh duplicate theo messageId và cập nhật totalMessages, lastSyncedAt
func FolkForm_UpsertMessages(pageId string, pageUsername string, conversationId string, customerId string, panCakeData interface{}, hasMore bool) (result map[string]interface{}, err error) {
	log.Printf("[FolkForm] Bắt đầu upsert messages - pageId: %s, conversationId: %s, customerId: %s, hasMore: %v", pageId, conversationId, customerId, hasMore)

	if err := checkApiToken(); err != nil {
		log.Printf("[FolkForm] LỖI: %v", err)
		return nil, err
	}

	client := createAuthorizedClient(longTimeout)
	data := map[string]interface{}{
		"pageId":         pageId,
		"pageUsername":   pageUsername,
		"conversationId": conversationId,
		"customerId":     customerId,
		"panCakeData":    panCakeData, // Gửi đầy đủ panCakeData bao gồm messages[], backend sẽ tự động tách
		"hasMore":        hasMore,     // Còn messages để sync không
	}

	log.Printf("[FolkForm] Đang gửi request upsert-messages đến FolkForm backend...")
	log.Printf("[FolkForm] Endpoint: /facebook/message/upsert-messages")
	log.Printf("[FolkForm] Lưu ý: Backend sẽ tự động tách messages[] ra khỏi panCakeData và lưu vào 2 collections riêng biệt")

	// Không cần filter vì endpoint này dùng conversationId để upsert metadata
	// và messageId để upsert từng message riêng lẻ
	result, err = executePostRequest(client, "/v1/facebook/message/upsert-messages", data, nil, "Upsert messages thành công", "Upsert messages thất bại. Thử lại lần thứ", false)
	if err != nil {
		log.Printf("[FolkForm] LỖI khi upsert messages: %v", err)
	} else {
		log.Printf("[FolkForm] Upsert messages thành công - pageId: %s, conversationId: %s", pageId, conversationId)
	}
	return result, err
}

// Hàm FolkForm_CreateMessage sẽ gửi yêu cầu tạo/cập nhật tin nhắn lên server (sử dụng upsert)
// DEPRECATED: Nên dùng FolkForm_UpsertMessages() thay vì hàm này
// Upsert sẽ tự động insert nếu chưa có, hoặc update nếu đã có dựa trên unique field
// Lưu ý: messageData có thể là object chứa array messages hoặc single message
// Filter nên dựa trên messageId (từ panCakeData.id hoặc panCakeData.message_id) để tránh đè mất messages cũ
func FolkForm_CreateMessage(pageId string, pageUsername string, conversationId string, customerId string, messageData interface{}) (result map[string]interface{}, err error) {
	log.Printf("[FolkForm] Bắt đầu tạo/cập nhật tin nhắn - pageId: %s, conversationId: %s, customerId: %s", pageId, conversationId, customerId)

	if err := checkApiToken(); err != nil {
		log.Printf("[FolkForm] LỖI: %v", err)
		return nil, err
	}

	client := createAuthorizedClient(longTimeout)

	// Tìm messageId từ messageData để tạo filter chính xác
	// Mỗi message trong panCakeData.messages có field "id" (không phải "messageId")
	var messageId string
	if messageDataMap, ok := messageData.(map[string]interface{}); ok {
		// Kiểm tra xem có array messages không
		if messagesArray, ok := messageDataMap["messages"].([]interface{}); ok && len(messagesArray) > 0 {
			// Lấy message đầu tiên để extract id
			if firstMessage, ok := messagesArray[0].(map[string]interface{}); ok {
				// Lấy id từ field "id" của message (ví dụ: "m_WEcv3kqFFSvzoF_S77LyQgMBLexzlInjdlLHZU4paUsdb8lSR0_GVIX7bHiVAdgCYsLEBUrT8ShCtbicVMLHYw")
				if id, ok := firstMessage["id"].(string); ok && id != "" {
					messageId = id
					log.Printf("[FolkForm] Tìm thấy message id từ panCakeData.messages[0].id: %s", messageId)
				}
			}
		} else {
			// Nếu không phải array, có thể là single message object
			if id, ok := messageDataMap["id"].(string); ok && id != "" {
				messageId = id
				log.Printf("[FolkForm] Tìm thấy message id từ panCakeData.id: %s", messageId)
			}
		}
	}

	data := map[string]interface{}{
		"pageId":         pageId,
		"pageUsername":   pageUsername,
		"conversationId": conversationId,
		"customerId":     customerId,
		"panCakeData":    messageData,
	}

	// Tạo filter cho upsert - ưu tiên dùng messageId nếu có, nếu không thì dùng conversationId + pageId
	params := make(map[string]string)
	if messageId != "" {
		// Filter theo messageId (unique) - đây là cách đúng để tránh đè mất messages cũ
		filterMap := map[string]string{
			"messageId": messageId,
		}
		filterBytes, err := json.Marshal(filterMap)
		if err != nil {
			log.Printf("[FolkForm] LỖI khi tạo filter JSON: %v", err)
			// Fallback: tạo filter thủ công
			filter := `{"messageId":"` + messageId + `"}`
			params["filter"] = filter
			log.Printf("[FolkForm] Tạo filter cho upsert message (theo messageId): %s", filter)
		} else {
			params["filter"] = string(filterBytes)
			log.Printf("[FolkForm] Tạo filter cho upsert message (theo messageId): %s", params["filter"])
		}
	} else if pageId != "" && conversationId != "" {
		// Fallback: dùng conversationId + pageId nếu không có messageId
		// Lưu ý: Filter này có thể không chính xác nếu có nhiều messages trong cùng conversation
		log.Printf("[FolkForm] CẢNH BÁO: Không tìm thấy messageId, dùng filter conversationId + pageId (có thể không chính xác)")
		filterMap := map[string]string{
			"conversationId": conversationId,
			"pageId":         pageId,
		}
		filterBytes, err := json.Marshal(filterMap)
		if err != nil {
			log.Printf("[FolkForm] LỖI khi tạo filter JSON: %v", err)
			filter := `{"conversationId":"` + conversationId + `","pageId":"` + pageId + `"}`
			params["filter"] = filter
			log.Printf("[FolkForm] Tạo filter cho upsert message (fallback): %s", filter)
		} else {
			params["filter"] = string(filterBytes)
			log.Printf("[FolkForm] Tạo filter cho upsert message (fallback): %s", params["filter"])
		}
	} else {
		// Fallback cuối cùng: chỉ dùng conversationId
		if conversationId != "" {
			filter := `{"conversationId":"` + conversationId + `"}`
			params["filter"] = filter
			log.Printf("[FolkForm] Tạo filter cho upsert message (fallback - thiếu pageId): %s", filter)
		} else {
			log.Printf("[FolkForm] CẢNH BÁO: Thiếu cả conversationId và pageId, upsert có thể không hoạt động đúng")
		}
	}

	log.Printf("[FolkForm] Đang gửi request upsert message đến FolkForm backend...")
	// Sử dụng upsert-one để tự động insert hoặc update
	result, err = executePostRequest(client, "/v1/facebook/message/upsert-one", data, params, "Gửi tin nhắn thành công", "Gửi tin nhắn thất bại. Thử lại lần thứ", false)
	if err != nil {
		log.Printf("[FolkForm] LỖI khi tạo/cập nhật tin nhắn: %v", err)
	} else {
		log.Printf("[FolkForm] Tạo/cập nhật tin nhắn thành công - pageId: %s, conversationId: %s", pageId, conversationId)
	}
	return result, err
}

// Hàm FolkForm_GetConversations sẽ gửi yêu cầu lấy danh sách hội thoại từ server
// Hàm FolkForm_GetConversations sẽ gửi yêu cầu lấy danh sách hội thoại từ server
// Hàm này sử dụng endpoint phân trang với page và limit
func FolkForm_GetConversations(page int, limit int) (result map[string]interface{}, err error) {

	log.Printf("[FolkForm] Bắt đầu lấy danh sách hội thoại với phân trang - page: %d, limit: %d", page, limit)

	if err := checkApiToken(); err != nil {
		log.Printf("[FolkForm] LỖI: %v", err)
		return nil, err
	}

	client := createAuthorizedClient(defaultTimeout)
	// Đảm bảo params phân trang luôn được gửi
	params := map[string]string{
		"page":  strconv.Itoa(page),
		"limit": strconv.Itoa(limit),
	}

	log.Printf("[FolkForm] Đang gửi request GET conversations với phân trang đến FolkForm backend...")
	log.Printf("[FolkForm] Endpoint: /facebook/conversation/find-with-pagination với params phân trang: page=%d, limit=%d", page, limit)
	result, err = executeGetRequest(client, "/v1/facebook/conversation/find-with-pagination", params, "")
	if err != nil {
		log.Printf("[FolkForm] LỖI khi lấy danh sách hội thoại (page=%d, limit=%d): %v", page, limit, err)
	} else {
		log.Printf("[FolkForm] Lấy danh sách hội thoại thành công với phân trang - page: %d, limit: %d", page, limit)
	}
	return result, err
}

// Hàm FolkForm_GetConversationsWithPageId sẽ gửi yêu cầu lấy danh sách hội thoại từ server với pageId
// Hàm này sử dụng endpoint phân trang với page và limit
func FolkForm_GetConversationsWithPageId(page int, limit int, pageId string) (result map[string]interface{}, err error) {

	log.Printf("[FolkForm] Bắt đầu lấy danh sách hội thoại theo pageId với phân trang - page: %d, limit: %d, pageId: %s", page, limit, pageId)

	if err := checkApiToken(); err != nil {
		log.Printf("[FolkForm] LỖI: %v", err)
		return nil, err
	}

	client := createAuthorizedClient(defaultTimeout)
	// Đảm bảo params phân trang luôn được gửi
	params := map[string]string{
		"page":   strconv.Itoa(page),
		"limit":  strconv.Itoa(limit),
		"pageId": pageId,
	}

	log.Printf("[FolkForm] Đang gửi request GET conversations với phân trang đến FolkForm backend...")
	log.Printf("[FolkForm] Endpoint: /facebook/conversation/sort-by-api-update với params phân trang: page=%d, limit=%d, pageId=%s", page, limit, pageId)
	// Sử dụng endpoint sort-by-api-update để lấy conversations mới nhất
	result, err = executeGetRequest(client, "/v1/facebook/conversation/sort-by-api-update", params, "")
	if err != nil {
		log.Printf("[FolkForm] LỖI khi lấy danh sách hội thoại theo pageId (page=%d, limit=%d): %v", page, limit, err)
	} else {
		log.Printf("[FolkForm] Lấy danh sách hội thoại theo pageId thành công với phân trang - pageId: %s, page: %d, limit: %d", pageId, page, limit)
	}
	return result, err
}

// FolkForm_GetUnrepliedConversationsWithPageId lấy conversations chưa trả lời trong khoảng thời gian từ FolkForm với filter MongoDB
// Sử dụng endpoint find-with-pagination với filter để chỉ lấy conversations cần thiết
// Tham số:
// - page: Số trang
// - limit: Số lượng items mỗi trang
// - pageId: ID của page
// - minMinutesAgo: Số phút tối thiểu trước (ví dụ: 5 phút)
// - maxMinutesAgo: Số phút tối đa trước (ví dụ: 300 phút)
// Trả về result map và error
func FolkForm_GetUnrepliedConversationsWithPageId(page int, limit int, pageId string, minMinutesAgo int, maxMinutesAgo int) (result map[string]interface{}, err error) {
	log.Printf("[FolkForm] Bắt đầu lấy danh sách conversations chưa trả lời theo pageId với filter - page: %d, limit: %d, pageId: %s, minMinutesAgo: %d, maxMinutesAgo: %d", page, limit, pageId, minMinutesAgo, maxMinutesAgo)

	if err := checkApiToken(); err != nil {
		log.Printf("[FolkForm] LỖI: %v", err)
		return nil, err
	}

	client := createAuthorizedClient(defaultTimeout)

	// Tính toán thời gian min và max (Unix timestamp milliseconds)
	now := time.Now()
	minTime := now.Add(-time.Duration(maxMinutesAgo) * time.Minute) // maxMinutesAgo phút trước (cũ nhất)
	maxTime := now.Add(-time.Duration(minMinutesAgo) * time.Minute) // minMinutesAgo phút trước (mới nhất)

	minTimeMs := minTime.Unix() * 1000
	maxTimeMs := maxTime.Unix() * 1000

	// Tạo MongoDB filter để chỉ lấy conversations:
	// 1. Có pageId đúng
	// 2. panCakeUpdatedAt trong khoảng minTimeMs - maxTimeMs (milliseconds)
	// 3. Không có tag "spam" hoặc "khách block"
	// Lưu ý:
	// - Không filter last_sent_by.id != pageId ở database level (backend không hỗ trợ $ne)
	// - Sẽ filter last_sent_by.id != pageId ở application level sau khi lấy dữ liệu
	// - Sử dụng panCakeUpdatedAt (number) thay vì updated_at (string) để filter hiệu quả hơn
	filter := map[string]interface{}{
		"pageId": pageId,
		"panCakeUpdatedAt": map[string]interface{}{
			"$gte": minTimeMs,
			"$lte": maxTimeMs,
		},
		"$or": []map[string]interface{}{
			{"panCakeData.tags": map[string]interface{}{"$exists": false}},
			{"panCakeData.tags": map[string]interface{}{"$size": 0}},
			{
				"panCakeData.tags": map[string]interface{}{
					"$not": map[string]interface{}{
						"$elemMatch": map[string]interface{}{
							"text": map[string]interface{}{
								"$in": []string{"spam", "khách block"},
							},
						},
					},
				},
			},
		},
	}

	// Convert filter sang JSON string để gửi trong query params
	filterJSON, err := json.Marshal(filter)
	if err != nil {
		log.Printf("[FolkForm] LỖI khi marshal filter: %v", err)
		return nil, err
	}

	// Đảm bảo params phân trang và filter luôn được gửi
	params := map[string]string{
		"page":   strconv.Itoa(page),
		"limit":  strconv.Itoa(limit),
		"filter": string(filterJSON),
	}

	log.Printf("[FolkForm] Đang gửi request GET conversations chưa trả lời với filter đến FolkForm backend...")
	log.Printf("[FolkForm] Endpoint: /facebook/conversation/find-with-pagination với filter: %s", string(filterJSON))
	log.Printf("[FolkForm] Params: page=%d, limit=%d", page, limit)

	result, err = executeGetRequest(client, "/v1/facebook/conversation/find-with-pagination", params, "")
	if err != nil {
		log.Printf("[FolkForm] LỖI khi lấy danh sách conversations chưa trả lời theo pageId (page=%d, limit=%d): %v", page, limit, err)
	} else {
		log.Printf("[FolkForm] Lấy danh sách conversations chưa trả lời theo pageId thành công với filter - pageId: %s, page: %d, limit: %d", pageId, page, limit)
	}
	return result, err
}

// FolkForm_GetUnseenConversationsWithPageId lấy conversations unseen từ FolkForm với filter MongoDB
// Sử dụng endpoint find-with-pagination với filter để chỉ lấy conversations unseen (panCakeData.seen = false)
// Tối ưu hơn so với việc lấy tất cả rồi filter ở code
func FolkForm_GetUnseenConversationsWithPageId(page int, limit int, pageId string) (result map[string]interface{}, err error) {
	log.Printf("[FolkForm] Bắt đầu lấy danh sách conversations unseen theo pageId với filter - page: %d, limit: %d, pageId: %s", page, limit, pageId)

	if err := checkApiToken(); err != nil {
		log.Printf("[FolkForm] LỖI: %v", err)
		return nil, err
	}

	client := createAuthorizedClient(defaultTimeout)

	// Tạo MongoDB filter để chỉ lấy conversations unseen
	// Filter: panCakeData.seen = false hoặc panCakeData.seen không tồn tại
	filter := map[string]interface{}{
		"pageId": pageId,
		"$or": []map[string]interface{}{
			{"panCakeData.seen": false},
			{"panCakeData.seen": map[string]interface{}{"$exists": false}},
		},
	}

	// Convert filter sang JSON string để gửi trong query params
	filterJSON, err := json.Marshal(filter)
	if err != nil {
		log.Printf("[FolkForm] LỖI khi marshal filter: %v", err)
		return nil, err
	}

	// Đảm bảo params phân trang và filter luôn được gửi
	params := map[string]string{
		"page":   strconv.Itoa(page),
		"limit":  strconv.Itoa(limit),
		"filter": string(filterJSON),
	}

	log.Printf("[FolkForm] Đang gửi request GET conversations unseen với filter đến FolkForm backend...")
	log.Printf("[FolkForm] Endpoint: /facebook/conversation/find-with-pagination với filter: %s", string(filterJSON))
	log.Printf("[FolkForm] Params: page=%d, limit=%d", page, limit)

	result, err = executeGetRequest(client, "/v1/facebook/conversation/find-with-pagination", params, "")
	if err != nil {
		log.Printf("[FolkForm] LỖI khi lấy danh sách conversations unseen theo pageId (page=%d, limit=%d): %v", page, limit, err)
	} else {
		log.Printf("[FolkForm] Lấy danh sách conversations unseen theo pageId thành công với filter - pageId: %s, page: %d, limit: %d", pageId, page, limit)
	}
	return result, err
}

// FolkForm_GetLastConversationId lấy conversation mới nhất từ FolkForm
// Sử dụng endpoint sort-by-api-update (sort desc - mới nhất trước)
// Endpoint này tự động filter theo pageId và sort theo panCakeUpdatedAt desc
func FolkForm_GetLastConversationId(pageId string) (conversationId string, err error) {
	log.Printf("[FolkForm] Lấy conversation mới nhất - pageId: %s", pageId)

	// Endpoint: GET /facebook/conversation/sort-by-api-update?page=1&limit=1&pageId={pageId}
	// Tự động filter theo pageId và sort theo panCakeUpdatedAt desc (mới nhất trước)
	result, err := FolkForm_GetConversationsWithPageId(1, 1, pageId)
	if err != nil {
		return "", err
	}

	// Parse response
	var items []interface{}
	if dataMap, ok := result["data"].(map[string]interface{}); ok {
		if itemsArray, ok := dataMap["items"].([]interface{}); ok {
			items = itemsArray
		}
	}

	if len(items) == 0 {
		log.Printf("[FolkForm] Không tìm thấy conversation nào - pageId: %s", pageId)
		return "", nil // Không có conversation → trả về empty
	}

	// items[0] = conversation mới nhất (panCakeUpdatedAt lớn nhất)
	firstItem := items[0]
	if conversation, ok := firstItem.(map[string]interface{}); ok {
		if convId, ok := conversation["conversationId"].(string); ok {
			log.Printf("[FolkForm] Tìm thấy conversation mới nhất - conversationId: %s", convId)
			return convId, nil
		}
	}

	return "", nil
}

// FolkForm_GetPrioritySyncConversations lấy conversations có needsPrioritySync=true từ FolkForm
// Sử dụng endpoint find-with-pagination với filter để chỉ lấy conversations cần ưu tiên sync
// Tham số:
// - page: Số trang
// - limit: Số lượng items mỗi trang
// Trả về result map và error
func FolkForm_GetPrioritySyncConversations(page int, limit int) (result map[string]interface{}, err error) {
	log.Printf("[FolkForm] Bắt đầu lấy danh sách conversations cần ưu tiên sync - page: %d, limit: %d", page, limit)

	if err := checkApiToken(); err != nil {
		log.Printf("[FolkForm] LỖI: %v", err)
		return nil, err
	}

	client := createAuthorizedClient(defaultTimeout)

	// Tạo MongoDB filter để chỉ lấy conversations có needsPrioritySync=true
	filter := map[string]interface{}{
		"needsPrioritySync": true,
	}

	// Convert filter sang JSON string để gửi trong query params
	filterJSON, err := json.Marshal(filter)
	if err != nil {
		log.Printf("[FolkForm] LỖI khi marshal filter: %v", err)
		return nil, err
	}

	// Đảm bảo params phân trang và filter luôn được gửi
	params := map[string]string{
		"page":   strconv.Itoa(page),
		"limit":  strconv.Itoa(limit),
		"filter": string(filterJSON),
	}

	log.Printf("[FolkForm] Đang gửi request GET conversations cần ưu tiên sync với filter đến FolkForm backend...")
	log.Printf("[FolkForm] Endpoint: /facebook/conversation/find-with-pagination với filter: %s", string(filterJSON))
	log.Printf("[FolkForm] Params: page=%d, limit=%d", page, limit)

	result, err = executeGetRequest(client, "/v1/facebook/conversation/find-with-pagination", params, "")
	if err != nil {
		log.Printf("[FolkForm] LỖI khi lấy danh sách conversations cần ưu tiên sync (page=%d, limit=%d): %v", page, limit, err)
	} else {
		log.Printf("[FolkForm] Lấy danh sách conversations cần ưu tiên sync thành công với filter - page: %d, limit: %d", page, limit)
	}
	return result, err
}

// FolkForm_UpdateConversationNeedsPrioritySync cập nhật flag needsPrioritySync của conversation
// Tham số:
// - conversationId: ID của conversation
// - needsPrioritySync: Giá trị mới của flag
// Trả về result map và error
func FolkForm_UpdateConversationNeedsPrioritySync(conversationId string, needsPrioritySync bool) (result map[string]interface{}, err error) {
	log.Printf("[FolkForm] Bắt đầu cập nhật flag needsPrioritySync - conversationId: %s, needsPrioritySync: %v", conversationId, needsPrioritySync)

	if err := checkApiToken(); err != nil {
		log.Printf("[FolkForm] LỖI: %v", err)
		return nil, err
	}

	client := createAuthorizedClient(defaultTimeout)

	// Tạo filter để tìm conversation theo conversationId
	filter := map[string]interface{}{
		"conversationId": conversationId,
	}

	// Tạo update data
	updateData := map[string]interface{}{
		"needsPrioritySync": needsPrioritySync,
	}

	// Convert filter và updateData sang JSON string
	filterJSON, err := json.Marshal(filter)
	if err != nil {
		log.Printf("[FolkForm] LỖI khi marshal filter: %v", err)
		return nil, err
	}

	params := map[string]string{
		"filter": string(filterJSON),
	}

	log.Printf("[FolkForm] Đang gửi request PUT update conversation needsPrioritySync đến FolkForm backend...")
	log.Printf("[FolkForm] Endpoint: /facebook/conversation/update-one với filter: %s", string(filterJSON))
	log.Printf("[FolkForm] Update data: %+v", updateData)

	result, err = executePutRequest(client, "/v1/facebook/conversation/update-one", updateData, params,
		"Cập nhật needsPrioritySync thành công", "Cập nhật needsPrioritySync thất bại. Thử lại lần thứ", false)
	if err != nil {
		log.Printf("[FolkForm] LỖI khi cập nhật needsPrioritySync: %v", err)
	} else {
		log.Printf("[FolkForm] Cập nhật needsPrioritySync thành công - conversationId: %s, needsPrioritySync: %v", conversationId, needsPrioritySync)
	}
	return result, err
}

// FolkForm_GetOldestConversationId lấy conversation cũ nhất từ FolkForm
// Filter theo pageId và sort theo panCakeUpdatedAt asc (cũ nhất trước)
func FolkForm_GetOldestConversationId(pageId string) (conversationId string, err error) {
	log.Printf("[FolkForm] Lấy conversation cũ nhất - pageId: %s", pageId)

	if err := checkApiToken(); err != nil {
		return "", err
	}

	client := createAuthorizedClient(defaultTimeout)

	// Dùng GET với query string
	// GET /facebook/conversation/find?filter={"pageId":"..."}&options={"sort":{"panCakeUpdatedAt":1},"limit":1}
	params := map[string]string{
		"filter":  `{"pageId":"` + pageId + `"}`,
		"options": `{"sort":{"panCakeUpdatedAt":1},"limit":1}`, // Sort asc (cũ nhất trước)
	}

	result, err := executeGetRequest(
		client,
		"/v1/facebook/conversation/find",
		params,
		"Lấy conversation cũ nhất thành công",
	)

	if err != nil {
		return "", err
	}

	// Parse response
	var items []interface{}
	if dataMap, ok := result["data"].(map[string]interface{}); ok {
		if itemsArray, ok := dataMap["items"].([]interface{}); ok {
			items = itemsArray
		} else if itemsArray, ok := dataMap["data"].([]interface{}); ok {
			items = itemsArray
		}
	} else if itemsArray, ok := result["data"].([]interface{}); ok {
		items = itemsArray
	}

	if len(items) == 0 {
		log.Printf("[FolkForm] Không tìm thấy conversation nào - pageId: %s", pageId)
		return "", nil // Không có conversation → trả về empty
	}

	// items[0] = conversation cũ nhất (panCakeUpdatedAt nhỏ nhất)
	firstItem := items[0]
	if conversation, ok := firstItem.(map[string]interface{}); ok {
		if convId, ok := conversation["conversationId"].(string); ok {
			log.Printf("[FolkForm] Tìm thấy conversation cũ nhất - conversationId: %s", convId)
			return convId, nil
		}
	}

	return "", nil
}

// Hàm FolkForm_CreateConversation sẽ gửi yêu cầu tạo/cập nhật hội thoại lên server (sử dụng upsert)
// Upsert sẽ tự động insert nếu chưa có, hoặc update nếu đã có dựa trên conversationId (unique)
func FolkForm_CreateConversation(pageId string, pageUsername string, conversation_data interface{}) (result map[string]interface{}, err error) {
	log.Printf("[FolkForm] Bắt đầu tạo/cập nhật hội thoại - pageId: %s, pageUsername: %s", pageId, pageUsername)

	if err := checkApiToken(); err != nil {
		log.Printf("[FolkForm] LỖI: %v", err)
		return nil, err
	}

	client := createAuthorizedClient(longTimeout)

	// Tạo bản copy của conversation_data và loại bỏ messages[] để tránh đè mất messages cũ
	// Messages sẽ được upsert riêng lẻ thông qua FolkForm_CreateMessage
	conversationDataWithoutMessages := make(map[string]interface{})
	if conversationMap, ok := conversation_data.(map[string]interface{}); ok {
		// Copy tất cả fields trừ messages
		for key, value := range conversationMap {
			if key != "messages" {
				conversationDataWithoutMessages[key] = value
			}
		}
		log.Printf("[FolkForm] Đã loại bỏ messages[] khỏi panCakeData để tránh đè mất messages cũ khi upsert conversation")
	} else {
		// Nếu không phải map, giữ nguyên
		conversationDataWithoutMessages = conversation_data.(map[string]interface{})
	}

	data := map[string]interface{}{
		"pageId":       pageId,
		"pageUsername": pageUsername,
		"panCakeData":  conversationDataWithoutMessages, // Không có messages[]
	}

	// Tạo filter cho upsert dựa trên conversationId từ panCakeData
	// Sử dụng JSON encoding để tạo filter an toàn, tránh lỗi với ký tự đặc biệt
	params := make(map[string]string)
	var conversationId string
	var customerId string

	if conversationMap, ok := conversation_data.(map[string]interface{}); ok {
		// Lấy conversationId từ panCakeData (field "id" trong response từ Pancake)
		if id, ok := conversationMap["id"].(string); ok && id != "" {
			conversationId = id
		} else {
			// Fallback: thử tìm conversationId trực tiếp
			if id, ok := conversationMap["conversationId"].(string); ok && id != "" {
				conversationId = id
			}
		}

		// Lấy customerId từ panCakeData (field "customer_id" trong response từ Pancake - snake_case)
		if cid, ok := conversationMap["customer_id"].(string); ok && cid != "" {
			customerId = cid
			// Thêm customerId vào data để backend có thể xử lý
			data["customerId"] = customerId
		} else {
			// Fallback: thử tìm customerId trực tiếp (camelCase)
			if cid, ok := conversationMap["customerId"].(string); ok && cid != "" {
				customerId = cid
				data["customerId"] = customerId
			}
		}

		// Tạo filter JSON an toàn bằng cách sử dụng json.Marshal
		if conversationId != "" {
			filterMap := map[string]string{
				"conversationId": conversationId,
			}
			filterBytes, err := json.Marshal(filterMap)
			if err != nil {
				log.Printf("[FolkForm] LỖI khi tạo filter JSON: %v", err)
			} else {
				params["filter"] = string(filterBytes)
				log.Printf("[FolkForm] Tạo filter cho upsert conversation: %s", params["filter"])
			}
		} else {
			log.Printf("[FolkForm] CẢNH BÁO: Không tìm thấy conversationId trong panCakeData, upsert có thể không hoạt động đúng")
		}
	} else {
		log.Printf("[FolkForm] CẢNH BÁO: conversation_data không phải là map, upsert có thể không hoạt động đúng")
	}

	log.Printf("[FolkForm] Đang gửi request upsert conversation đến FolkForm backend...")
	// Sử dụng upsert-one để tự động insert hoặc update dựa trên conversationId
	result, err = executePostRequest(client, "/v1/facebook/conversation/upsert-one", data, params, "Gửi hội thoại thành công", "Gửi hội thoại thất bại. Thử lại lần thứ", false)
	if err != nil {
		log.Printf("[FolkForm] LỖI khi tạo/cập nhật hội thoại: %v", err)
	} else {
		log.Printf("[FolkForm] Tạo/cập nhật hội thoại thành công - pageId: %s, conversationId: %s", pageId, conversationId)
	}
	return result, err
}

// Hàm FolkForm_GetFbPageById sẽ gửi yêu cầu lấy thông tin trang Facebook từ server
func FolkForm_GetFbPageById(id string) (result map[string]interface{}, err error) {
	log.Printf("[FolkForm] Bắt đầu lấy thông tin trang Facebook theo ID - id: %s", id)

	if err := checkApiToken(); err != nil {
		log.Printf("[FolkForm] LỖI: %v", err)
		return nil, err
	}

	client := createAuthorizedClient(defaultTimeout)
	log.Printf("[FolkForm] Đang gửi request GET page (find-by-id) đến FolkForm backend...")
	result, err = executeGetRequest(client, "/v1/facebook/page/find-by-id/"+id, nil, "")
	if err != nil {
		log.Printf("[FolkForm] LỖI khi lấy thông tin trang Facebook theo ID: %v", err)
	} else {
		log.Printf("[FolkForm] Lấy thông tin trang Facebook theo ID thành công - id: %s", id)
	}
	return result, err
}

// Hàm FolkForm_GetFbPageByPageId sẽ gửi yêu cầu lấy thông tin trang Facebook từ server
// Sử dụng endpoint đặc biệt /facebook/page/find-by-page-id/:id thay vì endpoint CRUD thông thường
func FolkForm_GetFbPageByPageId(pageId string) (result map[string]interface{}, err error) {
	log.Printf("[FolkForm] Bắt đầu lấy thông tin trang Facebook theo pageId - pageId: %s", pageId)

	if err := checkApiToken(); err != nil {
		log.Printf("[FolkForm] LỖI: %v", err)
		return nil, err
	}

	client := createAuthorizedClient(defaultTimeout)
	log.Printf("[FolkForm] Đang gửi request GET page (find-by-page-id) đến FolkForm backend...")
	// Sử dụng endpoint đặc biệt /facebook/page/find-by-page-id/:id thay vì find-one với filter
	result, err = executeGetRequest(client, "/v1/facebook/page/find-by-page-id/"+pageId, nil, "")
	if err != nil {
		log.Printf("[FolkForm] LỖI khi lấy thông tin trang Facebook theo pageId: %v", err)
	} else {
		log.Printf("[FolkForm] Lấy thông tin trang Facebook theo pageId thành công - pageId: %s", pageId)
	}
	return result, err
}

// Hàm FolkForm_GetFbPages sẽ gửi yêu cầu lấy danh sách trang Facebook từ server
// Hàm FolkForm_GetFbPages sẽ gửi yêu cầu lấy danh sách trang Facebook từ server
// Hàm này sử dụng endpoint phân trang với page và limit
func FolkForm_GetFbPages(page int, limit int) (result map[string]interface{}, err error) {

	log.Printf("[FolkForm] Bắt đầu lấy danh sách trang Facebook với phân trang - page: %d, limit: %d", page, limit)

	if err := checkApiToken(); err != nil {
		log.Printf("[FolkForm] LỖI: %v", err)
		return nil, err
	}

	client := createAuthorizedClient(defaultTimeout)
	// Đảm bảo params phân trang luôn được gửi
	params := map[string]string{
		"page":  strconv.Itoa(page),
		"limit": strconv.Itoa(limit),
	}

	log.Printf("[FolkForm] Đang gửi request GET pages với phân trang đến FolkForm backend...")
	log.Printf("[FolkForm] Endpoint: /facebook/page/find-with-pagination với params phân trang: page=%d, limit=%d", page, limit)
	result, err = executeGetRequest(client, "/v1/facebook/page/find-with-pagination", params, "")
	if err != nil {
		log.Printf("[FolkForm] LỖI khi lấy danh sách trang Facebook (page=%d, limit=%d): %v", page, limit, err)
	} else {
		log.Printf("[FolkForm] Lấy danh sách trang Facebook thành công với phân trang - page: %d, limit: %d", page, limit)
	}
	return result, err
}

// Hàm FolkForm_UpdatePageAccessToken sẽ gửi yêu cầu cập nhật access token của trang Facebook lên server
// Sử dụng endpoint đặc biệt /facebook/page/update-token thay vì endpoint CRUD thông thường
func FolkForm_UpdatePageAccessToken(page_id string, page_access_token string) (result map[string]interface{}, err error) {
	log.Printf("[FolkForm] Bắt đầu cập nhật page access token - page_id: %s", page_id)

	if err := checkApiToken(); err != nil {
		log.Printf("[FolkForm] LỖI: %v", err)
		return nil, err
	}

	client := createAuthorizedClient(defaultTimeout)
	// Endpoint đặc biệt yêu cầu cả pageId và pageAccessToken trong body
	updateData := map[string]interface{}{
		"pageId":          page_id,
		"pageAccessToken": page_access_token,
	}

	log.Printf("[FolkForm] Đang gửi request PUT page access token đến FolkForm backend...")
	// Sử dụng endpoint đặc biệt /facebook/page/update-token thay vì endpoint CRUD
	result, err = executePutRequest(client, "/v1/facebook/page/update-token", updateData, nil, "Cập nhật page_access_token thành công", "Cập nhật page_access_token thất bại. Thử lại lần thứ", true)
	if err != nil {
		log.Printf("[FolkForm] LỖI khi cập nhật page access token: %v", err)
	} else {
		log.Printf("[FolkForm] Cập nhật page access token thành công - page_id: %s", page_id)
	}
	return result, err
}

// Hàm FolkForm_CreateFbPage sẽ gửi yêu cầu lưu/cập nhật trang Facebook lên server (sử dụng upsert)
// Upsert sẽ tự động insert nếu chưa có, hoặc update nếu đã có dựa trên pageId (unique)
// Lưu ý: Hàm này sẽ lấy page hiện tại trước để giữ lại các field như isSync nếu page đã tồn tại
func FolkForm_CreateFbPage(access_token string, page_data interface{}) (result map[string]interface{}, err error) {
	log.Printf("[FolkForm] Bắt đầu tạo/cập nhật trang Facebook")

	if err := checkApiToken(); err != nil {
		log.Printf("[FolkForm] LỖI: %v", err)
		return nil, err
	}

	client := createAuthorizedClient(longTimeout)

	// Tạo filter cho upsert dựa trên pageId từ panCakeData
	params := make(map[string]string)
	var pageId string

	if pageMap, ok := page_data.(map[string]interface{}); ok {
		// Lấy pageId từ panCakeData (field "id" trong response từ Pancake)
		if id, ok := pageMap["id"].(string); ok && id != "" {
			pageId = id
			filter := `{"pageId":"` + pageId + `"}`
			params["filter"] = filter
			log.Printf("[FolkForm] Tạo filter cho upsert page: %s", filter)
		} else {
			// Fallback: thử tìm pageId trực tiếp
			if id, ok := pageMap["pageId"].(string); ok && id != "" {
				pageId = id
				filter := `{"pageId":"` + pageId + `"}`
				params["filter"] = filter
				log.Printf("[FolkForm] Tạo filter cho upsert page: %s", filter)
			} else {
				log.Printf("[FolkForm] CẢNH BÁO: Không tìm thấy pageId trong panCakeData, upsert có thể không hoạt động đúng")
			}
		}
	} else {
		log.Printf("[FolkForm] CẢNH BÁO: page_data không phải là map, upsert có thể không hoạt động đúng")
	}

	// Lấy page hiện tại để giữ lại các field như isSync, pageAccessToken, etc.
	var existingPageData map[string]interface{}
	if pageId != "" {
		log.Printf("[FolkForm] Lấy thông tin page hiện tại để giữ lại các field không có trong input...")
		existingPage, err := FolkForm_GetFbPageByPageId(pageId)
		if err == nil && existingPage != nil {
			if existingPageDataMap, ok := existingPage["data"].(map[string]interface{}); ok {
				existingPageData = existingPageDataMap
				log.Printf("[FolkForm] Đã lấy được thông tin page hiện tại")
			} else if existingPageArray, ok := existingPage["data"].([]interface{}); ok && len(existingPageArray) > 0 {
				if firstItem, ok := existingPageArray[0].(map[string]interface{}); ok {
					existingPageData = firstItem
					log.Printf("[FolkForm] Đã lấy được thông tin page hiện tại (từ array)")
				}
			}
		} else {
			log.Printf("[FolkForm] Không tìm thấy page hiện tại (có thể là page mới), sẽ tạo mới")
		}
	}

	// Tạo data để gửi, merge với existingPageData để giữ lại các field không có trong input
	data := map[string]interface{}{
		"accessToken": access_token,
		"panCakeData": page_data,
	}

	// Nếu có page hiện tại, merge các field quan trọng vào data để không bị mất
	if existingPageData != nil {
		// Giữ lại các field quan trọng nếu chúng không có trong input
		if isSync, ok := existingPageData["isSync"].(bool); ok {
			data["isSync"] = isSync
			log.Printf("[FolkForm] Giữ lại field isSync: %v", isSync)
		}
		if pageAccessToken, ok := existingPageData["pageAccessToken"].(string); ok && pageAccessToken != "" {
			// Chỉ giữ lại nếu chưa có trong data mới
			if _, exists := data["pageAccessToken"]; !exists {
				data["pageAccessToken"] = pageAccessToken
				log.Printf("[FolkForm] Giữ lại field pageAccessToken")
			}
		}
		// Có thể thêm các field khác cần giữ lại ở đây
		log.Printf("[FolkForm] Đã merge các field từ page hiện tại vào data")
	} else {
		// Nếu là page mới, set giá trị mặc định cho isSync
		data["isSync"] = true
		log.Printf("[FolkForm] Set giá trị mặc định isSync = true cho page mới")
	}

	log.Printf("[FolkForm] Đang gửi request upsert page đến FolkForm backend...")
	// Sử dụng upsert-one để tự động insert hoặc update dựa trên pageId
	result, err = executePostRequest(client, "/v1/facebook/page/upsert-one", data, params, "Gửi trang Facebook thành công", "Gửi trang Facebook thất bại. Thử lại lần thứ", false)
	if err != nil {
		log.Printf("[FolkForm] LỖI khi tạo/cập nhật trang Facebook: %v", err)
	} else {
		log.Printf("[FolkForm] Tạo/cập nhật trang Facebook thành công")
	}
	return result, err
}

// Hàm FolkForm_GetAccessTokens sẽ gửi yêu cầu lấy danh sách access token từ server
// Hàm này sử dụng endpoint phân trang với page và limit
// filter: JSON string của MongoDB filter (optional), ví dụ: `{"system":"Pancake"}`
func FolkForm_GetAccessTokens(page int, limit int, filter string) (result map[string]interface{}, err error) {

	log.Printf("[FolkForm] Bắt đầu lấy danh sách access token với phân trang - page: %d, limit: %d", page, limit)
	if filter != "" {
		log.Printf("[FolkForm] Sử dụng filter: %s", filter)
	}

	if err := checkApiToken(); err != nil {
		log.Printf("[FolkForm] LỖI: %v", err)
		return nil, err
	}

	client := createAuthorizedClient(defaultTimeout)
	// Đảm bảo params phân trang luôn được gửi
	params := map[string]string{
		"page":  strconv.Itoa(page),
		"limit": strconv.Itoa(limit),
	}
	// Thêm filter vào params nếu có
	if filter != "" {
		params["filter"] = filter
	}

	log.Printf("[FolkForm] Đang gửi request GET access tokens với phân trang đến FolkForm backend...")
	log.Printf("[FolkForm] Endpoint: /access-token/find-with-pagination với params phân trang: page=%d, limit=%d", page, limit)
	if filter != "" {
		log.Printf("[FolkForm] Filter: %s", filter)
	}
	result, err = executeGetRequest(client, "/v1/access-token/find-with-pagination", params, "")
	if err != nil {
		log.Printf("[FolkForm] LỖI khi lấy danh sách access token (page=%d, limit=%d): %v", page, limit, err)
	} else {
		log.Printf("[FolkForm] Lấy danh sách access token thành công với phân trang - page: %d, limit: %d", page, limit)
	}
	return result, err
}

// Hàm Firebase_GetIdToken đăng nhập vào Firebase và lấy ID Token
// Sử dụng Firebase REST API để đăng nhập bằng email/password
func Firebase_GetIdToken() (string, error) {
	log.Println("[Firebase] ========================================")
	log.Println("[Firebase] Bắt đầu đăng nhập Firebase...")

	// Kiểm tra cấu hình Firebase
	log.Println("[Firebase] [Bước 0/3] Kiểm tra cấu hình Firebase...")
	log.Printf("[Firebase] [Bước 0/3] Config source: %s", getConfigSource())

	if global.GlobalConfig.FirebaseApiKey == "" {
		log.Println("[Firebase] [Bước 0/3] ❌ LỖI: Firebase API Key chưa được cấu hình")
		log.Println("[Firebase] [Bước 0/3] Vui lòng cấu hình FIREBASE_API_KEY trong env file")
		return "", errors.New("Firebase API Key chưa được cấu hình. Vui lòng cấu hình FIREBASE_API_KEY trong file .env")
	}
	if global.GlobalConfig.FirebaseEmail == "" {
		log.Println("[Firebase] [Bước 0/3] ❌ LỖI: Firebase Email chưa được cấu hình")
		log.Println("[Firebase] [Bước 0/3] Vui lòng cấu hình FIREBASE_EMAIL trong env file")
		return "", errors.New("Firebase Email chưa được cấu hình. Vui lòng cấu hình FIREBASE_EMAIL trong file .env")
	}
	if global.GlobalConfig.FirebasePassword == "" {
		log.Println("[Firebase] [Bước 0/3] ❌ LỖI: Firebase Password chưa được cấu hình")
		log.Println("[Firebase] [Bước 0/3] Vui lòng cấu hình FIREBASE_PASSWORD trong env file")
		return "", errors.New("Firebase Password chưa được cấu hình. Vui lòng cấu hình FIREBASE_PASSWORD trong file .env")
	}

	log.Println("[Firebase] [Bước 0/3] ✅ Cấu hình Firebase đầy đủ")
	log.Printf("[Firebase] [Bước 0/3] Email: %s", global.GlobalConfig.FirebaseEmail)
	log.Printf("[Firebase] [Bước 0/3] API Key: %s...%s (length: %d)",
		global.GlobalConfig.FirebaseApiKey[:min(10, len(global.GlobalConfig.FirebaseApiKey))],
		global.GlobalConfig.FirebaseApiKey[max(0, len(global.GlobalConfig.FirebaseApiKey)-10):],
		len(global.GlobalConfig.FirebaseApiKey))
	log.Printf("[Firebase] [Bước 0/3] Password: %s (length: %d)",
		maskPassword(global.GlobalConfig.FirebasePassword),
		len(global.GlobalConfig.FirebasePassword))

	// Tạo HTTP client cho Firebase
	firebaseBaseURL := "https://identitytoolkit.googleapis.com"
	log.Printf("[Firebase] [Bước 1/3] Tạo HTTP client với base URL: %s", firebaseBaseURL)
	firebaseClient := httpclient.NewHttpClient(firebaseBaseURL, defaultTimeout)

	// Chuẩn bị dữ liệu đăng nhập
	data := map[string]interface{}{
		"email":             global.GlobalConfig.FirebaseEmail,
		"password":          global.GlobalConfig.FirebasePassword,
		"returnSecureToken": true,
	}
	log.Printf("[Firebase] [Bước 2/3] Chuẩn bị dữ liệu đăng nhập - email: %s (password đã ẩn)", global.GlobalConfig.FirebaseEmail)

	// Gọi Firebase REST API để đăng nhập
	endpoint := "/v1/accounts:signInWithPassword?key=" + global.GlobalConfig.FirebaseApiKey
	fullURL := firebaseBaseURL + endpoint
	log.Printf("[Firebase] [Bước 3/3] Gửi POST request đến Firebase API: %s", fullURL)
	log.Printf("[Firebase] [Bước 3/3] Request endpoint: %s", endpoint)

	resp, err := firebaseClient.POST(endpoint, data, nil)
	if err != nil {
		log.Printf("[Firebase] [Bước 3/3] LỖI khi gọi Firebase API: %v", err)
		log.Printf("[Firebase] [Bước 3/3] Request data: email=%s, returnSecureToken=true", global.GlobalConfig.FirebaseEmail)
		return "", errors.New("Lỗi khi gọi Firebase API: " + err.Error())
	}

	log.Printf("[Firebase] [Bước 3/3] Response Status Code: %d", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		// Đọc response body để lấy thông tin lỗi
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Printf("[Firebase] [Bước 3/3] LỖI khi đọc response body: %v", err)
		} else {
			log.Printf("[Firebase] [Bước 3/3] LỖI: Response Body (raw): %s", string(bodyBytes))
		}
		resp.Body.Close()

		var errorResult map[string]interface{}
		if len(bodyBytes) > 0 {
			if err := json.Unmarshal(bodyBytes, &errorResult); err != nil {
				log.Printf("[Firebase] [Bước 3/3] LỖI khi parse JSON: %v", err)
			} else {
				log.Printf("[Firebase] [Bước 3/3] LỖI: Response Body (parsed): %+v", errorResult)
			}
		}

		errorMessage := "Đăng nhập Firebase thất bại"
		isQuotaExceeded := false
		if errorResult["error"] != nil {
			if errorMap, ok := errorResult["error"].(map[string]interface{}); ok {
				if message, ok := errorMap["message"].(string); ok {
					errorMessage = message
					// Kiểm tra xem có phải lỗi QUOTA_EXCEEDED không
					if strings.Contains(message, "QUOTA_EXCEEDED") || strings.Contains(message, "Exceeded quota") {
						isQuotaExceeded = true
						log.Printf("[Firebase] [Bước 3/3] ⚠️  PHÁT HIỆN LỖI QUOTA_EXCEEDED - Firebase đã vượt quá quota verify password")
						log.Printf("[Firebase] [Bước 3/3] ⚠️  Cần đợi %v trước khi thử lại", quotaExceededWait)
					}
				}
				log.Printf("[Firebase] [Bước 3/3] Chi tiết lỗi: %+v", errorMap)
			}
		}
		log.Printf("[Firebase] [Bước 3/3] LỖI: %s", errorMessage)

		// Trả về error với prefix đặc biệt để FolkForm_Login có thể nhận biết
		if isQuotaExceeded {
			return "", errors.New("QUOTA_EXCEEDED: " + errorMessage)
		}
		return "", errors.New(errorMessage)
	}

	// Parse response để lấy ID Token
	var result map[string]interface{}
	if err := httpclient.ParseJSONResponse(resp, &result); err != nil {
		log.Printf("[Firebase] [Bước 3/3] LỖI khi phân tích phản hồi JSON: %v", err)
		// Đọc lại response body để log
		bodyBytes, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr == nil {
			log.Printf("[Firebase] [Bước 3/3] Response Body (raw): %s", string(bodyBytes))
		}
		return "", errors.New("Lỗi khi phân tích phản hồi từ Firebase: " + err.Error())
	}

	log.Printf("[Firebase] [Bước 3/3] Response Body (thành công): có %d keys", len(result))
	log.Printf("[Firebase] [Bước 3/3] Response keys: %+v", getMapKeys(result))

	// Lấy ID Token từ response
	idToken, ok := result["idToken"].(string)
	if !ok || idToken == "" {
		log.Printf("[Firebase] [Bước 3/3] ❌ LỖI: Không tìm thấy idToken trong response")
		log.Printf("[Firebase] [Bước 3/3] Response keys: %+v", getMapKeys(result))
		log.Printf("[Firebase] [Bước 3/3] Response Body: %+v", result)
		log.Println("[Firebase] ========================================")
		return "", errors.New("Không tìm thấy ID Token trong phản hồi từ Firebase")
	}

	log.Println("[Firebase] [Bước 3/3] ✅ Đăng nhập Firebase thành công!")
	log.Printf("[Firebase] [Bước 3/3] ID Token length: %d", len(idToken))
	log.Printf("[Firebase] [Bước 3/3] ID Token preview: %s...%s", idToken[:min(20, len(idToken))], idToken[max(0, len(idToken)-20):])

	// Log thêm thông tin từ response nếu có
	if localId, ok := result["localId"].(string); ok {
		log.Printf("[Firebase] [Bước 3/3] Local ID (Firebase UID): %s", localId)
	}
	if email, ok := result["email"].(string); ok {
		log.Printf("[Firebase] [Bước 3/3] Email: %s", email)
	}
	if expiresIn, ok := result["expiresIn"].(string); ok {
		log.Printf("[Firebase] [Bước 3/3] Token expires in: %s", expiresIn)
	}

	log.Println("[Firebase] ========================================")
	return idToken, nil
}

// Helper function để lấy keys của map
func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// Helper function để lấy min của 2 số
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Helper function để lấy max của 2 số
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Helper function để mask password (ẩn password trong log)
func maskPassword(pwd string) string {
	if len(pwd) == 0 {
		return "(empty)"
	}
	if len(pwd) <= 4 {
		return "****"
	}
	return pwd[:2] + "****" + pwd[len(pwd)-2:]
}

// Helper function để xác định config được đọc từ đâu
func getConfigSource() string {
	// Kiểm tra xem có file .env trong working directory không
	// Nếu có và có giá trị, có thể là từ file .env
	// Nếu không, có thể là từ systemd EnvironmentFile
	// (Đơn giản hóa: chỉ trả về thông tin cơ bản)
	if global.GlobalConfig != nil {
		// Kiểm tra xem có file .env không (đơn giản hóa)
		return "environment variables hoặc file .env"
	}
	return "unknown"
}

// FolkForm_GetRoles lấy danh sách roles của user hiện tại
// Sử dụng endpoint /auth/roles để lấy danh sách roles
// Trả về: []interface{} (danh sách roles), error
func FolkForm_GetRoles() ([]interface{}, error) {
	log.Printf("[FolkForm] Lấy danh sách roles của user")

	if err := checkApiToken(); err != nil {
		log.Printf("[FolkForm] LỖI: %v", err)
		return nil, err
	}

	client := createAuthorizedClient(defaultTimeout)
	result, err := executeGetRequest(client, "/v1/auth/roles", nil, "Lấy danh sách roles thành công")
	if err != nil {
		log.Printf("[FolkForm] LỖI khi lấy danh sách roles: %v", err)
		return nil, err
	}

	var roles []interface{}
	if dataMap, ok := result["data"].(map[string]interface{}); ok {
		// Thử lấy từ data.roles trước
		if rolesArray, ok := dataMap["roles"].([]interface{}); ok {
			roles = rolesArray
		} else if rolesArray, ok := dataMap["data"].([]interface{}); ok {
			// Fallback: thử lấy từ data.data
			roles = rolesArray
		}
	} else if rolesArray, ok := result["data"].([]interface{}); ok {
		// Fallback: thử lấy trực tiếp từ data (array)
		roles = rolesArray
	}

	if len(roles) > 0 {
		log.Printf("[FolkForm] Tìm thấy %d roles", len(roles))
	} else {
		log.Printf("[FolkForm] Không tìm thấy roles nào")
	}

	return roles, nil
}

// Hàm FolkForm_Login để Agent login vào hệ thống bằng Firebase
// Tự động đăng nhập Firebase để lấy ID Token, sau đó dùng token đó để đăng nhập backend
func FolkForm_Login() (result map[string]interface{}, resultError error) {
	log.Println("[FolkForm] [Login] ========================================")
	log.Println("[FolkForm] [Login] Bắt đầu quá trình đăng nhập vào FolkForm backend...")
	log.Printf("[FolkForm] [Login] API Base URL: %s", global.GlobalConfig.ApiBaseUrl)
	log.Printf("[FolkForm] [Login] Agent ID: %s", global.GlobalConfig.AgentId)

	client := httpclient.NewHttpClient(global.GlobalConfig.ApiBaseUrl, defaultTimeout)

	requestCount := 0
	for {
		requestCount++
		log.Printf("[FolkForm] [Login] [Lần thử %d/%d] Bắt đầu quá trình đăng nhập", requestCount, maxRetries)

		if requestCount > maxRetries {
			log.Printf("[FolkForm] [Login] LỖI: Đã thử quá nhiều lần (%d/%d). Thoát vòng lặp.", requestCount, maxRetries)
			return nil, errors.New("Đã thử quá nhiều lần. Thoát vòng lặp.")
		}

		// Sử dụng adaptive rate limiter cho FolkForm
		rateLimiter := apputility.GetFolkFormRateLimiter()
		rateLimiter.Wait()

		// Lấy hardware ID
		log.Printf("[FolkForm] [Login] [Bước 1/3] Lấy Hardware ID...")
		hwid, err := hwid.GenerateHardwareID()
		if err != nil {
			log.Printf("[FolkForm] [Login] [Bước 1/3] LỖI khi lấy Hardware ID: %v", err)
			continue
		}
		log.Printf("[FolkForm] [Login] [Bước 1/3] Hardware ID: %s", hwid)

		// Đăng nhập Firebase để lấy ID Token
		log.Printf("[FolkForm] [Login] [Bước 2/3] Đăng nhập Firebase để lấy ID Token...")
		firebaseIdToken, err := Firebase_GetIdToken()
		if err != nil {
			log.Printf("[FolkForm] [Login] [Bước 2/3] LỖI khi đăng nhập Firebase: %v", err)

			// Kiểm tra xem có phải lỗi QUOTA_EXCEEDED không
			if strings.Contains(err.Error(), "QUOTA_EXCEEDED") {
				log.Printf("[FolkForm] [Login] [Bước 2/3] ⚠️  Firebase đã vượt quá quota verify password")
				log.Printf("[FolkForm] [Login] [Bước 2/3] ⚠️  Đợi %v trước khi thử lại...", quotaExceededWait)
				log.Printf("[FolkForm] [Login] [Bước 2/3] ⚠️  Lưu ý: Quota thường được reset sau một khoảng thời gian (thường là 1 giờ)")
				time.Sleep(quotaExceededWait)
				log.Printf("[FolkForm] [Login] [Bước 2/3] ✅ Đã đợi xong, thử lại...")
				// Reset requestCount để không bị giới hạn bởi maxRetries khi gặp QUOTA_EXCEEDED
				requestCount = 0
			} else {
				// Đối với các lỗi khác, đợi một chút trước khi retry
				log.Printf("[FolkForm] [Login] [Bước 2/3] Đợi %v trước khi thử lại...", firebaseRetryDelay)
				time.Sleep(firebaseRetryDelay)
			}
			continue
		}
		log.Printf("[FolkForm] [Login] [Bước 2/3] Đã lấy được Firebase ID Token (length: %d)", len(firebaseIdToken))

		// Gửi Firebase ID Token và HWID đến endpoint /auth/login/firebase
		data := map[string]interface{}{
			"idToken": firebaseIdToken,
			"hwid":    hwid,
		}
		log.Printf("[FolkForm] [Login] [Bước 3/3] Gửi POST request đăng nhập đến FolkForm backend...")
		log.Printf("[FolkForm] [Login] [Bước 3/3] Endpoint: /v1/auth/login/firebase")
		log.Printf("[FolkForm] [Login] [Bước 3/3] Request data: idToken (length: %d), hwid: %s", len(firebaseIdToken), hwid)

		resp, err := client.POST("/v1/auth/login/firebase", data, nil)
		if err != nil {
			log.Printf("[FolkForm] [Login] [Bước 3/3] LỖI khi gọi API POST: %v", err)
			log.Printf("[FolkForm] [Login] [Bước 3/3] Request endpoint: /auth/login/firebase")
			log.Printf("[FolkForm] [Login] [Bước 3/3] Request data: idToken (length: %d), hwid: %s", len(firebaseIdToken), hwid)
			continue
		}

		statusCode := resp.StatusCode
		log.Printf("[FolkForm] [Login] [Bước 3/3] Response Status Code: %d", statusCode)

		if statusCode != http.StatusOK {
			log.Printf("[FolkForm] [Login] [Bước 3/3] ❌ Đăng nhập backend thất bại (Status: %d)", statusCode)
			// Đọc response body để lấy thông tin lỗi
			bodyBytes, err := io.ReadAll(resp.Body)
			if err != nil {
				log.Printf("[FolkForm] [Login] [Bước 3/3] LỖI khi đọc response body: %v", err)
			} else {
				log.Printf("[FolkForm] [Login] [Bước 3/3] LỖI: Response Body (raw): %s", string(bodyBytes))
			}
			resp.Body.Close()

			var errorResult map[string]interface{}
			if len(bodyBytes) > 0 {
				if err := json.Unmarshal(bodyBytes, &errorResult); err != nil {
					log.Printf("[FolkForm] [Login] [Bước 3/3] LỖI khi parse JSON: %v", err)
				} else {
					log.Printf("[FolkForm] [Login] [Bước 3/3] LỖI: Response Body (parsed): %+v", errorResult)
				}
			}
			// Ghi nhận lỗi để điều chỉnh rate limiter
			var errorCode interface{}
			if len(bodyBytes) > 0 {
				var errorResult map[string]interface{}
				if err := json.Unmarshal(bodyBytes, &errorResult); err == nil {
					if ec, ok := errorResult["error_code"]; ok {
						errorCode = ec
					} else if code, ok := errorResult["code"]; ok {
						errorCode = code
					}
				}
			}
			rateLimiter.RecordFailure(statusCode, errorCode)
			log.Printf("[FolkForm] [Login] [Bước 3/3] Đăng nhập thất bại. Thử lại lần thứ %d", requestCount)
			continue
		}

		var result map[string]interface{}
		if err := httpclient.ParseJSONResponse(resp, &result); err != nil {
			log.Printf("[FolkForm] [Login] [Bước 3/3] LỖI khi phân tích phản hồi JSON: %v", err)
			// Đọc lại response body để log
			bodyBytes, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			if readErr == nil {
				log.Printf("[FolkForm] [Login] [Bước 3/3] Response Body (raw): %s", string(bodyBytes))
			}
			continue
		}

		// Ghi nhận kết quả response để điều chỉnh rate limiter
		success := result["status"] == "success"
		var errorCode interface{}
		if ec, ok := result["error_code"]; ok {
			errorCode = ec
		} else if code, ok := result["code"]; ok {
			errorCode = code
		}
		rateLimiter.RecordResponse(statusCode, success, errorCode)

		log.Printf("[FolkForm] [Login] [Bước 3/3] Response Body: status=%v, có %d keys", result["status"], len(result))
		log.Printf("[FolkForm] [Login] [Bước 3/3] Response keys: %+v", getMapKeys(result))

		if result["status"] == "success" {
			log.Println("[FolkForm] [Login] [Bước 3/3] ✅ Đăng nhập backend thành công!")

			// Lưu token vào biến toàn cục
			if dataMap, ok := result["data"].(map[string]interface{}); ok {
				if token, ok := dataMap["token"].(string); ok {
					global.ApiToken = token
					log.Printf("[FolkForm] [Login] Đã lưu JWT token (length: %d)", len(token))
				} else {
					log.Printf("[FolkForm] [Login] CẢNH BÁO: Không tìm thấy token trong response data")
					log.Printf("[FolkForm] [Login] Response data: %+v", dataMap)
				}

				// QUAN TRỌNG: Kiểm tra xem có field 'id' trong response không (KHÔNG được dùng làm agentId)
				if id, exists := dataMap["id"]; exists {
					log.Printf("[FolkForm] [Login] ⚠️  CẢNH BÁO: Login response có field 'id': %v (KHÔNG được dùng làm agentId)", id)
					log.Printf("[FolkForm] [Login] ⚠️  AgentId đúng phải là: %s (từ ENV, KHÔNG phải từ login response.id)", global.GlobalConfig.AgentId)
				}
				// Kiểm tra xem có field 'agentId' trong response không
				if agentIdFromResponse, exists := dataMap["agentId"]; exists {
					log.Printf("[FolkForm] [Login] Login response có field 'agentId': %v", agentIdFromResponse)
					if agentIdFromResponse != global.GlobalConfig.AgentId {
						log.Printf("[FolkForm] [Login] ⚠️  CẢNH BÁO: agentId từ login response (%v) khác với agentId từ ENV (%s)", agentIdFromResponse, global.GlobalConfig.AgentId)
					}
				}
				// Kiểm tra xem có user.id không
				if user, ok := dataMap["user"].(map[string]interface{}); ok {
					if userId, exists := user["id"]; exists {
						log.Printf("[FolkForm] [Login] ⚠️  CẢNH BÁO: Login response có user.id: %v (KHÔNG được dùng làm agentId)", userId)
						log.Printf("[FolkForm] [Login] ⚠️  AgentId đúng phải là: %s (từ ENV, KHÔNG phải từ login response.user.id)", global.GlobalConfig.AgentId)
					}
				}

				// Lấy role ID đầu tiên nếu có (Organization Context System - Version 3.2)
				// Backend có thể trả về roles trong response hoặc cần gọi API riêng
				if roles, ok := dataMap["roles"].([]interface{}); ok && len(roles) > 0 {
					// Lấy role đầu tiên
					if firstRole, ok := roles[0].(map[string]interface{}); ok {
						if roleId, ok := firstRole["id"].(string); ok && roleId != "" {
							global.ActiveRoleId = roleId
							log.Printf("[FolkForm] [Login] Đã lưu Active Role ID từ login response: %s", roleId)
						} else if roleId, ok := firstRole["roleId"].(string); ok && roleId != "" {
							global.ActiveRoleId = roleId
							log.Printf("[FolkForm] [Login] Đã lưu Active Role ID từ login response: %s", roleId)
						}
					}
				} else if user, ok := dataMap["user"].(map[string]interface{}); ok {
					// Thử lấy từ user.roles
					if roles, ok := user["roles"].([]interface{}); ok && len(roles) > 0 {
						if firstRole, ok := roles[0].(map[string]interface{}); ok {
							if roleId, ok := firstRole["id"].(string); ok && roleId != "" {
								global.ActiveRoleId = roleId
								log.Printf("[FolkForm] [Login] Đã lưu Active Role ID từ user.roles: %s", roleId)
							} else if roleId, ok := firstRole["roleId"].(string); ok && roleId != "" {
								global.ActiveRoleId = roleId
								log.Printf("[FolkForm] [Login] Đã lưu Active Role ID từ user.roles: %s", roleId)
							}
						}
					}
				}
			} else {
				log.Printf("[FolkForm] [Login] CẢNH BÁO: Response data không phải là map")
				log.Printf("[FolkForm] [Login] Response: %+v", result)
			}

			// Nếu chưa có ActiveRoleId, sẽ được lấy sau trong SyncBaseAuth()
			if global.ActiveRoleId == "" {
				log.Printf("[FolkForm] [Login] [Bước 3/3] Chưa có Active Role ID, sẽ lấy sau trong SyncBaseAuth()")
			} else {
				log.Printf("[FolkForm] [Login] [Bước 3/3] Active Role ID: %s", global.ActiveRoleId)
			}

			log.Println("[FolkForm] [Login] ========================================")
			return result, nil
		} else {
			log.Printf("[FolkForm] [Login] [Bước 3/3] ❌ Response status không phải 'success': %v", result["status"])
			if result["message"] != nil {
				log.Printf("[FolkForm] [Login] [Bước 3/3] Response message: %v", result["message"])
			}
			log.Printf("[FolkForm] [Login] [Bước 3/3] Response Body: %+v", result)
		}

		// Kiểm tra lại ở cuối vòng lặp (không cần thiết nhưng giữ để tương thích)
		if requestCount > maxRetries {
			log.Printf("[FolkForm] [Login] LỖI: Đã thử quá nhiều lần. Response: %+v", result)
			return result, errors.New("Đã thử quá nhiều lần. Thoát vòng lặp.")
		}
	}
}

// Hàm Điểm danh sẽ gửi thông tin điểm danh lên server
func FolkForm_CheckIn() (result map[string]interface{}, err error) {
	log.Printf("[FolkForm] Bắt đầu điểm danh - agentId: %s", global.GlobalConfig.AgentId)

	if err := checkApiToken(); err != nil {
		log.Printf("[FolkForm] LỖI: %v", err)
		return nil, err
	}

	client := createAuthorizedClient(defaultTimeout)
	log.Printf("[FolkForm] Đang gửi request POST check-in đến FolkForm backend...")
	// Sử dụng endpoint đúng theo tài liệu: /api/v1/agent/check-in/:id
	result, err = executePostRequest(client, "/v1/agent/check-in/"+global.GlobalConfig.AgentId, nil, nil, "Điểm danh thành công", "Điểm danh thất bại. Thử lại lần thứ", true)
	if err != nil {
		log.Printf("[FolkForm] LỖI khi điểm danh: %v", err)
	} else {
		log.Printf("[FolkForm] Điểm danh thành công - agentId: %s", global.GlobalConfig.AgentId)
	}
	return result, err
}

// Hàm FolkForm_CreateFbPost sẽ gửi yêu cầu tạo/cập nhật post lên server (sử dụng upsert)
// postData: Dữ liệu post từ Pancake API (sẽ được gửi trong panCakeData)
// Backend sẽ tự động extract pageId, postId, insertedAt từ panCakeData
func FolkForm_CreateFbPost(postData interface{}) (result map[string]interface{}, err error) {
	log.Printf("[FolkForm] Bắt đầu tạo/cập nhật post Facebook")

	if err := checkApiToken(); err != nil {
		log.Printf("[FolkForm] LỖI: %v", err)
		return nil, err
	}

	client := createAuthorizedClient(defaultTimeout)

	// Tạo filter cho upsert dựa trên postId từ panCakeData
	params := make(map[string]string)
	var postId string

	if postMap, ok := postData.(map[string]interface{}); ok {
		// Lấy postId từ panCakeData (field "id" trong response từ Pancake)
		if id, ok := postMap["id"].(string); ok && id != "" {
			postId = id
			filter := `{"postId":"` + postId + `"}`
			params["filter"] = filter
			log.Printf("[FolkForm] Tạo filter cho upsert post: %s", filter)
		} else {
			log.Printf("[FolkForm] CẢNH BÁO: Không tìm thấy postId trong panCakeData, upsert có thể không hoạt động đúng")
		}
	} else {
		log.Printf("[FolkForm] CẢNH BÁO: post_data không phải là map, upsert có thể không hoạt động đúng")
	}

	// Tạo data với panCakeData
	data := map[string]interface{}{
		"panCakeData": postData, // Backend sẽ tự động extract pageId, postId, insertedAt
	}

	log.Printf("[FolkForm] Đang gửi request upsert post đến FolkForm backend...")
	// Sử dụng upsert-one để tự động insert hoặc update dựa trên postId
	result, err = executePostRequest(client, "/v1/facebook/post/upsert-one", data, params, "Gửi post thành công", "Gửi post thất bại. Thử lại lần thứ", false)
	if err != nil {
		log.Printf("[FolkForm] LỖI khi tạo/cập nhật post: %v", err)
	} else {
		log.Printf("[FolkForm] Tạo/cập nhật post thành công - postId: %s", postId)
	}
	return result, err
}

// Hàm FolkForm_GetLastPostId lấy postId và insertedAt (milliseconds) của post mới nhất
// Trả về: postId, insertedAtMs (milliseconds), error
func FolkForm_GetLastPostId(pageId string) (postId string, insertedAtMs int64, err error) {
	log.Printf("[FolkForm] Lấy post mới nhất - pageId: %s", pageId)

	if err := checkApiToken(); err != nil {
		return "", 0, err
	}

	client := createAuthorizedClient(defaultTimeout)

	// Query: filter theo pageId, sort theo insertedAt DESC, limit 1
	params := map[string]string{
		"filter":  `{"pageId":"` + pageId + `"}`,
		"options": `{"sort":{"insertedAt":-1},"limit":1}`, // Sort desc (mới nhất trước)
	}

	result, err := executeGetRequest(
		client,
		"/v1/facebook/post/find",
		params,
		"Lấy post mới nhất thành công",
	)

	if err != nil {
		return "", 0, err
	}

	// Parse response
	var items []interface{}
	if dataMap, ok := result["data"].(map[string]interface{}); ok {
		if itemsArray, ok := dataMap["items"].([]interface{}); ok {
			items = itemsArray
		}
	}

	if len(items) == 0 {
		log.Printf("[FolkForm] Không tìm thấy post nào - pageId: %s", pageId)
		return "", 0, nil // Không có post → trả về empty
	}

	// items[0] = post mới nhất (insertedAt lớn nhất)
	firstItem := items[0]
	if post, ok := firstItem.(map[string]interface{}); ok {
		if pid, ok := post["postId"].(string); ok {
			var insertedAt int64 = 0
			if insertedAtFloat, ok := post["insertedAt"].(float64); ok {
				insertedAt = int64(insertedAtFloat)
			}
			log.Printf("[FolkForm] Tìm thấy post mới nhất - postId: %s, insertedAt: %d (ms)", pid, insertedAt)
			return pid, insertedAt, nil
		}
	}

	return "", 0, nil
}

// Hàm FolkForm_GetOldestPostId lấy postId và insertedAt (milliseconds) của post cũ nhất
// Trả về: postId, insertedAtMs (milliseconds), error
func FolkForm_GetOldestPostId(pageId string) (postId string, insertedAtMs int64, err error) {
	log.Printf("[FolkForm] Lấy post cũ nhất - pageId: %s", pageId)

	if err := checkApiToken(); err != nil {
		return "", 0, err
	}

	client := createAuthorizedClient(defaultTimeout)

	// Query: filter theo pageId, sort theo insertedAt ASC, limit 1
	params := map[string]string{
		"filter":  `{"pageId":"` + pageId + `"}`,
		"options": `{"sort":{"insertedAt":1},"limit":1}`, // Sort asc (cũ nhất trước)
	}

	result, err := executeGetRequest(
		client,
		"/v1/facebook/post/find",
		params,
		"Lấy post cũ nhất thành công",
	)

	if err != nil {
		return "", 0, err
	}

	// Parse response
	var items []interface{}
	if dataMap, ok := result["data"].(map[string]interface{}); ok {
		if itemsArray, ok := dataMap["items"].([]interface{}); ok {
			items = itemsArray
		}
	}

	if len(items) == 0 {
		log.Printf("[FolkForm] Không tìm thấy post nào - pageId: %s", pageId)
		return "", 0, nil // Không có post → trả về empty
	}

	// items[0] = post cũ nhất (insertedAt nhỏ nhất)
	firstItem := items[0]
	if post, ok := firstItem.(map[string]interface{}); ok {
		if pid, ok := post["postId"].(string); ok {
			var insertedAt int64 = 0
			if insertedAtFloat, ok := post["insertedAt"].(float64); ok {
				insertedAt = int64(insertedAtFloat)
			}
			log.Printf("[FolkForm] Tìm thấy post cũ nhất - postId: %s, insertedAt: %d (ms)", pid, insertedAt)
			return pid, insertedAt, nil
		}
	}

	return "", 0, nil
}

// FolkForm_UpsertFbCustomer tạo/cập nhật FB customer vào FolkForm
// customerData: Dữ liệu customer từ Pancake API (map[string]interface{})
// Chỉ cần gửi đúng DTO: {panCakeData: customerData}
// Backend sẽ tự động extract dữ liệu từ panCakeData
// Filter: customerId (từ id) - ID để identify customer
func FolkForm_UpsertFbCustomer(customerData interface{}) (result map[string]interface{}, err error) {
	log.Printf("[FolkForm] Bắt đầu upsert FB customer")

	if err := checkApiToken(); err != nil {
		return nil, err
	}

	client := createAuthorizedClient(defaultTimeout)

	// Tạo params với filter cho upsert
	params := map[string]string{}

	// Tạo filter từ customer data để upsert
	// Dùng customerId (từ field "id") - ID để identify customer
	if customerMap, ok := customerData.(map[string]interface{}); ok {
		// Lấy customerId từ field "id" - luôn có trong dữ liệu thực tế
		if customerId, ok := customerMap["id"].(string); ok && customerId != "" {
			filter := fmt.Sprintf(`{"customerId":"%s"}`, customerId)
			params["filter"] = filter
			log.Printf("[FolkForm] Tạo filter cho upsert FB customer: %s", filter)
		} else {
			log.Printf("[FolkForm] CẢNH BÁO: Không tìm thấy id trong customer data")
		}
	}

	// Tạo data đúng DTO: {panCakeData: customerData}
	// Backend sẽ tự động extract dữ liệu từ panCakeData
	data := map[string]interface{}{
		"panCakeData": customerData,
	}

	log.Printf("[FolkForm] Đang gửi request upsert FB customer đến FolkForm backend...")
	result, err = executePostRequest(client, "/v1/fb-customer/upsert-one", data, params, "Gửi FB customer thành công", "Gửi FB customer thất bại. Thử lại lần thứ", false)
	if err != nil {
		log.Printf("[FolkForm] LỖI khi upsert FB customer: %v", err)
	} else {
		log.Printf("[FolkForm] Upsert FB customer thành công")
	}
	return result, err
}

// FolkForm_GetLastFbCustomerUpdatedAt lấy updatedAt (Unix timestamp giây) của FB customer cập nhật gần nhất
// Trả về: updatedAt (seconds), error
func FolkForm_GetLastFbCustomerUpdatedAt(pageId string) (updatedAt int64, err error) {
	log.Printf("[FolkForm] Lấy FB customer cập nhật gần nhất - pageId: %s", pageId)

	if err := checkApiToken(); err != nil {
		return 0, err
	}

	client := createAuthorizedClient(defaultTimeout)

	// Query: filter theo pageId, sort theo updatedAt DESC, limit 1
	params := map[string]string{
		"filter":  `{"pageId":"` + pageId + `"}`,
		"options": `{"sort":{"updatedAt":-1},"limit":1}`, // Sort desc (mới nhất trước)
	}

	result, err := executeGetRequest(
		client,
		"/v1/fb-customer/find",
		params,
		"Lấy FB customer cập nhật gần nhất thành công",
	)

	if err != nil {
		return 0, err
	}

	// Parse response
	var items []interface{}
	if dataMap, ok := result["data"].(map[string]interface{}); ok {
		if itemsArray, ok := dataMap["items"].([]interface{}); ok {
			items = itemsArray
		}
	}

	if len(items) == 0 {
		log.Printf("[FolkForm] Không tìm thấy FB customer nào - pageId: %s", pageId)
		return 0, nil // Không có customer → trả về 0
	}

	// items[0] = customer cập nhật gần nhất (updatedAt lớn nhất)
	firstItem := items[0]
	if customer, ok := firstItem.(map[string]interface{}); ok {
		var updatedAtFloat float64 = 0
		if ua, ok := customer["updatedAt"].(float64); ok {
			updatedAtFloat = ua
		}
		// Convert từ milliseconds sang seconds
		updatedAtSeconds := int64(updatedAtFloat) / 1000
		log.Printf("[FolkForm] Tìm thấy FB customer cập nhật gần nhất - updatedAt: %d (seconds)", updatedAtSeconds)
		return updatedAtSeconds, nil
	}

	return 0, nil
}

// FolkForm_GetOldestFbCustomerUpdatedAt lấy updatedAt (Unix timestamp giây) của FB customer cập nhật cũ nhất
// Trả về: updatedAt (seconds), error
func FolkForm_GetOldestFbCustomerUpdatedAt(pageId string) (updatedAt int64, err error) {
	log.Printf("[FolkForm] Lấy FB customer cập nhật cũ nhất - pageId: %s", pageId)

	if err := checkApiToken(); err != nil {
		return 0, err
	}

	client := createAuthorizedClient(defaultTimeout)

	// Query: filter theo pageId, sort theo updatedAt ASC, limit 1
	params := map[string]string{
		"filter":  `{"pageId":"` + pageId + `"}`,
		"options": `{"sort":{"updatedAt":1},"limit":1}`, // Sort asc (cũ nhất trước)
	}

	result, err := executeGetRequest(
		client,
		"/v1/fb-customer/find",
		params,
		"Lấy FB customer cập nhật cũ nhất thành công",
	)

	if err != nil {
		return 0, err
	}

	// Parse response
	var items []interface{}
	if dataMap, ok := result["data"].(map[string]interface{}); ok {
		if itemsArray, ok := dataMap["items"].([]interface{}); ok {
			items = itemsArray
		}
	}

	if len(items) == 0 {
		log.Printf("[FolkForm] Không tìm thấy FB customer nào - pageId: %s", pageId)
		return 0, nil // Không có customer → trả về 0
	}

	// items[0] = customer cập nhật cũ nhất (updatedAt nhỏ nhất)
	firstItem := items[0]
	if customer, ok := firstItem.(map[string]interface{}); ok {
		var updatedAtFloat float64 = 0
		if ua, ok := customer["updatedAt"].(float64); ok {
			updatedAtFloat = ua
		}
		// Convert từ milliseconds sang seconds
		updatedAtSeconds := int64(updatedAtFloat) / 1000
		log.Printf("[FolkForm] Tìm thấy FB customer cập nhật cũ nhất - updatedAt: %d (seconds)", updatedAtSeconds)
		return updatedAtSeconds, nil
	}

	return 0, nil
}

// FolkForm_UpsertCustomerFromPos tạo/cập nhật POS customer vào FolkForm
// customerData: Dữ liệu customer từ Pancake POS API (map[string]interface{})
// Chỉ cần gửi đúng format: {posData: customerData}
// Server sẽ tự động extract dữ liệu từ posData
// Filter: customerId (từ id) - ID để identify customer
// Trả về: map[string]interface{} response từ FolkForm
func FolkForm_UpsertCustomerFromPos(customerData interface{}) (result map[string]interface{}, err error) {
	log.Printf("[FolkForm] Bắt đầu upsert POS customer")

	if err := checkApiToken(); err != nil {
		return nil, err
	}

	client := createAuthorizedClient(defaultTimeout)

	// Tạo params với filter cho upsert
	params := map[string]string{}

	// Tạo filter từ customer data để upsert
	// Dùng customerId (từ field "id") - ID chung để identify customer từ cả 2 nguồn
	if customerMap, ok := customerData.(map[string]interface{}); ok {
		// Lấy customerId từ field "id" - luôn có trong dữ liệu thực tế
		if customerId, ok := customerMap["id"].(string); ok && customerId != "" {
			filter := fmt.Sprintf(`{"customerId":"%s"}`, customerId)
			params["filter"] = filter
			log.Printf("[FolkForm] Tạo filter cho upsert POS customer: %s", filter)
		} else {
			log.Printf("[FolkForm] CẢNH BÁO: Không tìm thấy id trong customer data từ POS, upsert có thể không hoạt động đúng")
		}
	}

	// Tạo data đúng DTO: {posData: customerData}
	// Server sẽ tự động:
	// - Extract các field: customerId (từ posData.id), posCustomerId, name, phoneNumbers, email, point, etc.
	// - Conflict resolution: Ưu tiên POS (priority=1) hơn Pancake (priority=2) cho thông tin cá nhân
	data := map[string]interface{}{
		"posData": customerData, // Gửi nguyên dữ liệu từ POS API
	}

	log.Printf("[FolkForm] Đang gửi request upsert POS customer đến FolkForm backend...")
	result, err = executePostRequest(client, "/v1/pc-pos-customer/upsert-one", data, params, "Gửi POS customer thành công", "Gửi POS customer thất bại. Thử lại lần thứ", false)
	if err != nil {
		log.Printf("[FolkForm] LỖI khi upsert POS customer: %v", err)
	} else {
		log.Printf("[FolkForm] Upsert POS customer thành công")
	}
	return result, err
}

// FolkForm_GetLastPosCustomerUpdatedAt lấy updatedAt (Unix timestamp giây) của POS customer cập nhật gần nhất
// Trả về: updatedAt (seconds), error
func FolkForm_GetLastPosCustomerUpdatedAt(shopId int) (updatedAt int64, err error) {
	log.Printf("[FolkForm] Lấy POS customer cập nhật gần nhất - shopId: %d", shopId)

	if err := checkApiToken(); err != nil {
		return 0, err
	}

	client := createAuthorizedClient(defaultTimeout)

	// Query: filter theo shopId, sort theo updatedAt DESC, limit 1
	params := map[string]string{
		"filter":  fmt.Sprintf(`{"shopId":%d}`, shopId),
		"options": `{"sort":{"updatedAt":-1},"limit":1}`, // Sort desc (mới nhất trước)
	}

	result, err := executeGetRequest(
		client,
		"/v1/pc-pos-customer/find",
		params,
		"Lấy POS customer cập nhật gần nhất thành công",
	)

	if err != nil {
		return 0, err
	}

	// Parse response
	var items []interface{}
	if dataMap, ok := result["data"].(map[string]interface{}); ok {
		if itemsArray, ok := dataMap["items"].([]interface{}); ok {
			items = itemsArray
		}
	}

	if len(items) == 0 {
		log.Printf("[FolkForm] Không tìm thấy POS customer nào - shopId: %d", shopId)
		return 0, nil // Không có customer → trả về 0
	}

	// items[0] = customer cập nhật gần nhất (updatedAt lớn nhất)
	firstItem := items[0]
	if customer, ok := firstItem.(map[string]interface{}); ok {
		var updatedAtFloat float64 = 0
		if ua, ok := customer["updatedAt"].(float64); ok {
			updatedAtFloat = ua
		}
		// Convert từ milliseconds sang seconds
		updatedAtSeconds := int64(updatedAtFloat) / 1000
		log.Printf("[FolkForm] Tìm thấy POS customer cập nhật gần nhất - updatedAt: %d (seconds)", updatedAtSeconds)
		return updatedAtSeconds, nil
	}

	return 0, nil
}

// FolkForm_GetOldestPosCustomerUpdatedAt lấy updatedAt (Unix timestamp giây) của POS customer cập nhật cũ nhất
// Trả về: updatedAt (seconds), error
func FolkForm_GetOldestPosCustomerUpdatedAt(shopId int) (updatedAt int64, err error) {
	log.Printf("[FolkForm] Lấy POS customer cập nhật cũ nhất - shopId: %d", shopId)

	if err := checkApiToken(); err != nil {
		return 0, err
	}

	client := createAuthorizedClient(defaultTimeout)

	// Query: filter theo shopId, sort theo updatedAt ASC, limit 1
	params := map[string]string{
		"filter":  fmt.Sprintf(`{"shopId":%d}`, shopId),
		"options": `{"sort":{"updatedAt":1},"limit":1}`, // Sort asc (cũ nhất trước)
	}

	result, err := executeGetRequest(
		client,
		"/v1/pc-pos-customer/find",
		params,
		"Lấy POS customer cập nhật cũ nhất thành công",
	)

	if err != nil {
		return 0, err
	}

	// Parse response
	var items []interface{}
	if dataMap, ok := result["data"].(map[string]interface{}); ok {
		if itemsArray, ok := dataMap["items"].([]interface{}); ok {
			items = itemsArray
		}
	}

	if len(items) == 0 {
		log.Printf("[FolkForm] Không tìm thấy POS customer nào - shopId: %d", shopId)
		return 0, nil // Không có customer → trả về 0
	}

	// items[0] = customer cập nhật cũ nhất (updatedAt nhỏ nhất)
	firstItem := items[0]
	if customer, ok := firstItem.(map[string]interface{}); ok {
		var updatedAtFloat float64 = 0
		if ua, ok := customer["updatedAt"].(float64); ok {
			updatedAtFloat = ua
		}
		// Convert từ milliseconds sang seconds
		updatedAtSeconds := int64(updatedAtFloat) / 1000
		log.Printf("[FolkForm] Tìm thấy POS customer cập nhật cũ nhất - updatedAt: %d (seconds)", updatedAtSeconds)
		return updatedAtSeconds, nil
	}

	return 0, nil
}

// FolkForm_UpsertShop tạo/cập nhật shop trong FolkForm
// shopData: Dữ liệu shop từ Pancake POS API (map[string]interface{})
// Trả về: map[string]interface{} response từ FolkForm
func FolkForm_UpsertShop(shopData interface{}) (result map[string]interface{}, err error) {
	log.Printf("[FolkForm] Bắt đầu tạo/cập nhật shop")

	if err := checkApiToken(); err != nil {
		return nil, err
	}

	client := createAuthorizedClient(defaultTimeout)

	// Tạo params với filter cho upsert
	params := map[string]string{}

	// Tạo filter từ shop data để upsert
	// Filter dùng shopId (integer) được trích xuất từ field "id" của shop mà Pancake POS trả về - BẮT BUỘC phải có
	if shopMap, ok := shopData.(map[string]interface{}); ok {
		// Lấy shopId từ field "id" của shop data từ Pancake POS
		if shopIdRaw, ok := shopMap["id"]; ok {
			var shopId int64
			switch v := shopIdRaw.(type) {
			case float64:
				shopId = int64(v)
			case int:
				shopId = int64(v)
			case int64:
				shopId = v
			default:
				log.Printf("[FolkForm] LỖI: shopId không phải là số: %T (giá trị: %v)", shopIdRaw, shopIdRaw)
				return nil, fmt.Errorf("shopId không phải là số: %T", shopIdRaw)
			}
			if shopId > 0 {
				filter := fmt.Sprintf(`{"shopId":%d}`, shopId)
				params["filter"] = filter
				log.Printf("[FolkForm] Tạo filter cho upsert shop (shopId được trích xuất từ field 'id' của Pancake POS): %s", filter)
			} else {
				log.Printf("[FolkForm] LỖI: shopId phải lớn hơn 0, nhận được: %d", shopId)
				return nil, fmt.Errorf("shopId phải lớn hơn 0, nhận được: %d", shopId)
			}
		} else {
			log.Printf("[FolkForm] LỖI: Không tìm thấy field 'id' trong shop data từ Pancake POS, không thể upsert")
			return nil, errors.New("Không tìm thấy field 'id' trong shop data")
		}
	} else {
		log.Printf("[FolkForm] LỖI: shopData không phải là map[string]interface{}")
		return nil, errors.New("shopData không phải là map[string]interface{}")
	}

	// Tạo data đúng DTO: ShopCreateInput {panCakeData: shopData}
	// Backend sẽ tự động extract dữ liệu từ panCakeData
	data := map[string]interface{}{
		"panCakeData": shopData,
	}

	log.Printf("[FolkForm] Đang gửi request upsert shop đến FolkForm backend...")
	result, err = executePostRequest(client, "/v1/pancake-pos/shop/upsert-one", data, params, "Gửi shop thành công", "Gửi shop thất bại. Thử lại lần thứ", false)
	if err != nil {
		log.Printf("[FolkForm] LỖI khi tạo/cập nhật shop: %v", err)
	} else {
		log.Printf("[FolkForm] Tạo/cập nhật shop thành công")
	}
	return result, err
}

// FolkForm_UpsertWarehouse tạo/cập nhật warehouse trong FolkForm
// warehouseData: Dữ liệu warehouse từ Pancake POS API (map[string]interface{})
// Trả về: map[string]interface{} response từ FolkForm
func FolkForm_UpsertWarehouse(warehouseData interface{}) (result map[string]interface{}, err error) {
	log.Printf("[FolkForm] Bắt đầu tạo/cập nhật warehouse")

	if err := checkApiToken(); err != nil {
		return nil, err
	}

	client := createAuthorizedClient(defaultTimeout)

	// Tạo params với filter cho upsert
	params := map[string]string{}

	// Tạo filter từ warehouse data để upsert
	// Filter dùng warehouseId (UUID string) được trích xuất từ field "id" của warehouse mà Pancake POS trả về - BẮT BUỘC phải có
	if warehouseMap, ok := warehouseData.(map[string]interface{}); ok {
		// Log warehouse data để debug
		log.Printf("[FolkForm] Warehouse data keys: %v", getMapKeys(warehouseMap))
		if idRaw, exists := warehouseMap["id"]; exists {
			log.Printf("[FolkForm] Field 'id' tồn tại - giá trị: %v, type: %T", idRaw, idRaw)
		} else {
			log.Printf("[FolkForm] CẢNH BÁO: Field 'id' không tồn tại trong warehouse data")
		}

		// Lấy warehouseId từ field "id" của warehouse data từ Pancake POS (UUID string)
		if warehouseId, ok := warehouseMap["id"].(string); ok && warehouseId != "" {
			filter := fmt.Sprintf(`{"warehouseId":"%s"}`, warehouseId)
			params["filter"] = filter
			log.Printf("[FolkForm] Tạo filter cho upsert warehouse (warehouseId được trích xuất từ field 'id' của Pancake POS): %s", filter)
		} else {
			// Kiểm tra xem có phải là số không (trường hợp id là số)
			if idRaw, ok := warehouseMap["id"]; ok {
				// Thử convert sang string nếu là số
				var warehouseIdStr string
				switch v := idRaw.(type) {
				case string:
					warehouseIdStr = v
				case float64:
					warehouseIdStr = fmt.Sprintf("%.0f", v)
				case int:
					warehouseIdStr = strconv.Itoa(v)
				case int64:
					warehouseIdStr = strconv.FormatInt(v, 10)
				default:
					log.Printf("[FolkForm] LỖI: warehouseId không thể convert sang string, type: %T, giá trị: %v", idRaw, idRaw)
					return nil, fmt.Errorf("warehouseId không thể convert sang string, type: %T", idRaw)
				}
				if warehouseIdStr != "" {
					filter := fmt.Sprintf(`{"warehouseId":"%s"}`, warehouseIdStr)
					params["filter"] = filter
					log.Printf("[FolkForm] Tạo filter cho upsert warehouse (warehouseId được convert từ %T sang string): %s", idRaw, filter)
				} else {
					log.Printf("[FolkForm] LỖI: warehouseId rỗng sau khi convert")
					return nil, errors.New("warehouseId rỗng sau khi convert")
				}
			} else {
				log.Printf("[FolkForm] LỖI: Không tìm thấy field 'id' trong warehouse data, không thể upsert")
				log.Printf("[FolkForm] Warehouse data: %+v", warehouseMap)
				return nil, errors.New("Không tìm thấy field 'id' trong warehouse data")
			}
		}
	} else {
		log.Printf("[FolkForm] LỖI: warehouseData không phải là map[string]interface{}, type: %T", warehouseData)
		return nil, errors.New("warehouseData không phải là map[string]interface{}")
	}

	// Tạo data đúng DTO: WarehouseCreateInput {panCakeData: warehouseData}
	// Backend sẽ tự động extract dữ liệu từ panCakeData
	data := map[string]interface{}{
		"panCakeData": warehouseData,
	}

	log.Printf("[FolkForm] Đang gửi request upsert warehouse đến FolkForm backend...")
	result, err = executePostRequest(client, "/v1/pancake-pos/warehouse/upsert-one", data, params, "Gửi warehouse thành công", "Gửi warehouse thất bại. Thử lại lần thứ", false)
	if err != nil {
		log.Printf("[FolkForm] LỖI khi tạo/cập nhật warehouse: %v", err)
	} else {
		log.Printf("[FolkForm] Tạo/cập nhật warehouse thành công")
	}
	return result, err
}

// FolkForm_UpsertProductFromPos tạo/cập nhật product trong FolkForm
// productData: Dữ liệu product từ Pancake POS API (map[string]interface{})
// shopId: ID của shop (integer) - được truyền từ context vì product data không có shop_id
// Trả về: map[string]interface{} response từ FolkForm
func FolkForm_UpsertProductFromPos(productData interface{}, shopId int) (result map[string]interface{}, err error) {
	log.Printf("[FolkForm] Bắt đầu tạo/cập nhật product")

	if err := checkApiToken(); err != nil {
		return nil, err
	}

	client := createAuthorizedClient(defaultTimeout)

	// Tạo params với filter cho upsert
	params := map[string]string{}

	// Tạo filter từ product data để upsert
	// Lưu ý: Product từ Pancake POS có id là UUID string, không phải số
	// Backend sẽ tự extract productId từ posData.id (UUID), nhưng filter cần dùng UUID string
	// shopId được truyền từ parameter (vì product data không có shop_id)
	if productMap, ok := productData.(map[string]interface{}); ok {
		// Đảm bảo shop_id có trong product data
		productMap["shop_id"] = shopId

		// Lấy productId từ field "id" (UUID string)
		// Backend sẽ tự extract productId từ UUID, nhưng filter cần dùng UUID string
		var productIdStr string
		var hasProductId bool

		if productIdRaw, ok := productMap["id"]; ok {
			switch v := productIdRaw.(type) {
			case string:
				productIdStr = v
				hasProductId = true
				log.Printf("[FolkForm] Tìm thấy productId (UUID) từ field 'id': %s", productIdStr)
			case float64:
				// Nếu là số, convert sang string
				productIdStr = fmt.Sprintf("%.0f", v)
				hasProductId = true
				log.Printf("[FolkForm] Tìm thấy productId (số) từ field 'id', convert sang string: %s", productIdStr)
			case int:
				productIdStr = strconv.Itoa(v)
				hasProductId = true
				log.Printf("[FolkForm] Tìm thấy productId (số) từ field 'id', convert sang string: %s", productIdStr)
			case int64:
				productIdStr = strconv.FormatInt(v, 10)
				hasProductId = true
				log.Printf("[FolkForm] Tìm thấy productId (số) từ field 'id', convert sang string: %s", productIdStr)
			default:
				log.Printf("[FolkForm] LỖI: productId có type không hỗ trợ: %T (giá trị: %v)", productIdRaw, productIdRaw)
			}
		}

		if hasProductId && productIdStr != "" && shopId > 0 {
			// Filter dùng productId và shopId
			// Lưu ý: Pancake POS trả về product id là UUID string, không phải số
			// Backend sẽ tự extract productId từ UUID trong posData.id và convert sang số (nếu cần)
			// Nhưng trong filter, có thể cần dùng UUID string hoặc số tùy backend implementation
			// Thử dùng UUID string trước, nếu không được thì backend sẽ tự xử lý
			// Hoặc có thể backend sẽ match theo UUID trong posData và extract productId
			filter := fmt.Sprintf(`{"productId":"%s","shopId":%d}`, productIdStr, shopId)
			params["filter"] = filter
			log.Printf("[FolkForm] Tạo filter cho upsert product (productId là UUID string, shopId là số): %s", filter)
		} else {
			log.Printf("[FolkForm] LỖI: Không tìm thấy hoặc không hợp lệ productId hoặc shopId")
			if !hasProductId {
				log.Printf("[FolkForm] LỖI: Không tìm thấy field 'id' trong product data. Các field có sẵn: %v", getMapKeys(productMap))
			}
			if shopId <= 0 {
				log.Printf("[FolkForm] LỖI: shopId không hợp lệ: %d", shopId)
			}
			return nil, errors.New("Không tìm thấy productId hoặc shopId không hợp lệ")
		}
	} else {
		log.Printf("[FolkForm] LỖI: productData không phải là map[string]interface{}")
		return nil, errors.New("productData không phải là map[string]interface{}")
	}

	// Tạo data đúng DTO: {posData: productData}
	// Backend sẽ tự động extract dữ liệu từ posData
	data := map[string]interface{}{
		"posData": productData,
	}

	log.Printf("[FolkForm] Đang gửi request upsert product đến FolkForm backend...")
	result, err = executePostRequest(client, "/v1/pancake-pos/product/upsert-one", data, params, "Gửi product thành công", "Gửi product thất bại. Thử lại lần thứ", false)
	if err != nil {
		log.Printf("[FolkForm] LỖI khi tạo/cập nhật product: %v", err)
	} else {
		log.Printf("[FolkForm] Tạo/cập nhật product thành công")
	}
	return result, err
}

// FolkForm_UpsertVariationFromPos tạo/cập nhật variation trong FolkForm
// variationData: Dữ liệu variation từ Pancake POS API (map[string]interface{})
// Trả về: map[string]interface{} response từ FolkForm
func FolkForm_UpsertVariationFromPos(variationData interface{}) (result map[string]interface{}, err error) {
	log.Printf("[FolkForm] Bắt đầu tạo/cập nhật variation")

	if err := checkApiToken(); err != nil {
		return nil, err
	}

	client := createAuthorizedClient(defaultTimeout)

	// Tạo params với filter cho upsert
	params := map[string]string{}

	// Tạo filter từ variation data để upsert
	// Filter dùng variationId (UUID string) được trích xuất từ field "id" của variation mà Pancake POS trả về - BẮT BUỘC phải có
	if variationMap, ok := variationData.(map[string]interface{}); ok {
		// Lấy variationId từ field "id" của variation data từ Pancake POS (UUID string)
		if variationId, ok := variationMap["id"].(string); ok && variationId != "" {
			filter := fmt.Sprintf(`{"variationId":"%s"}`, variationId)
			params["filter"] = filter
			log.Printf("[FolkForm] Tạo filter cho upsert variation (variationId được trích xuất từ field 'id' của Pancake POS): %s", filter)
		} else {
			// Kiểm tra xem có phải là số không (trường hợp id là số)
			if idRaw, ok := variationMap["id"]; ok {
				// Thử convert sang string nếu là số
				var variationIdStr string
				switch v := idRaw.(type) {
				case string:
					variationIdStr = v
				case float64:
					variationIdStr = fmt.Sprintf("%.0f", v)
				case int:
					variationIdStr = strconv.Itoa(v)
				case int64:
					variationIdStr = strconv.FormatInt(v, 10)
				default:
					log.Printf("[FolkForm] LỖI: variationId không thể convert sang string, type: %T, giá trị: %v", idRaw, idRaw)
					return nil, fmt.Errorf("variationId không thể convert sang string, type: %T", idRaw)
				}
				if variationIdStr != "" {
					filter := fmt.Sprintf(`{"variationId":"%s"}`, variationIdStr)
					params["filter"] = filter
					log.Printf("[FolkForm] Tạo filter cho upsert variation (variationId được convert từ %T sang string): %s", idRaw, filter)
				} else {
					log.Printf("[FolkForm] LỖI: variationId rỗng sau khi convert")
					return nil, errors.New("variationId rỗng sau khi convert")
				}
			} else {
				log.Printf("[FolkForm] LỖI: Không tìm thấy field 'id' trong variation data, không thể upsert")
				log.Printf("[FolkForm] Variation data: %+v", variationMap)
				return nil, errors.New("Không tìm thấy field 'id' trong variation data")
			}
		}
	} else {
		log.Printf("[FolkForm] LỖI: variationData không phải là map[string]interface{}, type: %T", variationData)
		return nil, errors.New("variationData không phải là map[string]interface{}")
	}

	// Tạo data đúng DTO: {posData: variationData}
	// Backend sẽ tự động extract dữ liệu từ posData
	data := map[string]interface{}{
		"posData": variationData,
	}

	log.Printf("[FolkForm] Đang gửi request upsert variation đến FolkForm backend...")
	result, err = executePostRequest(client, "/v1/pancake-pos/variation/upsert-one", data, params, "Gửi variation thành công", "Gửi variation thất bại. Thử lại lần thứ", false)
	if err != nil {
		log.Printf("[FolkForm] LỖI khi tạo/cập nhật variation: %v", err)
	} else {
		log.Printf("[FolkForm] Tạo/cập nhật variation thành công")
	}
	return result, err
}

// FolkForm_UpsertCategoryFromPos tạo/cập nhật category trong FolkForm
// categoryData: Dữ liệu category từ Pancake POS API (map[string]interface{})
// Trả về: map[string]interface{} response từ FolkForm
func FolkForm_UpsertCategoryFromPos(categoryData interface{}) (result map[string]interface{}, err error) {
	log.Printf("[FolkForm] Bắt đầu tạo/cập nhật category")

	if err := checkApiToken(); err != nil {
		return nil, err
	}

	client := createAuthorizedClient(defaultTimeout)

	// Tạo params với filter cho upsert
	params := map[string]string{}

	// Tạo filter từ category data để upsert
	// Filter dùng categoryId và shopId được trích xuất từ field "id" và "shop_id" của category mà Pancake POS trả về - BẮT BUỘC phải có
	if categoryMap, ok := categoryData.(map[string]interface{}); ok {
		// Lấy categoryId từ field "id" của category data từ Pancake POS
		var categoryId int64
		var shopId int64
		var hasCategoryId, hasShopId bool

		if categoryIdRaw, ok := categoryMap["id"]; ok {
			switch v := categoryIdRaw.(type) {
			case float64:
				categoryId = int64(v)
				hasCategoryId = true
			case int:
				categoryId = int64(v)
				hasCategoryId = true
			case int64:
				categoryId = v
				hasCategoryId = true
			default:
				log.Printf("[FolkForm] LỖI: categoryId không phải là số: %T (giá trị: %v)", categoryIdRaw, categoryIdRaw)
			}
		}

		if shopIdRaw, ok := categoryMap["shop_id"]; ok {
			switch v := shopIdRaw.(type) {
			case float64:
				shopId = int64(v)
				hasShopId = true
			case int:
				shopId = int64(v)
				hasShopId = true
			case int64:
				shopId = v
				hasShopId = true
			default:
				log.Printf("[FolkForm] LỖI: shopId không phải là số: %T (giá trị: %v)", shopIdRaw, shopIdRaw)
			}
		}

		if hasCategoryId && hasShopId && categoryId > 0 && shopId > 0 {
			filter := fmt.Sprintf(`{"categoryId":%d,"shopId":%d}`, categoryId, shopId)
			params["filter"] = filter
			log.Printf("[FolkForm] Tạo filter cho upsert category (categoryId và shopId được trích xuất từ Pancake POS): %s", filter)
		} else {
			log.Printf("[FolkForm] LỖI: Không tìm thấy hoặc không hợp lệ field 'id' hoặc 'shop_id' trong category data từ Pancake POS, không thể upsert")
			if !hasCategoryId {
				log.Printf("[FolkForm] LỖI: Không tìm thấy field 'id' trong category data")
			}
			if !hasShopId {
				log.Printf("[FolkForm] LỖI: Không tìm thấy field 'shop_id' trong category data")
			}
			return nil, errors.New("Không tìm thấy field 'id' hoặc 'shop_id' trong category data")
		}
	} else {
		log.Printf("[FolkForm] LỖI: categoryData không phải là map[string]interface{}")
		return nil, errors.New("categoryData không phải là map[string]interface{}")
	}

	// Tạo data đúng DTO: {posData: categoryData}
	// Backend sẽ tự động extract dữ liệu từ posData
	data := map[string]interface{}{
		"posData": categoryData,
	}

	log.Printf("[FolkForm] Đang gửi request upsert category đến FolkForm backend...")
	result, err = executePostRequest(client, "/v1/pancake-pos/category/upsert-one", data, params, "Gửi category thành công", "Gửi category thất bại. Thử lại lần thứ", false)
	if err != nil {
		log.Printf("[FolkForm] LỖI khi tạo/cập nhật category: %v", err)
	} else {
		log.Printf("[FolkForm] Tạo/cập nhật category thành công")
	}
	return result, err
}

// FolkForm_CreatePcPosOrder tạo/cập nhật order trong FolkForm
// orderData: Dữ liệu order từ Pancake POS API (map[string]interface{})
// Trả về: map[string]interface{} response từ FolkForm
func FolkForm_CreatePcPosOrder(orderData interface{}) (result map[string]interface{}, err error) {
	log.Printf("[FolkForm] Bắt đầu tạo/cập nhật order")

	if err := checkApiToken(); err != nil {
		return nil, err
	}

	client := createAuthorizedClient(defaultTimeout)

	// Tạo params với filter cho upsert
	params := map[string]string{}

	// Tạo filter từ order data để upsert
	// Filter dùng orderId và shopId được trích xuất từ field "id" và "shop_id" của order mà Pancake POS trả về - BẮT BUỘC phải có
	if orderMap, ok := orderData.(map[string]interface{}); ok {
		// Lấy orderId từ field "id" của order data từ Pancake POS
		var orderId int64
		var shopId int64
		var hasOrderId, hasShopId bool

		if orderIdRaw, ok := orderMap["id"]; ok {
			switch v := orderIdRaw.(type) {
			case float64:
				orderId = int64(v)
				hasOrderId = true
			case int:
				orderId = int64(v)
				hasOrderId = true
			case int64:
				orderId = v
				hasOrderId = true
			default:
				log.Printf("[FolkForm] LỖI: orderId không phải là số: %T (giá trị: %v)", orderIdRaw, orderIdRaw)
			}
		}

		if shopIdRaw, ok := orderMap["shop_id"]; ok {
			switch v := shopIdRaw.(type) {
			case float64:
				shopId = int64(v)
				hasShopId = true
			case int:
				shopId = int64(v)
				hasShopId = true
			case int64:
				shopId = v
				hasShopId = true
			default:
				log.Printf("[FolkForm] LỖI: shopId không phải là số: %T (giá trị: %v)", shopIdRaw, shopIdRaw)
			}
		}

		if hasOrderId && hasShopId && orderId > 0 && shopId > 0 {
			filter := fmt.Sprintf(`{"orderId":%d,"shopId":%d}`, orderId, shopId)
			params["filter"] = filter
			log.Printf("[FolkForm] Tạo filter cho upsert order (orderId và shopId được trích xuất từ Pancake POS): %s", filter)
		} else {
			log.Printf("[FolkForm] LỖI: Không tìm thấy hoặc không hợp lệ field 'id' hoặc 'shop_id' trong order data từ Pancake POS, không thể upsert")
			if !hasOrderId {
				log.Printf("[FolkForm] LỖI: Không tìm thấy field 'id' trong order data")
			}
			if !hasShopId {
				log.Printf("[FolkForm] LỖI: Không tìm thấy field 'shop_id' trong order data")
			}
			return nil, errors.New("Không tìm thấy field 'id' hoặc 'shop_id' trong order data")
		}
	} else {
		log.Printf("[FolkForm] LỖI: orderData không phải là map[string]interface{}")
		return nil, errors.New("orderData không phải là map[string]interface{}")
	}

	// Tạo data đúng DTO: {posData: orderData}
	// Backend sẽ tự động extract dữ liệu từ posData
	data := map[string]interface{}{
		"posData": orderData,
	}

	log.Printf("[FolkForm] Đang gửi request upsert order đến FolkForm backend...")
	result, err = executePostRequest(client, "/v1/pancake-pos/order/upsert-one", data, params, "Gửi order thành công", "Gửi order thất bại. Thử lại lần thứ", false)
	if err != nil {
		log.Printf("[FolkForm] LỖI khi tạo/cập nhật order: %v", err)
	} else {
		log.Printf("[FolkForm] Tạo/cập nhật order thành công")
	}
	return result, err
}

// FolkForm_GetLastOrderUpdatedAt lấy posUpdatedAt (Unix timestamp giây) của order cập nhật gần nhất
// shopId: ID của shop (integer)
// Trả về: posUpdatedAt (seconds), error
func FolkForm_GetLastOrderUpdatedAt(shopId int) (updatedAt int64, err error) {
	log.Printf("[FolkForm] Lấy order cập nhật gần nhất - shopId: %d", shopId)

	if err := checkApiToken(); err != nil {
		return 0, err
	}

	client := createAuthorizedClient(defaultTimeout)

	// Query: filter theo shopId, sort theo posUpdatedAt DESC, limit 1
	filter := fmt.Sprintf(`{"shopId":%d}`, shopId)
	params := map[string]string{
		"filter":  filter,
		"options": `{"sort":{"posUpdatedAt":-1},"limit":1}`, // Sort desc (mới nhất trước)
	}

	result, err := executeGetRequest(
		client,
		"/v1/pancake-pos/order/find",
		params,
		"Lấy order cập nhật gần nhất thành công",
	)

	if err != nil {
		return 0, err
	}

	// Parse response
	var items []interface{}
	if dataMap, ok := result["data"].(map[string]interface{}); ok {
		if itemsArray, ok := dataMap["items"].([]interface{}); ok {
			items = itemsArray
		}
	}

	if len(items) == 0 {
		log.Printf("[FolkForm] Không tìm thấy order nào - shopId: %d", shopId)
		return 0, nil // Không có order → trả về 0
	}

	// items[0] = order cập nhật gần nhất (posUpdatedAt lớn nhất)
	firstItem := items[0]
	if order, ok := firstItem.(map[string]interface{}); ok {
		var updatedAtFloat float64 = 0
		if ua, ok := order["posUpdatedAt"].(float64); ok {
			updatedAtFloat = ua
		}
		// Convert từ milliseconds sang seconds (nếu cần)
		// posUpdatedAt có thể là seconds hoặc milliseconds, tùy backend
		// Nếu > 1e10 thì là milliseconds, ngược lại là seconds
		var updatedAtSeconds int64
		if updatedAtFloat > 1e10 {
			updatedAtSeconds = int64(updatedAtFloat) / 1000
		} else {
			updatedAtSeconds = int64(updatedAtFloat)
		}
		log.Printf("[FolkForm] Tìm thấy order cập nhật gần nhất - shopId: %d, posUpdatedAt: %d (seconds)", shopId, updatedAtSeconds)
		return updatedAtSeconds, nil
	}

	return 0, nil
}

// FolkForm_GetOldestOrderUpdatedAt lấy posUpdatedAt (Unix timestamp giây) của order cập nhật cũ nhất
// shopId: ID của shop (integer)
// Trả về: posUpdatedAt (seconds), error
func FolkForm_GetOldestOrderUpdatedAt(shopId int) (updatedAt int64, err error) {
	log.Printf("[FolkForm] Lấy order cập nhật cũ nhất - shopId: %d", shopId)

	if err := checkApiToken(); err != nil {
		return 0, err
	}

	client := createAuthorizedClient(defaultTimeout)

	// Query: filter theo shopId, sort theo posUpdatedAt ASC, limit 1
	filter := fmt.Sprintf(`{"shopId":%d}`, shopId)
	params := map[string]string{
		"filter":  filter,
		"options": `{"sort":{"posUpdatedAt":1},"limit":1}`, // Sort asc (cũ nhất trước)
	}

	result, err := executeGetRequest(
		client,
		"/v1/pancake-pos/order/find",
		params,
		"Lấy order cập nhật cũ nhất thành công",
	)

	if err != nil {
		return 0, err
	}

	// Parse response
	var items []interface{}
	if dataMap, ok := result["data"].(map[string]interface{}); ok {
		if itemsArray, ok := dataMap["items"].([]interface{}); ok {
			items = itemsArray
		}
	}

	if len(items) == 0 {
		log.Printf("[FolkForm] Không tìm thấy order nào - shopId: %d", shopId)
		return 0, nil // Không có order → trả về 0
	}

	// items[0] = order cập nhật cũ nhất (posUpdatedAt nhỏ nhất)
	firstItem := items[0]
	if order, ok := firstItem.(map[string]interface{}); ok {
		var updatedAtFloat float64 = 0
		if ua, ok := order["posUpdatedAt"].(float64); ok {
			updatedAtFloat = ua
		}
		// Convert từ milliseconds sang seconds (nếu cần)
		var updatedAtSeconds int64
		if updatedAtFloat > 1e10 {
			updatedAtSeconds = int64(updatedAtFloat) / 1000
		} else {
			updatedAtSeconds = int64(updatedAtFloat)
		}
		log.Printf("[FolkForm] Tìm thấy order cập nhật cũ nhất - shopId: %d, posUpdatedAt: %d (seconds)", shopId, updatedAtSeconds)
		return updatedAtSeconds, nil
	}

	return 0, nil
}

// Hàm FolkForm_TriggerNotification sẽ gửi yêu cầu trigger notification từ event
// Tham số:
// - eventType: Loại event (ví dụ: "conversation_unreplied")
// - payload: Dữ liệu cho template variables (map[string]interface{})
// Trả về result map và error
func FolkForm_TriggerNotification(eventType string, payload map[string]interface{}) (result map[string]interface{}, err error) {
	log.Printf("[FolkForm] Bắt đầu trigger notification - eventType: %s", eventType)

	if err := checkApiToken(); err != nil {
		log.Printf("[FolkForm] LỖI: %v", err)
		return nil, err
	}

	client := createAuthorizedClient(defaultTimeout)
	data := map[string]interface{}{
		"eventType": eventType,
		"payload":   payload,
	}

	log.Printf("[FolkForm] Đang gửi request trigger notification đến FolkForm backend...")
	log.Printf("[FolkForm] Endpoint: /notification/trigger")
	log.Printf("[FolkForm] EventType: %s", eventType)
	log.Printf("[FolkForm] Payload: %+v", payload)

	// Lưu ý: withSleep=false vì rate limiter đã được gọi trong executePostRequest
	// Nhưng cần đảm bảo rate limiter được gọi trước khi POST
	// Lưu ý: Backend có thể trả về status code 200 nhưng không có status="success"
	// Nếu response có message "Không có routing rule nào cho eventType này",
	// có thể routing rule chưa được tạo đúng hoặc thiếu organizationIds/channelTypes
	result, err = executePostRequest(client, "/v1/notification/trigger", data, nil, "Trigger notification thành công", "Trigger notification thất bại. Thử lại lần thứ", true)
	if err != nil {
		log.Printf("[FolkForm] LỖI khi trigger notification: %v", err)
	} else {
		log.Printf("[FolkForm] Trigger notification thành công - eventType: %s", eventType)
	}
	return result, err
}

// FolkForm_CreateNotificationTemplate tạo notification template nếu chưa tồn tại
// Tham số:
// - eventType: Loại event (ví dụ: "conversation_unreplied")
// - channelType: Loại kênh ("email", "telegram", "webhook")
// - subject: Subject cho email (optional)
// - content: Nội dung template với variables (ví dụ: "Hội thoại {{conversationId}} chưa được trả lời {{minutes}} phút")
// - variables: Danh sách variables (optional)
// - ctaCodes: Danh sách CTA codes (optional)
// - description: Mô tả về template để người dùng hiểu được mục đích sử dụng (optional, Version 3.11+)
// Trả về result map và error
func FolkForm_CreateNotificationTemplate(eventType string, channelType string, subject string, content string, variables []string, ctaCodes []string, description string) (result map[string]interface{}, err error) {
	log.Printf("[FolkForm] Bắt đầu tạo notification template - eventType: %s, channelType: %s", eventType, channelType)

	if err := checkApiToken(); err != nil {
		log.Printf("[FolkForm] LỖI: %v", err)
		return nil, err
	}

	client := createAuthorizedClient(defaultTimeout)
	data := map[string]interface{}{
		"eventType":   eventType,
		"channelType": channelType,
		"content":     content,
		"isActive":    true,
	}

	if subject != "" {
		data["subject"] = subject
	}

	if len(variables) > 0 {
		data["variables"] = variables
	}

	if len(ctaCodes) > 0 {
		data["ctaCodes"] = ctaCodes
	}

	// Thêm description nếu có (Version 3.11+)
	if description != "" {
		data["description"] = description
	}

	log.Printf("[FolkForm] Đang gửi request tạo notification template đến FolkForm backend...")
	log.Printf("[FolkForm] Endpoint: /notification/template/insert-one")

	result, err = executePostRequest(client, "/v1/notification/template/insert-one", data, nil, "Tạo notification template thành công", "Tạo notification template thất bại. Thử lại lần thứ", false)
	if err != nil {
		log.Printf("[FolkForm] LỖI khi tạo notification template: %v", err)
	} else {
		log.Printf("[FolkForm] Tạo notification template thành công - eventType: %s, channelType: %s", eventType, channelType)
	}
	return result, err
}

// FolkForm_CreateNotificationRoutingRule tạo notification routing rule
// Tham số:
// - eventType: Loại event (ví dụ: "conversation_unreplied")
// - organizationIds: Danh sách organization IDs sẽ nhận notification (array of strings)
// - channelTypes: Filter channels theo type (optional, ví dụ: ["email", "telegram"])
//   - Nếu empty/nil → lấy tất cả channels của organizations
//   - Nếu có giá trị → chỉ lấy channels có type trong danh sách
//
// Trả về result map và error
// Lưu ý: Routing rule chỉ cần eventType và organizationIds. Channels sẽ được tự động lấy từ organizations khi trigger.
// Nếu organizations chưa có channels, notification sẽ không được gửi (nhưng routing rule vẫn được tạo thành công).
func FolkForm_CreateNotificationRoutingRule(eventType string, organizationIds []string, channelTypes []string) (result map[string]interface{}, err error) {
	log.Printf("[FolkForm] Bắt đầu tạo notification routing rule - eventType: %s, organizationIds: %v", eventType, organizationIds)

	if err := checkApiToken(); err != nil {
		log.Printf("[FolkForm] LỖI: %v", err)
		return nil, err
	}

	// Lấy ownerOrganizationId từ role hiện tại (Version 3.9+ - REQUIRED)
	// Routing rule giờ cần ownerOrganizationId để phân quyền dữ liệu
	var ownerOrganizationId string
	if global.ActiveRoleId != "" {
		roles, err := FolkForm_GetRoles()
		if err == nil && len(roles) > 0 {
			// Lấy role đầu tiên để lấy ownerOrganizationId
			if roleMap, ok := roles[0].(map[string]interface{}); ok {
				if ownerOrgId, ok := roleMap["ownerOrganizationId"].(string); ok && ownerOrgId != "" {
					ownerOrganizationId = ownerOrgId
					log.Printf("[FolkForm] Lấy ownerOrganizationId từ role: %s", ownerOrganizationId)
				}
			}
		}
	}

	// Nếu không có ownerOrganizationId, thử lấy từ organizationIds đầu tiên
	if ownerOrganizationId == "" && len(organizationIds) > 0 {
		ownerOrganizationId = organizationIds[0]
		log.Printf("[FolkForm] Sử dụng organizationId đầu tiên làm ownerOrganizationId: %s", ownerOrganizationId)
	}

	client := createAuthorizedClient(defaultTimeout)
	data := map[string]interface{}{
		"eventType":       eventType,
		"organizationIds": organizationIds,
		"isActive":        true,
	}

	// Thêm ownerOrganizationId (Version 3.9+ - REQUIRED)
	if ownerOrganizationId != "" {
		data["ownerOrganizationId"] = ownerOrganizationId
		log.Printf("[FolkForm] Thêm ownerOrganizationId vào routing rule: %s", ownerOrganizationId)
	} else {
		log.Printf("[FolkForm] ⚠️ CẢNH BÁO: Không có ownerOrganizationId, backend có thể tự động gán từ context")
	}

	// Chỉ thêm channelTypes nếu có giá trị (không phải empty)
	// Nếu không có channelTypes, backend sẽ lấy tất cả channels của organizations
	if len(channelTypes) > 0 {
		data["channelTypes"] = channelTypes
		log.Printf("[FolkForm] Filter channels theo types: %v", channelTypes)
	} else {
		log.Printf("[FolkForm] Không filter channelTypes - sẽ lấy tất cả channels của organizations")
	}

	log.Printf("[FolkForm] Đang gửi request tạo notification routing rule đến FolkForm backend...")
	log.Printf("[FolkForm] Endpoint: /notification/routing/insert-one")
	log.Printf("[FolkForm] Request data: eventType=%s, organizationIds=%v, isActive=true", eventType, organizationIds)

	result, err = executePostRequest(client, "/v1/notification/routing/insert-one", data, nil, "Tạo notification routing rule thành công", "Tạo notification routing rule thất bại. Thử lại lần thứ", false)
	if err != nil {
		log.Printf("[FolkForm] LỖI khi tạo notification routing rule: %v", err)
	} else {
		log.Printf("[FolkForm] Tạo notification routing rule thành công - eventType: %s", eventType)
		log.Printf("[FolkForm] ⚠️ Lưu ý: Đảm bảo organizations (%v) có channels (email/telegram/webhook) để nhận notifications", organizationIds)
	}
	return result, err
}

// FolkForm_GetOrganizationIdsFromRole lấy danh sách organization IDs từ role hiện tại
// Trả về danh sách organization IDs (có thể nhiều nếu role có quyền với nhiều organizations)
func FolkForm_GetOrganizationIdsFromRole() ([]string, error) {
	log.Printf("[FolkForm] Bắt đầu lấy organization IDs từ role hiện tại")

	if global.ActiveRoleId == "" {
		log.Printf("[FolkForm] Chưa có Active Role ID, đang lấy roles...")
		roles, err := FolkForm_GetRoles()
		if err != nil {
			return nil, err
		}
		if len(roles) > 0 {
			if firstRole, ok := roles[0].(map[string]interface{}); ok {
				if roleId, ok := firstRole["id"].(string); ok && roleId != "" {
					global.ActiveRoleId = roleId
				} else if roleId, ok := firstRole["roleId"].(string); ok && roleId != "" {
					global.ActiveRoleId = roleId
				}
			}
		}
	}

	if global.ActiveRoleId == "" {
		return nil, errors.New("Không thể lấy Active Role ID")
	}

	// Lấy thông tin role để lấy ownerOrganizationId
	roles, err := FolkForm_GetRoles()
	if err != nil {
		return nil, err
	}

	var organizationIds []string
	for _, role := range roles {
		if roleMap, ok := role.(map[string]interface{}); ok {
			roleId, _ := roleMap["id"].(string)
			if roleId == "" {
				if rId, ok := roleMap["roleId"].(string); ok {
					roleId = rId
				}
			}

			// Nếu là role hiện tại hoặc tất cả roles (nếu cần)
			if roleId == global.ActiveRoleId || global.ActiveRoleId == "" {
				if ownerOrgId, ok := roleMap["ownerOrganizationId"].(string); ok && ownerOrgId != "" {
					organizationIds = append(organizationIds, ownerOrgId)
					log.Printf("[FolkForm] Tìm thấy organization ID: %s từ role: %s", ownerOrgId, roleId)
				}
			}
		}
	}

	if len(organizationIds) == 0 {
		log.Printf("[FolkForm] Không tìm thấy organization ID nào từ role, sẽ dùng routing rule mặc định")
	}

	return organizationIds, nil
}

// FolkForm_CheckNotificationTemplateExists kiểm tra xem notification template đã tồn tại chưa
// Tham số:
// - eventType: Loại event
// - channelType: Loại kênh
// Trả về true nếu đã tồn tại, false nếu chưa có, error nếu có lỗi
func FolkForm_CheckNotificationTemplateExists(eventType string, channelType string) (bool, error) {
	if err := checkApiToken(); err != nil {
		return false, err
	}

	client := createAuthorizedClient(defaultTimeout)

	// Tạo filter để tìm template
	filter := map[string]interface{}{
		"eventType":   eventType,
		"channelType": channelType,
	}

	filterJSON, err := json.Marshal(filter)
	if err != nil {
		return false, err
	}

	params := map[string]string{
		"filter":  string(filterJSON),
		"options": `{"limit":1}`,
	}

	result, err := executeGetRequest(client, "/v1/notification/template/find", params, "")
	if err != nil {
		return false, err
	}

	// Parse response
	var items []interface{}
	if dataMap, ok := result["data"].(map[string]interface{}); ok {
		if itemsArray, ok := dataMap["items"].([]interface{}); ok {
			items = itemsArray
		}
	} else if dataArray, ok := result["data"].([]interface{}); ok {
		items = dataArray
	}

	exists := len(items) > 0
	if exists {
		log.Printf("[FolkForm] Template đã tồn tại - eventType: %s, channelType: %s", eventType, channelType)
	} else {
		log.Printf("[FolkForm] Template chưa tồn tại - eventType: %s, channelType: %s", eventType, channelType)
	}
	return exists, nil
}

// FolkForm_CreateNotificationChannel tạo notification channel cho organization
// Tham số:
// - organizationId: Organization ID sẽ nhận notification
// - channelType: Loại channel ("email", "telegram", "webhook")
// - name: Tên channel (ví dụ: "Telegram Sales Team")
// - recipients: Danh sách recipients (email addresses cho email, chat IDs cho telegram, webhook URL cho webhook)
// - description: Mô tả về channel để người dùng hiểu được mục đích sử dụng (optional, Version 3.11+)
// Trả về result map và error
func FolkForm_CreateNotificationChannel(organizationId string, channelType string, name string, recipients []string, description string) (result map[string]interface{}, err error) {
	log.Printf("[FolkForm] Bắt đầu tạo notification channel - organizationId: %s, channelType: %s, name: %s", organizationId, channelType, name)

	if err := checkApiToken(); err != nil {
		log.Printf("[FolkForm] LỖI: %v", err)
		return nil, err
	}

	client := createAuthorizedClient(defaultTimeout)
	data := map[string]interface{}{
		"organizationId": organizationId,
		"channelType":    channelType,
		"name":           name,
		"isActive":       true,
	}

	// Thêm description nếu có (Version 3.11+)
	if description != "" {
		data["description"] = description
	}

	// Thêm recipients dựa trên channel type
	if channelType == "email" {
		data["recipients"] = recipients
	} else if channelType == "telegram" {
		data["chatIds"] = recipients
	} else if channelType == "webhook" {
		if len(recipients) > 0 {
			data["webhookUrl"] = recipients[0] // Webhook chỉ có 1 URL
		}
	}

	log.Printf("[FolkForm] Đang gửi request tạo notification channel đến FolkForm backend...")
	log.Printf("[FolkForm] Endpoint: /notification/channel/insert-one")
	log.Printf("[FolkForm] Request data: organizationId=%s, channelType=%s, name=%s", organizationId, channelType, name)

	result, err = executePostRequest(client, "/v1/notification/channel/insert-one", data, nil, "Tạo notification channel thành công", "Tạo notification channel thất bại. Thử lại lần thứ", false)
	if err != nil {
		// Kiểm tra xem có phải lỗi duplicate (409 Conflict) không
		// Backend đã có unique constraint và tự động validate duplicate
		// Nếu duplicate, không cần log error, chỉ log info
		if strings.Contains(err.Error(), "409") || strings.Contains(err.Error(), "Conflict") || strings.Contains(err.Error(), "duplicate") {
			log.Printf("[FolkForm] ℹ️ Channel đã tồn tại (backend đã validate duplicate) - organizationId: %s, channelType: %s, name: %s", organizationId, channelType, name)
			// Trả về nil error để coi như thành công (channel đã tồn tại)
			return result, nil
		}
		log.Printf("[FolkForm] LỖI khi tạo notification channel: %v", err)
	} else {
		log.Printf("[FolkForm] Tạo notification channel thành công - organizationId: %s, channelType: %s", organizationId, channelType)
	}
	return result, err
}

// FolkForm_CheckNotificationChannelExists kiểm tra xem notification channel đã tồn tại chưa
// Tham số:
// - organizationId: Organization ID
// - channelType: Loại channel ("email", "telegram", "webhook")
// Trả về true nếu đã tồn tại, false nếu chưa có, error nếu có lỗi
func FolkForm_CheckNotificationChannelExists(organizationId string, channelType string) (bool, error) {
	if err := checkApiToken(); err != nil {
		return false, err
	}

	client := createAuthorizedClient(defaultTimeout)

	// Tạo filter để tìm channel
	filter := map[string]interface{}{
		"organizationId": organizationId,
		"channelType":    channelType,
		"isActive":       true,
	}

	filterJSON, err := json.Marshal(filter)
	if err != nil {
		return false, err
	}

	params := map[string]string{
		"filter":  string(filterJSON),
		"options": `{"limit":1}`,
	}

	result, err := executeGetRequest(client, "/v1/notification/channel/find", params, "")
	if err != nil {
		return false, err
	}

	// Parse response
	var items []interface{}
	if dataMap, ok := result["data"].(map[string]interface{}); ok {
		if itemsArray, ok := dataMap["items"].([]interface{}); ok {
			items = itemsArray
		}
	} else if dataArray, ok := result["data"].([]interface{}); ok {
		items = dataArray
	}

	exists := len(items) > 0
	if exists {
		log.Printf("[FolkForm] Channel đã tồn tại - organizationId: %s, channelType: %s", organizationId, channelType)
	} else {
		log.Printf("[FolkForm] Channel chưa tồn tại - organizationId: %s, channelType: %s", organizationId, channelType)
	}
	return exists, nil
}

// FolkForm_CheckNotificationRoutingRuleExists kiểm tra xem notification routing rule đã tồn tại chưa
// Tham số:
// - eventType: Loại event
// Trả về true nếu đã tồn tại, false nếu chưa có, error nếu có lỗi
func FolkForm_CheckNotificationRoutingRuleExists(eventType string) (bool, error) {
	if err := checkApiToken(); err != nil {
		return false, err
	}

	client := createAuthorizedClient(defaultTimeout)

	// Tạo filter để tìm routing rule
	filter := map[string]interface{}{
		"eventType": eventType,
	}

	filterJSON, err := json.Marshal(filter)
	if err != nil {
		return false, err
	}

	params := map[string]string{
		"filter":  string(filterJSON),
		"options": `{"limit":1}`,
	}

	result, err := executeGetRequest(client, "/v1/notification/routing/find", params, "")
	if err != nil {
		return false, err
	}

	// Parse response
	var items []interface{}
	if dataMap, ok := result["data"].(map[string]interface{}); ok {
		if itemsArray, ok := dataMap["items"].([]interface{}); ok {
			items = itemsArray
		}
	} else if dataArray, ok := result["data"].([]interface{}); ok {
		items = dataArray
	}

	exists := len(items) > 0
	if exists {
		log.Printf("[FolkForm] Routing rule đã tồn tại - eventType: %s", eventType)
	} else {
		log.Printf("[FolkForm] Routing rule chưa tồn tại - eventType: %s", eventType)
	}
	return exists, nil
}

// FolkForm_CreateCTALibrary tạo CTA Library
// Tham số:
// - code: Mã CTA (unique trong organization, ví dụ: "view_detail")
// - label: Label hiển thị (có thể chứa {{variable}})
// - action: URL action (có thể chứa {{variable}})
// - style: Style của CTA ("primary", "success", "secondary", "danger")
// - variables: Danh sách variables (optional)
// - organizationId: Organization ID (optional, nếu rỗng sẽ lấy từ role)
// - description: Mô tả về CTA để người dùng hiểu được mục đích sử dụng (optional, Version 3.11+)
// Trả về result map và error
func FolkForm_CreateCTALibrary(code string, label string, action string, style string, variables []string, organizationId string, description string) (result map[string]interface{}, err error) {
	log.Printf("[FolkForm] Bắt đầu tạo CTA Library - code: %s", code)

	if err := checkApiToken(); err != nil {
		log.Printf("[FolkForm] LỖI: %v", err)
		return nil, err
	}

	client := createAuthorizedClient(defaultTimeout)
	data := map[string]interface{}{
		"code":     code,
		"label":    label,
		"action":   action,
		"isActive": true,
	}

	if style != "" {
		data["style"] = style
	}

	if len(variables) > 0 {
		data["variables"] = variables
	}

	if organizationId != "" {
		data["ownerOrganizationId"] = organizationId
	}

	// Thêm description nếu có (Version 3.11+)
	if description != "" {
		data["description"] = description
	}

	log.Printf("[FolkForm] Đang gửi request tạo CTA Library đến FolkForm backend...")
	log.Printf("[FolkForm] Endpoint: /cta/library/insert-one")

	result, err = executePostRequest(client, "/v1/cta/library/insert-one", data, nil, "Tạo CTA Library thành công", "Tạo CTA Library thất bại. Thử lại lần thứ", false)
	if err != nil {
		log.Printf("[FolkForm] LỖI khi tạo CTA Library: %v", err)
	} else {
		log.Printf("[FolkForm] Tạo CTA Library thành công - code: %s", code)
	}
	return result, err
}

// FolkForm_CheckCTALibraryExists kiểm tra xem CTA Library đã tồn tại chưa
// Tham số:
// - code: Mã CTA
// - organizationId: Organization ID (optional)
// Trả về true nếu đã tồn tại, false nếu chưa có, error nếu có lỗi
func FolkForm_CheckCTALibraryExists(code string, organizationId string) (bool, error) {
	if err := checkApiToken(); err != nil {
		return false, err
	}

	client := createAuthorizedClient(defaultTimeout)

	// Tạo filter để tìm CTA Library
	filter := map[string]interface{}{
		"code":     code,
		"isActive": true,
	}

	if organizationId != "" {
		filter["ownerOrganizationId"] = organizationId
	}

	filterJSON, err := json.Marshal(filter)
	if err != nil {
		return false, err
	}

	params := map[string]string{
		"filter":  string(filterJSON),
		"options": `{"limit":1}`,
	}

	result, err := executeGetRequest(client, "/v1/cta/library/find", params, "")
	if err != nil {
		return false, err
	}

	// Parse response
	var items []interface{}
	if dataMap, ok := result["data"].(map[string]interface{}); ok {
		if itemsArray, ok := dataMap["items"].([]interface{}); ok {
			items = itemsArray
		}
	} else if dataArray, ok := result["data"].([]interface{}); ok {
		items = dataArray
	}

	exists := len(items) > 0
	if exists {
		log.Printf("[FolkForm] CTA Library đã tồn tại - code: %s", code)
	} else {
		log.Printf("[FolkForm] CTA Library chưa tồn tại - code: %s", code)
	}
	return exists, nil
}

// FolkForm_EnsureNotificationSetup đảm bảo notification template và routing rule đã được tạo cho eventType
// Hàm này sẽ tạo CTA Library, template và routing rule mặc định nếu chưa có
// Tham số:
// - eventType: Loại event (ví dụ: "conversation_unreplied")
// - organizationIds: Danh sách organization IDs sẽ nhận notification (optional, nếu rỗng sẽ lấy từ role)
// Trả về error nếu có lỗi
func FolkForm_EnsureNotificationSetup(eventType string, organizationIds []string) error {
	log.Printf("[FolkForm] 🔧 Bắt đầu đảm bảo notification setup cho eventType: %s", eventType)

	// Lấy organizationIds từ role nếu chưa có (để tạo CTA Library)
	if len(organizationIds) == 0 {
		log.Printf("[FolkForm] 🔍 Đang lấy organization IDs từ role hiện tại để tạo CTA Library...")
		orgIds, err := FolkForm_GetOrganizationIdsFromRole()
		if err != nil {
			log.Printf("[FolkForm] ⚠️ Lưu ý: Không thể lấy organization IDs từ role: %v", err)
		} else {
			organizationIds = orgIds
			log.Printf("[FolkForm] ✅ Đã lấy được %d organization IDs từ role", len(organizationIds))
		}
	}

	// Tạo CTA Library "view_detail" nếu chưa có
	// CTA này sẽ được dùng trong notification templates
	ctaCode := "view_detail"
	ctaLabel := "Xem chi tiết"
	ctaAction := "{{conversationLink}}"
	ctaStyle := "primary"
	ctaVariables := []string{"conversationLink"}
	ctaDescription := "CTA để xem chi tiết conversation trong notification" // Version 3.11+

	// Kiểm tra CTA đã tồn tại chưa (tìm trong tất cả organizations hoặc system)
	var ctaExists bool
	var ctaErr error
	ctaExists, ctaErr = FolkForm_CheckCTALibraryExists(ctaCode, "")
	if ctaErr != nil {
		log.Printf("[FolkForm] ⚠️ Lỗi khi kiểm tra CTA Library: %v", ctaErr)
	} else if !ctaExists {
		log.Printf("[FolkForm] 📝 Tạo mới CTA Library - code: %s", ctaCode)
		// Tạo CTA cho organization đầu tiên (hoặc system nếu không có organization)
		orgId := ""
		if len(organizationIds) > 0 {
			orgId = organizationIds[0]
		}
		_, ctaErr = FolkForm_CreateCTALibrary(ctaCode, ctaLabel, ctaAction, ctaStyle, ctaVariables, orgId, ctaDescription)
		if ctaErr != nil {
			log.Printf("[FolkForm] ❌ Lỗi khi tạo CTA Library: %v", ctaErr)
		} else {
			log.Printf("[FolkForm] ✅ Đã tạo CTA Library thành công - code: %s", ctaCode)
		}
	} else {
		log.Printf("[FolkForm] ✅ CTA Library đã tồn tại - code: %s", ctaCode)
	}

	// Tạo template cho Telegram (phổ biến nhất)
	telegramContent := `🔔 *Tương tác CHẬM*

📄 Page: {{pageUsername}}
👤 Khách: {{customerName}}
📨 Loại: {{conversationType}}
🕐 Cập nhật: {{updatedAt}}
⏰ Trễ: {{minutes}} phút
🏷️ Tags: {{tags}}

🔗 [Xem hội thoại]({{conversationLink}})

*Yêu cầu*: Phản hồi khách sớm.`

	telegramVariables := []string{"pageUsername", "customerName", "conversationType", "updatedAt", "minutes", "tags", "conversationLink"}
	telegramCtaCodes := []string{"view_detail"}

	// Kiểm tra xem template đã tồn tại chưa
	exists, err := FolkForm_CheckNotificationTemplateExists(eventType, "telegram")
	if err != nil {
		log.Printf("[FolkForm] ⚠️ Lỗi khi kiểm tra template Telegram: %v", err)
	} else if !exists {
		log.Printf("[FolkForm] 📝 Tạo mới template Telegram cho eventType: %s", eventType)
		templateDescription := fmt.Sprintf("Template Telegram cho event %s - Cảnh báo hội thoại chưa được trả lời", eventType)
		_, err := FolkForm_CreateNotificationTemplate(
			eventType,
			"telegram",
			"", // Telegram không cần subject
			telegramContent,
			telegramVariables,
			telegramCtaCodes,
			templateDescription,
		)
		if err != nil {
			log.Printf("[FolkForm] ❌ Lỗi khi tạo template Telegram: %v", err)
		} else {
			log.Printf("[FolkForm] ✅ Đã tạo template Telegram thành công")
		}
	} else {
		log.Printf("[FolkForm] ✅ Template Telegram đã tồn tại, bỏ qua")
	}

	// Tạo template cho Email
	emailSubject := "🔔 Cảnh báo: Hội thoại chưa được trả lời"
	emailContent := `<h2>🔔 Tương tác CHẬM</h2>

<p><strong>📄 Page:</strong> {{pageUsername}}</p>
<p><strong>👤 Khách:</strong> {{customerName}}</p>
<p><strong>📨 Loại:</strong> {{conversationType}}</p>
<p><strong>🕐 Cập nhật:</strong> {{updatedAt}}</p>
<p><strong>⏰ Trễ:</strong> {{minutes}} phút</p>
<p><strong>🏷️ Tags:</strong> {{tags}}</p>

<p><a href="{{conversationLink}}">🔗 Xem hội thoại</a></p>

<p><strong>Yêu cầu:</strong> Phản hồi khách sớm.</p>`

	emailVariables := []string{"pageUsername", "customerName", "conversationType", "updatedAt", "minutes", "tags", "conversationLink"}
	emailCtaCodes := []string{"view_detail"}

	// Kiểm tra xem template đã tồn tại chưa
	exists, err = FolkForm_CheckNotificationTemplateExists(eventType, "email")
	if err != nil {
		log.Printf("[FolkForm] ⚠️ Lỗi khi kiểm tra template Email: %v", err)
	} else if !exists {
		log.Printf("[FolkForm] 📝 Tạo mới template Email cho eventType: %s", eventType)
		templateDescription := fmt.Sprintf("Template Email cho event %s - Cảnh báo hội thoại chưa được trả lời", eventType)
		_, err = FolkForm_CreateNotificationTemplate(
			eventType,
			"email",
			emailSubject,
			emailContent,
			emailVariables,
			emailCtaCodes,
			templateDescription,
		)
		if err != nil {
			log.Printf("[FolkForm] ❌ Lỗi khi tạo template Email: %v", err)
		} else {
			log.Printf("[FolkForm] ✅ Đã tạo template Email thành công")
		}
	} else {
		log.Printf("[FolkForm] ✅ Template Email đã tồn tại, bỏ qua")
	}

	// Tạo template cho Webhook
	webhookContent := `{
  "event": "{{eventType}}",
  "conversationId": "{{conversationId}}",
  "pageId": "{{pageId}}",
  "pageUsername": "{{pageUsername}}",
  "customerName": "{{customerName}}",
  "conversationType": "{{conversationType}}",
  "minutes": {{minutes}},
  "updatedAt": "{{updatedAt}}",
  "conversationLink": "{{conversationLink}}",
  "tags": "{{tags}}"
}`

	webhookVariables := []string{"eventType", "conversationId", "pageId", "pageUsername", "customerName", "conversationType", "minutes", "updatedAt", "conversationLink", "tags"}

	// Kiểm tra xem template đã tồn tại chưa
	exists, err = FolkForm_CheckNotificationTemplateExists(eventType, "webhook")
	if err != nil {
		log.Printf("[FolkForm] ⚠️ Lỗi khi kiểm tra template Webhook: %v", err)
	} else if !exists {
		log.Printf("[FolkForm] 📝 Tạo mới template Webhook cho eventType: %s", eventType)
		templateDescription := fmt.Sprintf("Template Webhook cho event %s - Cảnh báo hội thoại chưa được trả lời", eventType)
		_, err = FolkForm_CreateNotificationTemplate(
			eventType,
			"webhook",
			"", // Webhook không cần subject
			webhookContent,
			webhookVariables,
			nil, // Webhook không cần CTA
			templateDescription,
		)
		if err != nil {
			log.Printf("[FolkForm] ❌ Lỗi khi tạo template Webhook: %v", err)
		} else {
			log.Printf("[FolkForm] ✅ Đã tạo template Webhook thành công")
		}
	} else {
		log.Printf("[FolkForm] ✅ Template Webhook đã tồn tại, bỏ qua")
	}

	// Lấy organizationIds từ role nếu chưa có
	if len(organizationIds) == 0 {
		log.Printf("[FolkForm] 🔍 Đang lấy organization IDs từ role hiện tại...")
		orgIds, err := FolkForm_GetOrganizationIdsFromRole()
		if err != nil {
			log.Printf("[FolkForm] ⚠️ Lưu ý: Không thể lấy organization IDs từ role: %v", err)
		} else {
			organizationIds = orgIds
			log.Printf("[FolkForm] ✅ Đã lấy được %d organization IDs từ role", len(organizationIds))
		}
	}

	// Tạo channels cho mỗi organization nếu chưa có
	// Telegram channel với chat ID mặc định: -5139196836
	telegramChatId := "-5139196836"
	if len(organizationIds) > 0 {
		log.Printf("[FolkForm] 🔍 Kiểm tra và tạo channels cho %d organizations...", len(organizationIds))
		for _, orgId := range organizationIds {
			// Backend đã có unique constraint và validation tự động (Version 3.10)
			// - Unique compound index: (ownerOrganizationId, channelType, name)
			// - Handler tự động validate uniqueness → trả về 409 Conflict nếu duplicate
			// - Duplicate chatIDs: Mỗi organization chỉ có thể có 1 channel cho mỗi chatID
			//
			// Vẫn check trước để tránh gọi API không cần thiết, nhưng nếu check fails
			// vẫn thử tạo (backend sẽ trả về 409 nếu duplicate, không sao)
			exists, err := FolkForm_CheckNotificationChannelExists(orgId, "telegram")
			if err != nil {
				// Nếu check fails, vẫn thử tạo (backend sẽ validate)
				log.Printf("[FolkForm] ⚠️ Lỗi khi kiểm tra Telegram channel cho organization %s: %v", orgId, err)
				log.Printf("[FolkForm] 📝 Vẫn thử tạo channel (backend sẽ validate uniqueness)")
			} else if exists {
				log.Printf("[FolkForm] ✅ Telegram channel đã tồn tại cho organization: %s, bỏ qua", orgId)
				continue
			}

			// Tạo channel (backend sẽ trả về 409 Conflict nếu duplicate)
			log.Printf("[FolkForm] 📝 Tạo mới Telegram channel cho organization: %s với chatId: %s", orgId, telegramChatId)
			channelDescription := fmt.Sprintf("Telegram channel cho organization %s để nhận notifications", orgId)
			_, err = FolkForm_CreateNotificationChannel(orgId, "telegram", "Telegram Channel", []string{telegramChatId}, channelDescription)
			if err != nil {
				// Kiểm tra xem có phải lỗi duplicate không (409 Conflict)
				if strings.Contains(err.Error(), "409") || strings.Contains(err.Error(), "Conflict") || strings.Contains(err.Error(), "duplicate") {
					log.Printf("[FolkForm] ⚠️ Channel đã tồn tại (backend trả về 409 Conflict), bỏ qua: %v", err)
				} else {
					log.Printf("[FolkForm] ❌ Lỗi khi tạo Telegram channel: %v", err)
				}
			} else {
				log.Printf("[FolkForm] ✅ Đã tạo Telegram channel thành công cho organization: %s", orgId)
			}
		}
	}

	// Tạo routing rule nếu có organizationIds
	// Lưu ý: Routing rule chỉ cần eventType và organizationIds
	// Channels sẽ được tự động lấy từ organizations khi trigger notification
	if len(organizationIds) > 0 {
		log.Printf("[FolkForm] 📝 Tạo/cập nhật routing rule cho eventType: %s với %d organizations: %v", eventType, len(organizationIds), organizationIds)
		// Không chỉ định channelTypes để lấy tất cả channels của organizations
		// Nếu muốn filter, có thể chỉ định: channelTypes := []string{"telegram", "email", "webhook"}
		channelTypes := []string{} // Empty = lấy tất cả channels
		_, err = FolkForm_CreateNotificationRoutingRule(eventType, organizationIds, channelTypes)
		if err != nil {
			log.Printf("[FolkForm] ❌ Lỗi khi tạo routing rule: %v", err)
		} else {
			log.Printf("[FolkForm] ✅ Đã tạo/cập nhật routing rule thành công với organizationIds: %v", organizationIds)
		}
	} else {
		log.Printf("[FolkForm] ⚠️ Không có organization IDs, bỏ qua tạo routing rule")
	}

	log.Printf("[FolkForm] ✅ Hoàn thành đảm bảo notification setup cho eventType: %s", eventType)
	return nil
}

// FolkForm_CheckNotificationQueueItemExists kiểm tra xem notification queue item đã tồn tại chưa
// Tham số:
// - eventType: Loại event (ví dụ: "conversation_unreplied")
// - conversationId: ID của conversation (để kiểm tra notification đã được tạo cho conversation này chưa)
// Trả về true nếu đã tồn tại, false nếu chưa có, error nếu có lỗi
// Lưu ý: Kiểm tra dựa trên eventType và payload.conversationId trong queue item
func FolkForm_CheckNotificationQueueItemExists(eventType string, conversationId string) (bool, error) {
	if err := checkApiToken(); err != nil {
		return false, err
	}

	client := createAuthorizedClient(defaultTimeout)

	// Tạo filter để tìm queue item với eventType và payload.conversationId
	// Backend lưu payload trong queue item, cần filter theo payload.conversationId
	filter := map[string]interface{}{
		"eventType":              eventType,
		"payload.conversationId": conversationId,
		// Chỉ kiểm tra các item chưa được xử lý (status chưa là "completed" hoặc "failed")
		// Có thể thêm filter status nếu backend hỗ trợ
	}

	filterJSON, err := json.Marshal(filter)
	if err != nil {
		return false, err
	}

	params := map[string]string{
		"filter":  string(filterJSON),
		"options": `{"limit":1}`,
	}

	// Thử endpoint /notification/queue-item/find (nếu backend hỗ trợ)
	// Nếu không có, có thể thử /notification/queue/find hoặc endpoint khác
	result, err := executeGetRequest(client, "/v1/notification/queue-item/find", params, "")
	if err != nil {
		// Nếu endpoint không tồn tại, log warning và trả về false (cho phép tạo mới)
		// Điều này cho phép job tiếp tục hoạt động ngay cả khi backend chưa có endpoint này
		log.Printf("[FolkForm] ⚠️ Lỗi khi kiểm tra notification queue item (có thể endpoint chưa có hoặc chưa được implement): %v", err)
		log.Printf("[FolkForm] ⚠️ Sẽ tiếp tục tạo notification mới (không kiểm tra trùng lặp)")
		return false, nil // Trả về false để cho phép tạo mới nếu không kiểm tra được
	}

	// Parse response
	var items []interface{}
	if dataMap, ok := result["data"].(map[string]interface{}); ok {
		if itemsArray, ok := dataMap["items"].([]interface{}); ok {
			items = itemsArray
		}
	} else if dataArray, ok := result["data"].([]interface{}); ok {
		items = dataArray
	}

	exists := len(items) > 0
	if exists {
		log.Printf("[FolkForm] Notification queue item đã tồn tại - eventType: %s, conversationId: %s", eventType, conversationId)
	} else {
		log.Printf("[FolkForm] Notification queue item chưa tồn tại - eventType: %s, conversationId: %s", eventType, conversationId)
	}
	return exists, nil
}

// FolkForm_GetNotificationHistory lấy notification history với filter
// Tham số:
// - eventType: Loại event (ví dụ: "conversation_unreplied")
// - conversationId: ID của conversation (optional, nếu có sẽ filter theo payload.conversationId)
// - limit: Số lượng items tối đa (default: 20)
// Trả về danh sách notification history items
func FolkForm_GetNotificationHistory(eventType string, conversationId string, limit int) (items []interface{}, err error) {
	if err := checkApiToken(); err != nil {
		return nil, err
	}

	client := createAuthorizedClient(defaultTimeout)

	// Tạo filter
	filter := map[string]interface{}{
		"eventType": eventType,
	}
	if conversationId != "" {
		filter["payload.conversationId"] = conversationId
	}

	filterJSON, err := json.Marshal(filter)
	if err != nil {
		return nil, err
	}

	if limit <= 0 {
		limit = 20
	}

	params := map[string]string{
		"filter":  string(filterJSON),
		"options": fmt.Sprintf(`{"sort":{"createdAt":-1},"limit":%d}`, limit),
	}

	result, err := executeGetRequest(client, "/v1/notification/history/find", params, "")
	if err != nil {
		log.Printf("[FolkForm] ⚠️ Lỗi khi lấy notification history: %v", err)
		return nil, err
	}

	// Parse response
	if dataMap, ok := result["data"].(map[string]interface{}); ok {
		if itemsArray, ok := dataMap["items"].([]interface{}); ok {
			items = itemsArray
		}
	} else if dataArray, ok := result["data"].([]interface{}); ok {
		items = dataArray
	}

	log.Printf("[FolkForm] Đã lấy được %d notification history items", len(items))
	return items, nil
}

// FolkForm_GetNotificationQueueItems lấy notification queue items với filter
// Tham số:
// - eventType: Loại event (ví dụ: "conversation_unreplied")
// - conversationId: ID của conversation (optional, nếu có sẽ filter theo payload.conversationId)
// - limit: Số lượng items tối đa (default: 20)
// Trả về danh sách notification queue items
func FolkForm_GetNotificationQueueItems(eventType string, conversationId string, limit int) (items []interface{}, err error) {
	if err := checkApiToken(); err != nil {
		return nil, err
	}

	client := createAuthorizedClient(defaultTimeout)

	// Tạo filter
	filter := map[string]interface{}{
		"eventType": eventType,
	}
	if conversationId != "" {
		filter["payload.conversationId"] = conversationId
	}

	filterJSON, err := json.Marshal(filter)
	if err != nil {
		return nil, err
	}

	if limit <= 0 {
		limit = 20
	}

	params := map[string]string{
		"filter":  string(filterJSON),
		"options": fmt.Sprintf(`{"sort":{"createdAt":-1},"limit":%d}`, limit),
	}

	result, err := executeGetRequest(client, "/v1/notification/queue-item/find", params, "")
	if err != nil {
		log.Printf("[FolkForm] ⚠️ Lỗi khi lấy notification queue items: %v", err)
		return nil, err
	}

	// Parse response
	if dataMap, ok := result["data"].(map[string]interface{}); ok {
		if itemsArray, ok := dataMap["items"].([]interface{}); ok {
			items = itemsArray
		}
	} else if dataArray, ok := result["data"].([]interface{}); ok {
		items = dataArray
	}

	log.Printf("[FolkForm] Đã lấy được %d notification queue items", len(items))
	return items, nil
}

// FolkForm_EnhancedCheckIn gửi enhanced check-in với đầy đủ thông tin
// Tham số:
// - agentId: ID của agent (được gửi trong request body, không cần trong URL)
// - data: AgentCheckInRequest chứa system info, metrics, job status, config version/hash
// Trả về response từ server (AgentCheckInResponse)
// Endpoint mới: POST /api/v1/agent-management/check-in (theo API v3.12)
func FolkForm_EnhancedCheckIn(agentId string, data interface{}) (map[string]interface{}, error) {
	if err := checkApiToken(); err != nil {
		log.Printf("[FolkForm] [EnhancedCheckIn] LỖI: %v", err)
		return nil, err
	}

	client := createAuthorizedClient(defaultTimeout)

	// Sử dụng endpoint: /v1/agent-management/check-in (theo API v3.12)
	// agentId được gửi trong request body, không cần trong URL
	// Helper function sẽ tự động thêm /v1 vào đầu
	result, err := executePostRequest(client, "/v1/agent-management/check-in", data, nil,
		"", "Enhanced check-in thất bại. Thử lại lần thứ", false) // Bỏ log success message, chỉ log lỗi
	if err != nil {
		log.Printf("[FolkForm] [EnhancedCheckIn] ❌ Lỗi: %v", err)
	} else {
		// Log response để debug
		if result != nil {
			if data, ok := result["data"].(map[string]interface{}); ok {
				if commands, ok := data["commands"].([]interface{}); ok {
					log.Printf("[FolkForm] [EnhancedCheckIn] 📥 Response có %d command(s)", len(commands))
					for i, cmd := range commands {
						if cmdMap, ok := cmd.(map[string]interface{}); ok {
							cmdID, _ := cmdMap["id"].(string)
							cmdType, _ := cmdMap["type"].(string)
							cmdTarget, _ := cmdMap["target"].(string)
							log.Printf("[FolkForm] [EnhancedCheckIn]   Command[%d]: ID=%s, Type=%s, Target=%s", i, cmdID, cmdType, cmdTarget)
						}
					}
				} else {
					log.Printf("[FolkForm] [EnhancedCheckIn] 📥 Response không có commands hoặc commands không phải array")
				}
			} else {
				log.Printf("[FolkForm] [EnhancedCheckIn] 📥 Response không có data field")
			}
		}
	}
	return result, err
}

// FolkForm_SubmitConfig gửi config lên server
// Sử dụng endpoint: PUT /api/v1/agent-management/config/:agentId/update-data
// Endpoint này tự động tạo version mới mỗi lần update configData (theo tài liệu API v3.12+)
// Tham số:
// - agentId: ID của agent
// - configData: Config data (map[string]interface{})
// - configHash: Hash của config
// Trả về result chứa version (int64) và hash từ server
func FolkForm_SubmitConfig(agentId string, configData map[string]interface{}, configHash string) (map[string]interface{}, error) {
	log.Printf("[FolkForm] [SubmitConfig] ========================================")
	log.Printf("[FolkForm] [SubmitConfig] Bắt đầu submit config - agentId: %s", agentId)
	log.Printf("[FolkForm] [SubmitConfig] Config hash: %s", configHash)

	// QUAN TRỌNG: Kiểm tra agentId có hợp lệ không
	if agentId == "" {
		log.Printf("[FolkForm] [SubmitConfig] ❌ LỖI: agentId rỗng!")
		return nil, errors.New("agentId không được để trống")
	}

	if err := checkApiToken(); err != nil {
		log.Printf("[FolkForm] [SubmitConfig] LỖI: %v", err)
		return nil, err
	}

	client := createAuthorizedClient(defaultTimeout)

	// Build request body
	// QUAN TRỌNG: Set isActive=true để đảm bảo config này là active config cho agent
	// QUAN TRỌNG: Không set version trong request body vì backend sẽ tự động tạo version mới
	// Nếu đã có config (upsert) → backend sẽ giữ nguyên version hoặc tạo version mới
	// Nếu chưa có config (insert) → backend sẽ tạo version mới
	// QUAN TRỌNG: Luôn dùng agentId từ parameter (ENV), KHÔNG lấy từ response
	requestBody := map[string]interface{}{
		"agentId":        agentId, // QUAN TRỌNG: Dùng agentId từ parameter, KHÔNG lấy từ response
		"configData":     configData,
		"configHash":     configHash,
		"botVersion":     "1.0.0", // TODO: Lấy từ build info
		"submittedByBot": true,
		"isActive":       true, // QUAN TRỌNG: Đảm bảo config này là active
		// Lưu ý: KHÔNG set "version" trong request body - backend sẽ tự động tạo version
	}

	log.Printf("[FolkForm] [SubmitConfig] Đang gửi request PUT submit config đến FolkForm backend...")
	log.Printf("[FolkForm] [SubmitConfig] Endpoint: /v1/agent-management/config/%s/update-data", agentId)
	log.Printf("[FolkForm] [SubmitConfig] Request body - agentId: %s (từ parameter, KHÔNG từ response), isActive: true, configHash: %s", agentId, configHash)
	log.Printf("[FolkForm] [SubmitConfig] 🔍 Xác nhận: agentId trong requestBody = %s (phải khớp với parameter)", requestBody["agentId"])

	// Sử dụng endpoint: PUT /v1/agent-management/config/:agentId/update-data (theo tài liệu API)
	// QUAN TRỌNG: Endpoint này tự động tạo version mới mỗi lần update configData
	// - Nếu có config active → deactivate config cũ, tạo config mới với version mới
	// - Nếu chưa có config → tạo config mới
	// - Version được server tự động gán bằng Unix timestamp
	// Theo tài liệu: dòng 3855-3863 trong api-context.md
	log.Printf("[FolkForm] [SubmitConfig] Sử dụng endpoint update-data để tự động tạo version mới")

	// Helper function sẽ tự động thêm /v1 vào đầu
	endpoint := fmt.Sprintf("/v1/agent-management/config/%s/update-data", agentId)
	result, err := executePutRequest(client, endpoint, requestBody, nil,
		"Submit config thành công", "Submit config thất bại. Thử lại lần thứ", true)
	if err != nil {
		log.Printf("[FolkForm] [SubmitConfig] ❌ LỖI khi submit config: %v", err)
		log.Printf("[FolkForm] [SubmitConfig] ========================================")
	} else {
		log.Printf("[FolkForm] [SubmitConfig] ✅ Submit config thành công - agentId: %s", agentId)
		if result != nil {
			// QUAN TRỌNG: Chỉ lấy version và hash từ response, KHÔNG lấy id
			// Response có thể có data.id (ID của config document) nhưng KHÔNG được dùng làm agentId
			// Response có thể có version ở root level hoặc trong data
			// Backend v3.12+ trả về version là Unix timestamp (int64) - không phải string
			var version int64
			var hash string

			// Parse version từ response - theo API v3.12, version là int64 (Unix timestamp)
			// JSON unmarshal có thể trả về float64 cho số, nên cần convert
			parseVersion := func(v interface{}) int64 {
				if v == nil {
					return 0
				}
				switch val := v.(type) {
				case int64:
					return val
				case float64:
					return int64(val)
				case int:
					return int64(val)
				default:
					log.Printf("[FolkForm] [SubmitConfig] ⚠️  Version không phải số: %T %v", val, val)
					return 0
				}
			}

			// Thử lấy version từ root level trước
			if v, exists := result["version"]; exists {
				version = parseVersion(v)
				if version != 0 {
					log.Printf("[FolkForm] [SubmitConfig] Config version từ server (root): %d", version)
				}
			}
			// Thử lấy version từ data nếu không có ở root
			if version == 0 {
				if data, ok := result["data"].(map[string]interface{}); ok {
					if v, exists := data["version"]; exists {
						version = parseVersion(v)
						if version != 0 {
							log.Printf("[FolkForm] [SubmitConfig] Config version từ server (data): %d", version)
						}
					}
				}
			}
			// Nếu vẫn không có version → cảnh báo
			if version == 0 {
				log.Printf("[FolkForm] [SubmitConfig] ⚠️  CẢNH BÁO: Response không có version! Có thể config mới được tạo nhưng chưa có version")
				log.Printf("[FolkForm] [SubmitConfig] ⚠️  Response structure: %+v", result)
			}

			// Thử lấy hash từ root level trước
			if h, ok := result["configHash"].(string); ok && h != "" {
				hash = h
				log.Printf("[FolkForm] [SubmitConfig] Config hash từ server (root): %s", hash)
			}
			// Thử lấy hash từ data nếu không có ở root
			if hash == "" {
				if data, ok := result["data"].(map[string]interface{}); ok {
					if h, ok := data["configHash"].(string); ok && h != "" {
						hash = h
						log.Printf("[FolkForm] [SubmitConfig] Config hash từ server (data): %s", hash)
					}
				}
			}

			// Trả về version và hash trong result để ConfigManager có thể sử dụng
			if result["version"] == nil {
				result["version"] = version
			}
			if result["configHash"] == nil {
				result["configHash"] = hash
			}

			// Log để debug: Kiểm tra xem có id trong response không (KHÔNG được dùng)
			if data, ok := result["data"].(map[string]interface{}); ok {
				if id, exists := data["id"]; exists {
					log.Printf("[FolkForm] [SubmitConfig] ⚠️  CẢNH BÁO: Response có field 'id': %v (KHÔNG được dùng làm agentId)", id)
					log.Printf("[FolkForm] [SubmitConfig] ⚠️  AgentId đúng phải là: %s (từ parameter, KHÔNG phải từ response.id)", agentId)
				}
				// Kiểm tra xem có agentId trong response không (để so sánh)
				if agentIdFromResponse, exists := data["agentId"]; exists {
					log.Printf("[FolkForm] [SubmitConfig] Response có field 'agentId': %v", agentIdFromResponse)
					if agentIdFromResponse != agentId {
						log.Printf("[FolkForm] [SubmitConfig] ⚠️  CẢNH BÁO: agentId từ response (%v) khác với agentId từ parameter (%s)", agentIdFromResponse, agentId)
					}
				}
			}
		}
		log.Printf("[FolkForm] [SubmitConfig] ========================================")
	}
	return result, err
}

// FolkForm_GetCurrentConfig lấy config hiện tại từ server
// Tham số:
// - agentId: ID của agent
// Trả về AgentConfig từ server
func FolkForm_GetCurrentConfig(agentId string) (*AgentConfig, error) {
	log.Printf("[FolkForm] [GetCurrentConfig] Bắt đầu lấy config - agentId: %s", agentId)

	if err := checkApiToken(); err != nil {
		log.Printf("[FolkForm] [GetCurrentConfig] LỖI: %v", err)
		return nil, err
	}

	client := createAuthorizedClient(defaultTimeout)

	log.Printf("[FolkForm] [GetCurrentConfig] Đang gửi request GET current config đến FolkForm backend...")

	// Sử dụng endpoint: /v1/agent-management/config/find với filter agentId và isActive=true (theo API v3.12)
	// Tìm config active của agent
	// Helper function sẽ tự động thêm /v1 vào đầu
	filter := map[string]interface{}{
		"agentId":  agentId,
		"isActive": true,
	}
	filterJSON, _ := json.Marshal(filter)
	params := map[string]string{
		"filter":  string(filterJSON),
		"options": `{"sort":{"createdAt":-1},"limit":1}`, // Lấy config mới nhất
	}
	result, err := executeGetRequest(client, "/v1/agent-management/config/find", params, "Lấy config thành công")
	if err != nil {
		log.Printf("[FolkForm] [GetCurrentConfig] LỖI khi lấy config: %v", err)
		return nil, err
	}

	// Parse response - response có thể là array hoặc object
	var config AgentConfig

	// Helper function để parse version từ interface{} sang int64
	// Theo API v3.12, version là Unix timestamp (int64) - không phải string
	parseVersion := func(v interface{}) int64 {
		if v == nil {
			return 0
		}
		switch val := v.(type) {
		case int64:
			return val
		case float64:
			return int64(val)
		case int:
			return int64(val)
		default:
			log.Printf("[FolkForm] [GetCurrentConfig] ⚠️  Version không phải số: %T %v", val, val)
			return 0
		}
	}

	// Nếu response.data là array (từ find endpoint)
	if dataArray, ok := result["data"].([]interface{}); ok && len(dataArray) > 0 {
		if data, ok := dataArray[0].(map[string]interface{}); ok {
			// Parse config từ item đầu tiên
			if v, exists := data["version"]; exists {
				config.Version = parseVersion(v)
			}
			if hash, ok := data["configHash"].(string); ok {
				config.ConfigHash = hash
			}
			if configData, ok := data["configData"].(map[string]interface{}); ok {
				config.ConfigData = configData
			}
		}
	} else if data, ok := result["data"].(map[string]interface{}); ok {
		// Nếu response.data là object (từ find-by-id hoặc insert-one)
		if v, exists := data["version"]; exists {
			config.Version = parseVersion(v)
		}
		if hash, ok := data["configHash"].(string); ok {
			config.ConfigHash = hash
		}
		if configData, ok := data["configData"].(map[string]interface{}); ok {
			config.ConfigData = configData
		}
	}

	log.Printf("[FolkForm] [GetCurrentConfig] Lấy config thành công - version: %d", config.Version)
	return &config, nil
}

// AgentConfig struct cho response từ server
// Backend v3.12+ trả về version là Unix timestamp (int64)
type AgentConfig struct {
	Version    int64                  `json:"version"` // Unix timestamp (server tự động quyết định)
	ConfigHash string                 `json:"configHash"`
	ConfigData map[string]interface{} `json:"configData"`
}

// FolkForm_UpdateCommand cập nhật trạng thái và kết quả của command
// Tham số:
// - commandID: ID của command cần update
// - updateData: Dữ liệu cần update (status, result, error, executedAt, completedAt)
// Trả về result map và error
func FolkForm_UpdateCommand(commandID string, updateData map[string]interface{}) (map[string]interface{}, error) {
	log.Printf("[FolkForm] [UpdateCommand] Bắt đầu update command - commandID: %s", commandID)

	if commandID == "" {
		log.Printf("[FolkForm] [UpdateCommand] ❌ LỖI: commandID rỗng!")
		return nil, errors.New("commandID không được để trống")
	}

	if err := checkApiToken(); err != nil {
		log.Printf("[FolkForm] [UpdateCommand] LỖI: %v", err)
		return nil, err
	}

	client := createAuthorizedClient(defaultTimeout)

	log.Printf("[FolkForm] [UpdateCommand] Đang gửi request PUT update command đến FolkForm backend...")
	log.Printf("[FolkForm] [UpdateCommand] Command ID: %s", commandID)
	log.Printf("[FolkForm] [UpdateCommand] Update data: %+v", updateData)

	// Sử dụng endpoint: /v1/agent-management/command/update-by-id/:id
	// Helper function sẽ tự động thêm /v1 vào đầu
	endpoint := fmt.Sprintf("/v1/agent-management/command/update-by-id/%s", commandID)
	result, err := executePutRequest(client, endpoint, updateData, nil,
		"Update command thành công", "Update command thất bại. Thử lại lần thứ", true)
	if err != nil {
		log.Printf("[FolkForm] [UpdateCommand] ❌ LỖI khi update command: %v", err)
	} else {
		log.Printf("[FolkForm] [UpdateCommand] ✅ Update command thành công - commandID: %s", commandID)
	}
	return result, err
}

// ========================================
// MODULE 2: AI SERVICE API INTEGRATION
// ========================================

// ClaimDebugLogFn là callback để ghi log chi tiết vào logger của job (ví dụ: workflow-commands-job.log).
// Nếu job truyền callback này, mọi log chi tiết claim sẽ ghi cả vào stdlog và vào file log của job.
type ClaimDebugLogFn func(msg string)

// FolkForm_ClaimWorkflowCommands claim pending workflow commands từ Module 2 với atomic operation.
// Theo api-context.md: POST /api/v1/ai/workflow-commands/claim-pending
// Request: { "agentId": "agent-123", "limit": 5 } (limit tối đa 100)
// Response: { "status": "success", "code": 200, "message": "...", "data": [...] } — data là danh sách commands (hoặc null khi không có)
// Tham số:
// - agentId: ID của agent
// - limit: Số lượng commands tối đa muốn claim (tối đa 100)
// - logToJob: (tùy chọn) callback ghi log chi tiết vào logger của job; nil = chỉ ghi stdlog
// Trả về danh sách commands đã được claim và error
func FolkForm_ClaimWorkflowCommands(agentId string, limit int, logToJob ClaimDebugLogFn) ([]interface{}, error) {
	clog := func(format string, args ...interface{}) {
		msg := fmt.Sprintf(format, args...)
		log.Printf("[FolkForm] [ClaimWorkflowCommands] %s", msg)
		if logToJob != nil {
			logToJob(msg)
		}
	}

	// Ghi một block nhiều dòng vào log (dùng logToJob nếu có để chắc chắn thấy trong job log / console)
	writeBlock := func(title, block string) {
		if logToJob != nil {
			logToJob(title + "\n" + block)
		} else {
			log.Printf("[FolkForm] [ClaimWorkflowCommands] %s\n%s", title, block)
		}
	}

	endpoint := "/v1/ai/workflow-commands/claim-pending"
	fullURL := strings.TrimSuffix(global.GlobalConfig.ApiBaseUrl, "/") + endpoint

	if err := checkApiToken(); err != nil {
		clog("LỖI token: %v", err)
		return nil, err
	}

	client := createAuthorizedClient(defaultTimeout)

	// Validate limit
	if limit <= 0 {
		limit = 5 // Default limit
	}
	if limit > 100 {
		limit = 100 // Max limit
	}

	// Chuẩn bị request body
	requestBody := map[string]interface{}{
		"agentId": agentId,
		"limit":   limit,
	}
	requestBodyJSON, _ := json.Marshal(requestBody)

	// ---------- In chi tiết REQUEST (một block để dễ thấy) ----------
	requestBlock := fmt.Sprintf("Method: POST\nURL: %s\nHeaders: Authorization: Bearer ***, X-Active-Role-ID: %s\nBody: %s",
		fullURL, global.ActiveRoleId, string(requestBodyJSON))
	writeBlock("========== REQUEST (Claim Workflow Commands) ==========", requestBlock)

	// Gọi API claim-pending
	result, err := executePostRequest(client, endpoint, requestBody, nil,
		"Claim workflow commands thành công", "Claim workflow commands thất bại. Thử lại lần thứ", true)
	if err != nil {
		clog("❌ Lỗi khi claim commands: %v", err)
		return nil, err
	}

	// ---------- In chi tiết RESPONSE (một block để dễ thấy) ----------
	if result == nil {
		writeBlock("========== RESPONSE (Claim Workflow Commands) ==========", "Body: (nil)")
		return nil, nil
	}

	resultJSON, _ := json.Marshal(result)
	responseBlock := fmt.Sprintf("HTTP Status: 200 OK\nBody (raw JSON): %s\n--- Parsed ---\n  status: %v\n  code: %v\n  message: %v\n  data: %v",
		string(resultJSON), result["status"], result["code"], result["message"], result["data"])
	writeBlock("========== RESPONSE (Claim Workflow Commands) ==========", responseBlock)

	dataRaw := result["data"]
	if dataRaw == nil {
		clog("⚠️ data = null → 0 commands (backend trả về không có danh sách commands; nếu backend có pending command thì cần trả về data: [] hoặc data: [{...}])")
		clog("Full response (map): %+v", result)
		return []interface{}{}, nil
	}

	// Parse response - response có thể là array hoặc object với data field
	var commands []interface{}
	if data, ok := result["data"].([]interface{}); ok {
		commands = data
		clog("data là array, length = %d", len(commands))
	} else if dataObj, ok := result["data"].(map[string]interface{}); ok {
		clog("data là object, keys: %v", getMapKeys(dataObj))
		if items, ok := dataObj["items"].([]interface{}); ok {
			commands = items
			clog("data.items là array, length = %d", len(commands))
		} else if items, ok := dataObj["commands"].([]interface{}); ok {
			commands = items
			clog("data.commands là array, length = %d", len(commands))
		} else {
			clog("⚠️ data không có 'items' hay 'commands' → 0 commands. data = %+v", dataObj)
		}
	} else {
		clog("⚠️ data không phải array cũng không phải object (type = %T) → 0 commands", dataRaw)
		clog("data value: %+v", dataRaw)
	}

	if len(commands) == 0 {
		clog("Kết quả: 0 command (server trả về data: null hoặc không có pending command)")
	} else {
		clog("✅ Claim thành công - đã claim %d command(s)", len(commands))
		for i, cmd := range commands {
			if cm, ok := cmd.(map[string]interface{}); ok {
				if id, _ := cm["id"].(string); id != "" {
					clog("  command[%d] id=%s", i, id)
				}
			}
		}
	}
	return commands, nil
}

// FolkForm_StartWorkflowRun gọi API Module 2 để start workflow run
// Tham số:
// - workflowId: ID của workflow
// - rootRefId: ID của root reference (ví dụ: layer ID)
// - rootRefType: Type của root reference (ví dụ: "layer")
// - params: Additional parameters
// Trả về result map và error
func FolkForm_StartWorkflowRun(workflowId, rootRefId, rootRefType string, params map[string]interface{}) (map[string]interface{}, error) {
	log.Printf("[FolkForm] [StartWorkflowRun] Bắt đầu start workflow run - workflowId: %s, rootRefId: %s, rootRefType: %s", workflowId, rootRefId, rootRefType)

	if err := checkApiToken(); err != nil {
		log.Printf("[FolkForm] [StartWorkflowRun] LỖI: %v", err)
		return nil, err
	}

	client := createAuthorizedClient(longTimeout) // Dùng longTimeout vì workflow có thể chạy lâu

	// Chuẩn bị request body
	requestBody := map[string]interface{}{
		"workflowId":  workflowId,
		"rootRefId":   rootRefId,
		"rootRefType": rootRefType,
	}
	if params != nil {
		requestBody["params"] = params
	}

	// Theo api-context.md: POST /api/v1/ai/workflow-runs/insert-one
	result, err := executePostRequest(client, "/v1/ai/workflow-runs/insert-one", requestBody, nil,
		"Start workflow run thành công", "Start workflow run thất bại. Thử lại lần thứ", true)
	if err != nil {
		log.Printf("[FolkForm] [StartWorkflowRun] ❌ Lỗi khi start workflow run: %v", err)
	} else {
		log.Printf("[FolkForm] [StartWorkflowRun] ✅ Start workflow run thành công")
	}
	return result, err
}

// FolkForm_UpdateWorkflowCommand update status của workflow command
// Sử dụng endpoint: PUT /api/v1/ai/workflow-commands/update-by-id/:id (theo tài liệu API)
// Tham số:
// - commandID: ID của command
// - status: Status mới (ví dụ: "processing", "completed", "failed")
// - result: Result data (optional)
// Trả về result map và error
func FolkForm_UpdateWorkflowCommand(commandID string, status string, result map[string]interface{}) (map[string]interface{}, error) {
	log.Printf("[FolkForm] [UpdateWorkflowCommand] Bắt đầu update workflow command - commandID: %s, status: %s", commandID, status)

	if commandID == "" {
		log.Printf("[FolkForm] [UpdateWorkflowCommand] ❌ LỖI: commandID rỗng!")
		return nil, errors.New("commandID không được để trống")
	}

	if err := checkApiToken(); err != nil {
		log.Printf("[FolkForm] [UpdateWorkflowCommand] LỖI: %v", err)
		return nil, err
	}

	client := createAuthorizedClient(defaultTimeout)

	// Chuẩn bị update data
	updateData := map[string]interface{}{
		"status": status,
	}
	if result != nil {
		updateData["result"] = result
	}

	// Sử dụng endpoint: /v1/ai/workflow-commands/update-by-id/:id (theo tài liệu API)
	endpoint := fmt.Sprintf("/v1/ai/workflow-commands/update-by-id/%s", commandID)
	apiResult, err := executePutRequest(client, endpoint, updateData, nil,
		"Update workflow command thành công", "Update workflow command thất bại. Thử lại lần thứ", true)
	if err != nil {
		log.Printf("[FolkForm] [UpdateWorkflowCommand] ❌ LỖI khi update command: %v", err)
	} else {
		log.Printf("[FolkForm] [UpdateWorkflowCommand] ✅ Update command thành công - commandID: %s", commandID)
	}
	return apiResult, err
}

// ========================================
// MODULE 2: LOAD DEFINITIONS
// ========================================

// FolkForm_GetWorkflow lấy workflow definition từ Module 2
// Sử dụng endpoint: GET /api/v1/ai/workflows/find-by-id/:id (theo pattern CRUD chuẩn)
func FolkForm_GetWorkflow(workflowId string) (map[string]interface{}, error) {
	if err := checkApiToken(); err != nil {
		return nil, err
	}

	client := createAuthorizedClient(defaultTimeout)
	endpoint := fmt.Sprintf("/v1/ai/workflows/find-by-id/%s", workflowId)
	result, err := executeGetRequest(client, endpoint, nil, "Get workflow thành công")
	return result, err
}

// FolkForm_GetStep lấy step definition từ Module 2
// Sử dụng endpoint: GET /api/v1/ai/steps/find-by-id/:id (theo pattern CRUD chuẩn)
func FolkForm_GetStep(stepId string) (map[string]interface{}, error) {
	if err := checkApiToken(); err != nil {
		return nil, err
	}

	client := createAuthorizedClient(defaultTimeout)
	endpoint := fmt.Sprintf("/v1/ai/steps/find-by-id/%s", stepId)
	result, err := executeGetRequest(client, endpoint, nil, "Get step thành công")
	return result, err
}

// FolkForm_GetPromptTemplate lấy prompt template từ Module 2
// Sử dụng endpoint: GET /api/v1/ai/prompt-templates/find-by-id/:id (theo pattern CRUD chuẩn)
func FolkForm_GetPromptTemplate(templateId string) (map[string]interface{}, error) {
	if err := checkApiToken(); err != nil {
		return nil, err
	}

	client := createAuthorizedClient(defaultTimeout)
	endpoint := fmt.Sprintf("/v1/ai/prompt-templates/find-by-id/%s", templateId)
	result, err := executeGetRequest(client, endpoint, nil, "Get prompt template thành công")
	return result, err
}

// FolkForm_GetProviderProfile lấy provider profile từ Module 2
// Sử dụng endpoint: GET /api/v1/ai/provider-profiles/find-by-id/:id (theo pattern CRUD chuẩn)
func FolkForm_GetProviderProfile(profileId string) (map[string]interface{}, error) {
	if err := checkApiToken(); err != nil {
		return nil, err
	}

	client := createAuthorizedClient(defaultTimeout)
	endpoint := fmt.Sprintf("/v1/ai/provider-profiles/find-by-id/%s", profileId)
	result, err := executeGetRequest(client, endpoint, nil, "Get provider profile thành công")
	return result, err
}

// FolkForm_RenderPromptForStep render prompt cho step và lấy AI config (API v2)
// Sử dụng endpoint: POST /api/v2/ai/steps/:id/render-prompt
// Request: { variables: { layerName: "...", targetAudience: "B2C", ... } }
// Response: { renderedPrompt: "...", providerProfileId: "...", provider: "openai", model: "gpt-4", temperature: 0.7, maxTokens: 2000, variables: {...} }
func FolkForm_RenderPromptForStep(stepId string, variables map[string]interface{}) (map[string]interface{}, error) {
	log.Printf("[FolkForm] [RenderPromptForStep] Bắt đầu render prompt cho step - stepId: %s", stepId)

	if err := checkApiToken(); err != nil {
		log.Printf("[FolkForm] [RenderPromptForStep] LỖI: %v", err)
		return nil, err
	}

	client := createAuthorizedClient(defaultTimeout)

	// Chuẩn bị request body
	requestBody := map[string]interface{}{
		"variables": variables,
	}

	// Sử dụng endpoint: /v2/ai/steps/:id/render-prompt
	endpoint := fmt.Sprintf("/v2/ai/steps/%s/render-prompt", stepId)
	result, err := executePostRequest(client, endpoint, requestBody, nil,
		"Render prompt thành công", "Render prompt thất bại. Thử lại lần thứ", true)
	if err != nil {
		log.Printf("[FolkForm] [RenderPromptForStep] ❌ Lỗi khi render prompt: %v", err)
	} else {
		log.Printf("[FolkForm] [RenderPromptForStep] ✅ Render prompt thành công")
	}
	return result, err
}

// FolkForm_GetContentNode lấy content node từ Module 1
// Sử dụng endpoint: GET /api/v1/content/nodes/find-by-id/:id (theo pattern CRUD chuẩn)
func FolkForm_GetContentNode(nodeId string) (map[string]interface{}, error) {
	if err := checkApiToken(); err != nil {
		return nil, err
	}

	client := createAuthorizedClient(defaultTimeout)
	endpoint := fmt.Sprintf("/v1/content/nodes/find-by-id/%s", nodeId)
	result, err := executeGetRequest(client, endpoint, nil, "Get content node thành công")
	return result, err
}

// FolkForm_GetDraftNode lấy draft node từ Module 1
// Sử dụng endpoint: GET /api/v1/content/drafts/nodes/find-by-id/:id (theo pattern CRUD chuẩn)
func FolkForm_GetDraftNode(nodeId string) (map[string]interface{}, error) {
	if err := checkApiToken(); err != nil {
		return nil, err
	}

	client := createAuthorizedClient(defaultTimeout)
	endpoint := fmt.Sprintf("/v1/content/drafts/nodes/find-by-id/%s", nodeId)
	result, err := executeGetRequest(client, endpoint, nil, "Get draft node thành công")
	return result, err
}

// FolkForm_CreateWorkflowRun tạo workflow run record trong Module 2
func FolkForm_CreateWorkflowRun(workflowId, rootRefId, rootRefType string, params map[string]interface{}) (map[string]interface{}, error) {
	if err := checkApiToken(); err != nil {
		return nil, err
	}

	client := createAuthorizedClient(defaultTimeout)
	requestBody := map[string]interface{}{
		"workflowId":  workflowId,
		"rootRefId":   rootRefId,
		"rootRefType": rootRefType,
		"status":      "running",
	}
	if params != nil {
		requestBody["params"] = params
	}

	// Theo api-context.md: POST /api/v1/ai/workflow-runs/insert-one
	result, err := executePostRequest(client, "/v1/ai/workflow-runs/insert-one", requestBody, nil,
		"Create workflow run thành công", "Create workflow run thất bại. Thử lại lần thứ", true)
	return result, err
}

// FolkForm_CreateStepRun tạo step run record trong Module 2
func FolkForm_CreateStepRun(workflowRunId, stepId string, input map[string]interface{}) (map[string]interface{}, error) {
	if err := checkApiToken(); err != nil {
		return nil, err
	}

	client := createAuthorizedClient(defaultTimeout)
	requestBody := map[string]interface{}{
		"workflowRunId": workflowRunId,
		"stepId":        stepId,
		"status":        "running",
	}
	if input != nil {
		requestBody["input"] = input
	}

	// Theo api-context.md: POST /api/v1/ai/step-runs/insert-one (pattern CRUD)
	result, err := executePostRequest(client, "/v1/ai/step-runs/insert-one", requestBody, nil,
		"Create step run thành công", "Create step run thất bại. Thử lại lần thứ", true)
	return result, err
}

// FolkForm_UpdateStepRun update step run record
func FolkForm_UpdateStepRun(stepRunId string, output map[string]interface{}, status string) (map[string]interface{}, error) {
	if err := checkApiToken(); err != nil {
		return nil, err
	}

	client := createAuthorizedClient(defaultTimeout)
	updateData := map[string]interface{}{
		"status": status,
	}
	if output != nil {
		updateData["output"] = output
	}

	// Sử dụng endpoint: PUT /api/v1/ai/step-runs/update-by-id/:id (theo pattern CRUD chuẩn)
	endpoint := fmt.Sprintf("/v1/ai/step-runs/update-by-id/%s", stepRunId)
	result, err := executePutRequest(client, endpoint, updateData, nil,
		"Update step run thành công", "Update step run thất bại. Thử lại lần thứ", true)
	return result, err
}

// FolkForm_CreateAIRun tạo AI run record trong Module 2
func FolkForm_CreateAIRun(stepRunId, workflowRunId, promptTemplateId, providerProfileId, prompt string) (map[string]interface{}, error) {
	if err := checkApiToken(); err != nil {
		return nil, err
	}

	client := createAuthorizedClient(defaultTimeout)
	requestBody := map[string]interface{}{
		"stepRunId":         stepRunId,
		"promptTemplateId":  promptTemplateId,
		"providerProfileId": providerProfileId,
		"prompt":            prompt,
		"status":            "pending",
	}
	if workflowRunId != "" {
		requestBody["workflowRunId"] = workflowRunId
	}

	// Theo api-context.md: POST /api/v1/ai/ai-runs/insert-one (pattern CRUD)
	result, err := executePostRequest(client, "/v1/ai/ai-runs/insert-one", requestBody, nil,
		"Create AI run thành công", "Create AI run thất bại. Thử lại lần thứ", true)
	return result, err
}

// FolkForm_UpdateAIRun update AI run record
func FolkForm_UpdateAIRun(aiRunId string, response string, cost float64, latencyMs int64, status string) (map[string]interface{}, error) {
	if err := checkApiToken(); err != nil {
		return nil, err
	}

	client := createAuthorizedClient(defaultTimeout)
	updateData := map[string]interface{}{
		"status":   status,
		"response": response,
		"cost":     cost,
		"latency":  latencyMs,
	}

	// Sử dụng endpoint: PUT /api/v1/ai/ai-runs/update-by-id/:id (theo pattern CRUD chuẩn)
	endpoint := fmt.Sprintf("/v1/ai/ai-runs/update-by-id/%s", aiRunId)
	result, err := executePutRequest(client, endpoint, updateData, nil,
		"Update AI run thành công", "Update AI run thất bại. Thử lại lần thứ", true)
	return result, err
}

// FolkForm_CreateGenerationBatch tạo generation batch trong Module 2
func FolkForm_CreateGenerationBatch(stepRunId string, targetCount int) (map[string]interface{}, error) {
	if err := checkApiToken(); err != nil {
		return nil, err
	}

	client := createAuthorizedClient(defaultTimeout)
	requestBody := map[string]interface{}{
		"stepRunId":   stepRunId,
		"targetCount": targetCount,
	}

	// Theo api-context.md: POST /api/v1/ai/generation-batches/insert-one (pattern CRUD)
	result, err := executePostRequest(client, "/v1/ai/generation-batches/insert-one", requestBody, nil,
		"Create generation batch thành công", "Create generation batch thất bại. Thử lại lần thứ", true)
	return result, err
}

// FolkForm_CreateCandidate tạo candidate trong Module 2
func FolkForm_CreateCandidate(generationBatchId, aiRunId, text string) (map[string]interface{}, error) {
	if err := checkApiToken(); err != nil {
		return nil, err
	}

	client := createAuthorizedClient(defaultTimeout)
	requestBody := map[string]interface{}{
		"generationBatchId": generationBatchId,
		"createdByAIRunID":  aiRunId,
		"text":              text,
		"selected":          false,
	}

	// Theo api-context.md: POST /api/v1/ai/candidates/insert-one (pattern CRUD)
	result, err := executePostRequest(client, "/v1/ai/candidates/insert-one", requestBody, nil,
		"Create candidate thành công", "Create candidate thất bại. Thử lại lần thứ", true)
	return result, err
}

// FolkForm_CreateDraftNode tạo draft node trong Module 1
func FolkForm_CreateDraftNode(nodeType, text, parentDraftId, workflowRunId, candidateId string) (map[string]interface{}, error) {
	if err := checkApiToken(); err != nil {
		return nil, err
	}

	client := createAuthorizedClient(defaultTimeout)
	requestBody := map[string]interface{}{
		"type": nodeType,
		"text": text,
	}
	if parentDraftId != "" {
		requestBody["parentDraftId"] = parentDraftId
	}
	if workflowRunId != "" {
		requestBody["workflowRunId"] = workflowRunId
	}
	if candidateId != "" {
		requestBody["createdByCandidateID"] = candidateId
	}

	// Theo api-context.md: POST /api/v1/content/drafts/nodes/insert-one
	result, err := executePostRequest(client, "/v1/content/drafts/nodes/insert-one", requestBody, nil,
		"Create draft node thành công", "Create draft node thất bại. Thử lại lần thứ", true)
	return result, err
}

// FolkForm_UpdateWorkflowRun update workflow run status
// Sử dụng endpoint: PUT /api/v1/ai/workflow-runs/update-by-id/:id (theo pattern CRUD chuẩn)
func FolkForm_UpdateWorkflowRun(workflowRunId string, status string) (map[string]interface{}, error) {
	if err := checkApiToken(); err != nil {
		return nil, err
	}

	client := createAuthorizedClient(defaultTimeout)
	updateData := map[string]interface{}{
		"status": status,
	}

	endpoint := fmt.Sprintf("/v1/ai/workflow-runs/update-by-id/%s", workflowRunId)
	result, err := executePutRequest(client, endpoint, updateData, nil,
		"Update workflow run thành công", "Update workflow run thất bại. Thử lại lần thứ", true)
	return result, err
}

// FolkForm_UpdateWorkflowCommandHeartbeat update heartbeat và progress của workflow command
// Tham số:
// - agentId: ID của agent
// - commandID: ID của command
// - progress: Progress data (optional) - map[string]interface{} với step, percentage, message
// Trả về result map và error
func FolkForm_UpdateWorkflowCommandHeartbeat(agentId string, commandID string, progress map[string]interface{}) (map[string]interface{}, error) {
	if commandID == "" {
		return nil, errors.New("commandID không được để trống")
	}

	if err := checkApiToken(); err != nil {
		return nil, err
	}

	client := createAuthorizedClient(defaultTimeout)

	// Chuẩn bị request body
	requestBody := map[string]interface{}{
		"commandId": commandID,
	}
	if progress != nil {
		requestBody["progress"] = progress
	}

	// Sử dụng endpoint: /v1/ai/workflow-commands/update-heartbeat
	params := map[string]string{}
	if agentId != "" {
		params["agentId"] = agentId
	}

	result, err := executePostRequest(client, "/v1/ai/workflow-commands/update-heartbeat", requestBody, params,
		"", "Update heartbeat thất bại. Thử lại lần thứ", false) // Không log success để giảm log
	if err != nil {
		log.Printf("[FolkForm] [UpdateWorkflowCommandHeartbeat] ❌ Lỗi khi update heartbeat - commandID: %s, error: %v", commandID, err)
	}
	return result, err
}
