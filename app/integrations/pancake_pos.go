/*
Package integrations chứa các hàm tích hợp với các hệ thống bên ngoài.
File pancake_pos.go chứa các hàm gọi API từ Pancake POS để lấy dữ liệu:
- Shops (cửa hàng)
- Warehouses (kho hàng)
- Products (sản phẩm)
- Variations (biến thể sản phẩm)
- Categories (danh mục)
- Customers (khách hàng)
- Orders (đơn hàng)
Tất cả các hàm đều có retry logic và sử dụng adaptive rate limiter để tránh rate limit.
*/
package integrations

import (
	apputility "agent_pancake/app/utility"
	"agent_pancake/utility/httpclient"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"time"
)

// PancakePos_GetShops lấy danh sách shop từ Pancake POS API
// apiKey: API key từ FolkForm (system: "Pancake POS")
// Trả về: []interface{} chứa danh sách shops
func PancakePos_GetShops(apiKey string) (shops []interface{}, err error) {
	log.Printf("[PancakePOS] Bắt đầu lấy danh sách shops từ Pancake POS")
	log.Printf("[PancakePOS] Pancake POS Base URL: https://pos.pages.fm/api/v1")

	// Khởi tạo client
	client := httpclient.NewHttpClient("https://pos.pages.fm/api/v1", 60*time.Second)

	// Thiết lập params
	params := map[string]string{
		"api_key": apiKey,
	}

	// Số lần thử request
	requestCount := 0
	for {
		requestCount++
		log.Printf("[PancakePOS] [Lần thử %d/5] Bắt đầu lấy danh sách shops", requestCount)

		// Nếu số lần thử vượt quá 5 lần thì thoát vòng lặp
		if requestCount > 5 {
			logError("[PancakePOS] LỖI: Đã thử quá nhiều lần (%d/5). Thoát vòng lặp.", requestCount)
			return nil, errors.New("Đã thử quá nhiều lần. Thoát vòng lặp.")
		}

		// Sử dụng adaptive rate limiter để nghỉ trước khi gửi request
		// Pancake POS có thể dùng chung rate limiter với Pancake hoặc tạo riêng
		// Tạm thời dùng Pancake rate limiter
		rateLimiter := apputility.GetPancakeRateLimiter()
		rateLimiter.Wait()

		log.Printf("[PancakePOS] [Lần thử %d/5] Gửi GET request đến endpoint: /shops", requestCount)
		log.Printf("[PancakePOS] [Lần thử %d/5] Request params: api_key (length: %d)", requestCount, len(apiKey))

		// Gửi yêu cầu GET
		resp, err := client.GET("/shops", params)
		if err != nil {
			logError("[PancakePOS] [Lần thử %d/5] ❌ LỖI khi gọi API GET: %v", requestCount, err)
			log.Printf("[PancakePOS] [Lần thử %d/5] Request endpoint: /shops", requestCount)
			log.Printf("[PancakePOS] [Lần thử %d/5] 📝 Chi tiết lỗi: %s", requestCount, err.Error())
			continue
		}

		statusCode := resp.StatusCode
		log.Printf("[PancakePOS] [Lần thử %d/5] Response Status Code: %d", requestCount, statusCode)

		// Kiểm tra mã trạng thái, nếu không phải 200 thì thử lại
		if statusCode != 200 {
			// Đọc response body để log lỗi
			bodyBytes, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			var errorCode interface{}
			if readErr == nil {
				logError("[PancakePOS] [Lần thử %d/5] ❌ LỖI: Response Body (raw): %s", requestCount, string(bodyBytes))
				var errorResult map[string]interface{}
				if err := json.Unmarshal(bodyBytes, &errorResult); err == nil {
					logError("[PancakePOS] [Lần thử %d/5] ❌ LỖI: Response Body (parsed): %+v", requestCount, errorResult)
					// In message lỗi nếu có
					if message, ok := errorResult["message"].(string); ok {
						log.Printf("[PancakePOS] [Lần thử %d/5] 📝 Message lỗi từ Pancake POS: %s", requestCount, message)
					}
					if ec, ok := errorResult["error_code"]; ok {
						errorCode = ec
						log.Printf("[PancakePOS] [Lần thử %d/5] 🔢 Error Code: %v", requestCount, errorCode)
					}
				}
			} else {
				log.Printf("[PancakePOS] [Lần thử %d/5] ❌ Không thể đọc response body: %v", requestCount, readErr)
			}
			// Ghi nhận lỗi để điều chỉnh rate limiter
			rateLimiter.RecordFailure(statusCode, errorCode)
			log.Printf("[PancakePOS] [Lần thử %d/5] ⚠️ Status Code: %d - Lấy danh sách shops thất bại. Thử lại", requestCount, statusCode)
			continue
		}

		// Đọc dữ liệu từ phản hồi
		bodyBytes, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			log.Printf("[PancakePOS] [Lần thử %d/5] ❌ Không thể đọc response body: %v", requestCount, readErr)
			continue
		}

		// Parse response - có thể là array trực tiếp hoặc object có field "shops"
		var shopsArray []interface{}
		var result map[string]interface{}

		// Thử parse như object trước
		if err := json.Unmarshal(bodyBytes, &result); err == nil {
			// Nếu có field "shops" thì lấy từ đó
			if shopsRaw, ok := result["shops"]; ok {
				if shopsArrayRaw, ok := shopsRaw.([]interface{}); ok {
					shopsArray = shopsArrayRaw
				}
			} else {
				// Nếu không có field "shops", có thể là array trực tiếp
				// Thử parse lại như array
				if err := json.Unmarshal(bodyBytes, &shopsArray); err != nil {
					logError("[PancakePOS] [Lần thử %d/5] ❌ LỖI khi phân tích phản hồi JSON: %v", requestCount, err)
					log.Printf("[PancakePOS] [Lần thử %d/5] 📝 Response Body (raw): %s", requestCount, string(bodyBytes))
					continue
				}
			}
		} else {
			// Nếu không parse được như object, thử parse như array
			if err := json.Unmarshal(bodyBytes, &shopsArray); err != nil {
				logError("[PancakePOS] [Lần thử %d/5] ❌ LỖI khi phân tích phản hồi JSON: %v", requestCount, err)
				log.Printf("[PancakePOS] [Lần thử %d/5] 📝 Response Body (raw): %s", requestCount, string(bodyBytes))
				continue
			}
		}

		// Ghi nhận kết quả response để điều chỉnh rate limiter
		rateLimiter.RecordSuccess()

		log.Printf("[PancakePOS] Lấy danh sách shops thành công - Số lượng: %d", len(shopsArray))
		return shopsArray, nil
	}
}

// PancakePos_GetWarehouses lấy danh sách warehouse từ Pancake POS API
// apiKey: API key từ FolkForm
// shopId: ID của shop (integer)
// Trả về: []interface{} chứa danh sách warehouses
func PancakePos_GetWarehouses(apiKey string, shopId int) (warehouses []interface{}, err error) {
	log.Printf("[PancakePOS] Bắt đầu lấy danh sách warehouses từ Pancake POS - shopId: %d", shopId)
	log.Printf("[PancakePOS] Pancake POS Base URL: https://pos.pages.fm/api/v1")

	// Khởi tạo client
	client := httpclient.NewHttpClient("https://pos.pages.fm/api/v1", 60*time.Second)

	// Thiết lập params
	params := map[string]string{
		"api_key": apiKey,
	}

	// Số lần thử request
	requestCount := 0
	for {
		requestCount++
		log.Printf("[PancakePOS] [Lần thử %d/5] Bắt đầu lấy danh sách warehouses cho shopId: %d", requestCount, shopId)

		// Nếu số lần thử vượt quá 5 lần thì thoát vòng lặp
		if requestCount > 5 {
			logError("[PancakePOS] LỖI: Đã thử quá nhiều lần (%d/5). Thoát vòng lặp.", requestCount)
			return nil, errors.New("Đã thử quá nhiều lần. Thoát vòng lặp.")
		}

		// Sử dụng adaptive rate limiter để nghỉ trước khi gửi request
		rateLimiter := apputility.GetPancakeRateLimiter()
		rateLimiter.Wait()

		endpoint := fmt.Sprintf("/shops/%d/warehouses", shopId)
		log.Printf("[PancakePOS] [Lần thử %d/5] Gửi GET request đến endpoint: %s", requestCount, endpoint)
		log.Printf("[PancakePOS] [Lần thử %d/5] Request params: api_key (length: %d)", requestCount, len(apiKey))

		// Gửi yêu cầu GET
		resp, err := client.GET(endpoint, params)
		if err != nil {
			logError("[PancakePOS] [Lần thử %d/5] ❌ LỖI khi gọi API GET: %v", requestCount, err)
			log.Printf("[PancakePOS] [Lần thử %d/5] Request endpoint: %s", requestCount, endpoint)
			log.Printf("[PancakePOS] [Lần thử %d/5] 📝 Chi tiết lỗi: %s", requestCount, err.Error())
			continue
		}

		statusCode := resp.StatusCode
		log.Printf("[PancakePOS] [Lần thử %d/5] Response Status Code: %d", requestCount, statusCode)

		// Kiểm tra mã trạng thái, nếu không phải 200 thì thử lại
		if statusCode != 200 {
			// Đọc response body để log lỗi
			bodyBytes, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			var errorCode interface{}
			if readErr == nil {
				logError("[PancakePOS] [Lần thử %d/5] ❌ LỖI: Response Body (raw): %s", requestCount, string(bodyBytes))
				var errorResult map[string]interface{}
				if err := json.Unmarshal(bodyBytes, &errorResult); err == nil {
					logError("[PancakePOS] [Lần thử %d/5] ❌ LỖI: Response Body (parsed): %+v", requestCount, errorResult)
					// In message lỗi nếu có
					if message, ok := errorResult["message"].(string); ok {
						log.Printf("[PancakePOS] [Lần thử %d/5] 📝 Message lỗi từ Pancake POS: %s", requestCount, message)
					}
					if ec, ok := errorResult["error_code"]; ok {
						errorCode = ec
						log.Printf("[PancakePOS] [Lần thử %d/5] 🔢 Error Code: %v", requestCount, errorCode)
					}
				}
			} else {
				log.Printf("[PancakePOS] [Lần thử %d/5] ❌ Không thể đọc response body: %v", requestCount, readErr)
			}
			// Ghi nhận lỗi để điều chỉnh rate limiter
			rateLimiter.RecordFailure(statusCode, errorCode)
			log.Printf("[PancakePOS] [Lần thử %d/5] ⚠️ Status Code: %d - Lấy danh sách warehouses thất bại. Thử lại", requestCount, statusCode)
			continue
		}

		// Đọc dữ liệu từ phản hồi
		bodyBytes, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			log.Printf("[PancakePOS] [Lần thử %d/5] ❌ Không thể đọc response body: %v", requestCount, readErr)
			continue
		}

		// Parse response - format: {"data": [...], "success": true} hoặc array trực tiếp
		var warehousesArray []interface{}
		var result map[string]interface{}

		// Thử parse như object trước
		if err := json.Unmarshal(bodyBytes, &result); err == nil {
			// Nếu có field "data" thì lấy từ đó (format: {"data": [...], "success": true})
			if dataRaw, ok := result["data"]; ok {
				if dataArray, ok := dataRaw.([]interface{}); ok {
					warehousesArray = dataArray
				} else {
					logError("[PancakePOS] [Lần thử %d/5] ❌ LỖI: field 'data' không phải là array: %T", requestCount, dataRaw)
					log.Printf("[PancakePOS] [Lần thử %d/5] 📝 Response Body (raw): %s", requestCount, string(bodyBytes))
					continue
				}
			} else if warehousesRaw, ok := result["warehouses"]; ok {
				// Nếu có field "warehouses" thì lấy từ đó
				if warehousesArrayRaw, ok := warehousesRaw.([]interface{}); ok {
					warehousesArray = warehousesArrayRaw
				}
			} else {
				// Nếu không có field "data" hoặc "warehouses", có thể là array trực tiếp
				// Thử parse lại như array
				if err := json.Unmarshal(bodyBytes, &warehousesArray); err != nil {
					logError("[PancakePOS] [Lần thử %d/5] ❌ LỖI khi phân tích phản hồi JSON: %v", requestCount, err)
					log.Printf("[PancakePOS] [Lần thử %d/5] 📝 Response Body (raw): %s", requestCount, string(bodyBytes))
					continue
				}
			}
		} else {
			// Nếu không parse được như object, thử parse như array
			if err := json.Unmarshal(bodyBytes, &warehousesArray); err != nil {
				logError("[PancakePOS] [Lần thử %d/5] ❌ LỖI khi phân tích phản hồi JSON: %v", requestCount, err)
				log.Printf("[PancakePOS] [Lần thử %d/5] 📝 Response Body (raw): %s", requestCount, string(bodyBytes))
				continue
			}
		}

		// Ghi nhận kết quả response để điều chỉnh rate limiter
		rateLimiter.RecordSuccess()

		log.Printf("[PancakePOS] Lấy danh sách warehouses thành công - Số lượng: %d", len(warehousesArray))
		return warehousesArray, nil
	}
}

// PancakePos_GetCustomers lấy danh sách customers từ Pancake POS API
// apiKey: API key từ FolkForm (system: "Pancake POS")
// shopId: ID của shop (integer)
// pageNumber: Số trang (mặc định: 1)
// pageSize: Số lượng items mỗi trang (mặc định: 30)
// startTimeUpdatedAt: Thời gian bắt đầu (Unix timestamp, giây) - 0 nếu không filter
// endTimeUpdatedAt: Thời gian kết thúc (Unix timestamp, giây) - 0 nếu không filter
// Trả về: []interface{} chứa danh sách customers
func PancakePos_GetCustomers(apiKey string, shopId int, pageNumber int, pageSize int, startTimeUpdatedAt int64, endTimeUpdatedAt int64) (customers []interface{}, err error) {
	log.Printf("[PancakePOS] Bắt đầu lấy danh sách customers từ Pancake POS - shopId: %d, page: %d, size: %d, startTime: %d, endTime: %d", shopId, pageNumber, pageSize, startTimeUpdatedAt, endTimeUpdatedAt)
	log.Printf("[PancakePOS] Pancake POS Base URL: https://pos.pages.fm/api/v1")

	// Khởi tạo client
	client := httpclient.NewHttpClient("https://pos.pages.fm/api/v1", 60*time.Second)

	// Thiết lập params
	params := map[string]string{
		"api_key":     apiKey,
		"page_number": fmt.Sprintf("%d", pageNumber),
		"page_size":   fmt.Sprintf("%d", pageSize),
	}

	// Thêm start_time_updated_at và end_time_updated_at nếu có
	if startTimeUpdatedAt > 0 {
		params["start_time_updated_at"] = fmt.Sprintf("%d", startTimeUpdatedAt)
		log.Printf("[PancakePOS] Thêm param start_time_updated_at: %d", startTimeUpdatedAt)
	}
	if endTimeUpdatedAt > 0 {
		params["end_time_updated_at"] = fmt.Sprintf("%d", endTimeUpdatedAt)
		log.Printf("[PancakePOS] Thêm param end_time_updated_at: %d", endTimeUpdatedAt)
	}

	// Số lần thử request
	requestCount := 0
	for {
		requestCount++
		log.Printf("[PancakePOS] [Lần thử %d/5] Bắt đầu lấy danh sách customers cho shopId: %d", requestCount, shopId)

		// Nếu số lần thử vượt quá 5 lần thì thoát vòng lặp
		if requestCount > 5 {
			logError("[PancakePOS] LỖI: Đã thử quá nhiều lần (%d/5). Thoát vòng lặp.", requestCount)
			return nil, errors.New("Đã thử quá nhiều lần. Thoát vòng lặp.")
		}

		// Sử dụng adaptive rate limiter để nghỉ trước khi gửi request
		rateLimiter := apputility.GetPancakeRateLimiter()
		rateLimiter.Wait()

		endpoint := fmt.Sprintf("/shops/%d/customers", shopId)
		log.Printf("[PancakePOS] [Lần thử %d/5] Gửi GET request đến endpoint: %s", requestCount, endpoint)
		log.Printf("[PancakePOS] [Lần thử %d/5] Request params: api_key (length: %d), page_number=%d, page_size=%d", requestCount, len(apiKey), pageNumber, pageSize)
		if startTimeUpdatedAt > 0 {
			log.Printf("[PancakePOS] [Lần thử %d/5] start_time_updated_at: %d", requestCount, startTimeUpdatedAt)
		}
		if endTimeUpdatedAt > 0 {
			log.Printf("[PancakePOS] [Lần thử %d/5] end_time_updated_at: %d", requestCount, endTimeUpdatedAt)
		}

		// Gửi yêu cầu GET
		resp, err := client.GET(endpoint, params)
		if err != nil {
			logError("[PancakePOS] [Lần thử %d/5] ❌ LỖI khi gọi API GET: %v", requestCount, err)
			log.Printf("[PancakePOS] [Lần thử %d/5] Request endpoint: %s", requestCount, endpoint)
			log.Printf("[PancakePOS] [Lần thử %d/5] 📝 Chi tiết lỗi: %s", requestCount, err.Error())
			continue
		}

		statusCode := resp.StatusCode
		log.Printf("[PancakePOS] [Lần thử %d/5] Response Status Code: %d", requestCount, statusCode)

		// Kiểm tra mã trạng thái, nếu không phải 200 thì thử lại
		if statusCode != 200 {
			// Đọc response body để log lỗi
			bodyBytes, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			var errorCode interface{}
			if readErr == nil {
				logError("[PancakePOS] [Lần thử %d/5] ❌ LỖI: Response Body (raw): %s", requestCount, string(bodyBytes))
				var errorResult map[string]interface{}
				if err := json.Unmarshal(bodyBytes, &errorResult); err == nil {
					logError("[PancakePOS] [Lần thử %d/5] ❌ LỖI: Response Body (parsed): %+v", requestCount, errorResult)
					// In message lỗi nếu có
					if message, ok := errorResult["message"].(string); ok {
						log.Printf("[PancakePOS] [Lần thử %d/5] 📝 Message lỗi từ Pancake POS: %s", requestCount, message)
					}
					if ec, ok := errorResult["error_code"]; ok {
						errorCode = ec
						log.Printf("[PancakePOS] [Lần thử %d/5] 🔢 Error Code: %v", requestCount, errorCode)
					}
				}
			} else {
				log.Printf("[PancakePOS] [Lần thử %d/5] ❌ Không thể đọc response body: %v", requestCount, readErr)
			}
			// Ghi nhận lỗi để điều chỉnh rate limiter
			rateLimiter.RecordFailure(statusCode, errorCode)
			log.Printf("[PancakePOS] [Lần thử %d/5] ⚠️ Status Code: %d - Lấy danh sách customers thất bại. Thử lại", requestCount, statusCode)
			continue
		}

		// Đọc dữ liệu từ phản hồi
		bodyBytes, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			log.Printf("[PancakePOS] [Lần thử %d/5] ❌ Không thể đọc response body: %v", requestCount, readErr)
			continue
		}

		// Parse response - có thể là array trực tiếp hoặc object có field "customers" hoặc "data"
		var customersArray []interface{}
		var result map[string]interface{}

		// Thử parse như object trước
		if err := json.Unmarshal(bodyBytes, &result); err == nil {
			// Nếu có field "customers" thì lấy từ đó
			if customersRaw, ok := result["customers"]; ok {
				if customersArrayRaw, ok := customersRaw.([]interface{}); ok {
					customersArray = customersArrayRaw
				}
			} else if dataRaw, ok := result["data"]; ok {
				// Nếu có field "data" thì lấy từ đó (format: {"data": [...], "success": true})
				if dataArray, ok := dataRaw.([]interface{}); ok {
					customersArray = dataArray
				}
			} else {
				// Nếu không có field "customers" hoặc "data", có thể là array trực tiếp
				// Thử parse lại như array
				if err := json.Unmarshal(bodyBytes, &customersArray); err != nil {
					logError("[PancakePOS] [Lần thử %d/5] ❌ LỖI khi phân tích phản hồi JSON: %v", requestCount, err)
					log.Printf("[PancakePOS] [Lần thử %d/5] 📝 Response Body (raw): %s", requestCount, string(bodyBytes))
					continue
				}
			}
		} else {
			// Nếu không parse được như object, thử parse như array
			if err := json.Unmarshal(bodyBytes, &customersArray); err != nil {
				logError("[PancakePOS] [Lần thử %d/5] ❌ LỖI khi phân tích phản hồi JSON: %v", requestCount, err)
				log.Printf("[PancakePOS] [Lần thử %d/5] 📝 Response Body (raw): %s", requestCount, string(bodyBytes))
				continue
			}
		}

		// Ghi nhận kết quả response để điều chỉnh rate limiter
		rateLimiter.RecordSuccess()

		log.Printf("[PancakePOS] Lấy danh sách customers thành công - Số lượng: %d", len(customersArray))
		return customersArray, nil
	}
}

// PancakePos_GetProducts lấy danh sách products từ Pancake POS API
// apiKey: API key từ FolkForm (system: "Pancake POS")
// shopId: ID của shop (integer)
// pageNumber: Số trang (mặc định: 1)
// pageSize: Số lượng items mỗi trang (mặc định: 30)
// Trả về: []interface{} chứa danh sách products
func PancakePos_GetProducts(apiKey string, shopId int, pageNumber int, pageSize int) (products []interface{}, err error) {
	log.Printf("[PancakePOS] Bắt đầu lấy danh sách products từ Pancake POS - shopId: %d, page: %d, size: %d", shopId, pageNumber, pageSize)
	log.Printf("[PancakePOS] Pancake POS Base URL: https://pos.pages.fm/api/v1")

	// Khởi tạo client
	client := httpclient.NewHttpClient("https://pos.pages.fm/api/v1", 60*time.Second)

	// Thiết lập params
	params := map[string]string{
		"api_key":     apiKey,
		"page_number": fmt.Sprintf("%d", pageNumber),
		"page_size":   fmt.Sprintf("%d", pageSize),
	}

	// Số lần thử request
	requestCount := 0
	for {
		requestCount++
		log.Printf("[PancakePOS] [Lần thử %d/5] Bắt đầu lấy danh sách products cho shopId: %d", requestCount, shopId)

		// Nếu số lần thử vượt quá 5 lần thì thoát vòng lặp
		if requestCount > 5 {
			logError("[PancakePOS] LỖI: Đã thử quá nhiều lần (%d/5). Thoát vòng lặp.", requestCount)
			return nil, errors.New("Đã thử quá nhiều lần. Thoát vòng lặp.")
		}

		// Sử dụng adaptive rate limiter để nghỉ trước khi gửi request
		rateLimiter := apputility.GetPancakeRateLimiter()
		rateLimiter.Wait()

		endpoint := fmt.Sprintf("/shops/%d/products", shopId)
		log.Printf("[PancakePOS] [Lần thử %d/5] Gửi GET request đến endpoint: %s", requestCount, endpoint)
		log.Printf("[PancakePOS] [Lần thử %d/5] Request params: api_key (length: %d), page_number=%d, page_size=%d", requestCount, len(apiKey), pageNumber, pageSize)

		// Gửi yêu cầu GET
		resp, err := client.GET(endpoint, params)
		if err != nil {
			logError("[PancakePOS] [Lần thử %d/5] ❌ LỖI khi gọi API GET: %v", requestCount, err)
			log.Printf("[PancakePOS] [Lần thử %d/5] Request endpoint: %s", requestCount, endpoint)
			log.Printf("[PancakePOS] [Lần thử %d/5] 📝 Chi tiết lỗi: %s", requestCount, err.Error())
			continue
		}

		statusCode := resp.StatusCode
		log.Printf("[PancakePOS] [Lần thử %d/5] Response Status Code: %d", requestCount, statusCode)

		// Kiểm tra mã trạng thái, nếu không phải 200 thì thử lại
		if statusCode != 200 {
			// Đọc response body để log lỗi
			bodyBytes, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			var errorCode interface{}
			if readErr == nil {
				logError("[PancakePOS] [Lần thử %d/5] ❌ LỖI: Response Body (raw): %s", requestCount, string(bodyBytes))
				var errorResult map[string]interface{}
				if err := json.Unmarshal(bodyBytes, &errorResult); err == nil {
					logError("[PancakePOS] [Lần thử %d/5] ❌ LỖI: Response Body (parsed): %+v", requestCount, errorResult)
					// In message lỗi nếu có
					if message, ok := errorResult["message"].(string); ok {
						log.Printf("[PancakePOS] [Lần thử %d/5] 📝 Message lỗi từ Pancake POS: %s", requestCount, message)
					}
					if ec, ok := errorResult["error_code"]; ok {
						errorCode = ec
						log.Printf("[PancakePOS] [Lần thử %d/5] 🔢 Error Code: %v", requestCount, errorCode)
					}
				}
			} else {
				log.Printf("[PancakePOS] [Lần thử %d/5] ❌ Không thể đọc response body: %v", requestCount, readErr)
			}
			// Ghi nhận lỗi để điều chỉnh rate limiter
			rateLimiter.RecordFailure(statusCode, errorCode)
			log.Printf("[PancakePOS] [Lần thử %d/5] ⚠️ Status Code: %d - Lấy danh sách products thất bại. Thử lại", requestCount, statusCode)
			continue
		}

		// Đọc dữ liệu từ phản hồi
		bodyBytes, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			log.Printf("[PancakePOS] [Lần thử %d/5] ❌ Không thể đọc response body: %v", requestCount, readErr)
			continue
		}

		// Parse response - có thể là array trực tiếp hoặc object có field "products" hoặc "data"
		var productsArray []interface{}
		var result map[string]interface{}

		// Thử parse như object trước
		if err := json.Unmarshal(bodyBytes, &result); err == nil {
			// Nếu có field "products" thì lấy từ đó
			if productsRaw, ok := result["products"]; ok {
				if productsArrayRaw, ok := productsRaw.([]interface{}); ok {
					productsArray = productsArrayRaw
				}
			} else if dataRaw, ok := result["data"]; ok {
				// Nếu có field "data" thì lấy từ đó (format: {"data": [...], "success": true})
				if dataArray, ok := dataRaw.([]interface{}); ok {
					productsArray = dataArray
				}
			} else {
				// Nếu không có field "products" hoặc "data", có thể là array trực tiếp
				// Thử parse lại như array
				if err := json.Unmarshal(bodyBytes, &productsArray); err != nil {
					logError("[PancakePOS] [Lần thử %d/5] ❌ LỖI khi phân tích phản hồi JSON: %v", requestCount, err)
					log.Printf("[PancakePOS] [Lần thử %d/5] 📝 Response Body (raw): %s", requestCount, string(bodyBytes))
					continue
				}
			}
		} else {
			// Nếu không parse được như object, thử parse như array
			if err := json.Unmarshal(bodyBytes, &productsArray); err != nil {
				logError("[PancakePOS] [Lần thử %d/5] ❌ LỖI khi phân tích phản hồi JSON: %v", requestCount, err)
				log.Printf("[PancakePOS] [Lần thử %d/5] 📝 Response Body (raw): %s", requestCount, string(bodyBytes))
				continue
			}
		}

		// Ghi nhận kết quả response để điều chỉnh rate limiter
		rateLimiter.RecordSuccess()

		log.Printf("[PancakePOS] Lấy danh sách products thành công - Số lượng: %d", len(productsArray))
		return productsArray, nil
	}
}

// PancakePos_GetVariations lấy danh sách variations từ Pancake POS API
// apiKey: API key từ FolkForm
// shopId: ID của shop (integer)
// productId: ID của product (integer, 0 nếu lấy tất cả)
// pageNumber: Số trang
// pageSize: Số lượng items mỗi trang
// Trả về: []interface{} chứa danh sách variations
func PancakePos_GetVariations(apiKey string, shopId int, productId int, pageNumber int, pageSize int) (variations []interface{}, err error) {
	log.Printf("[PancakePOS] Bắt đầu lấy danh sách variations từ Pancake POS - shopId: %d, productId: %d, page: %d, size: %d", shopId, productId, pageNumber, pageSize)
	log.Printf("[PancakePOS] Pancake POS Base URL: https://pos.pages.fm/api/v1")

	// Khởi tạo client
	client := httpclient.NewHttpClient("https://pos.pages.fm/api/v1", 60*time.Second)

	// Thiết lập params
	params := map[string]string{
		"api_key":     apiKey,
		"page_number": fmt.Sprintf("%d", pageNumber),
		"page_size":   fmt.Sprintf("%d", pageSize),
	}

	// Thêm product_id nếu có
	if productId > 0 {
		params["product_id"] = fmt.Sprintf("%d", productId)
	}

	// Số lần thử request
	requestCount := 0
	for {
		requestCount++
		log.Printf("[PancakePOS] [Lần thử %d/5] Bắt đầu lấy danh sách variations cho shopId: %d, productId: %d", requestCount, shopId, productId)

		// Nếu số lần thử vượt quá 5 lần thì thoát vòng lặp
		if requestCount > 5 {
			logError("[PancakePOS] LỖI: Đã thử quá nhiều lần (%d/5). Thoát vòng lặp.", requestCount)
			return nil, errors.New("Đã thử quá nhiều lần. Thoát vòng lặp.")
		}

		// Sử dụng adaptive rate limiter để nghỉ trước khi gửi request
		rateLimiter := apputility.GetPancakeRateLimiter()
		rateLimiter.Wait()

		endpoint := fmt.Sprintf("/shops/%d/products/variations", shopId)
		log.Printf("[PancakePOS] [Lần thử %d/5] Gửi GET request đến endpoint: %s", requestCount, endpoint)
		log.Printf("[PancakePOS] [Lần thử %d/5] Request params: api_key (length: %d), page_number=%d, page_size=%d", requestCount, len(apiKey), pageNumber, pageSize)
		if productId > 0 {
			log.Printf("[PancakePOS] [Lần thử %d/5] product_id: %d", requestCount, productId)
		}

		// Gửi yêu cầu GET
		resp, err := client.GET(endpoint, params)
		if err != nil {
			logError("[PancakePOS] [Lần thử %d/5] ❌ LỖI khi gọi API GET: %v", requestCount, err)
			log.Printf("[PancakePOS] [Lần thử %d/5] Request endpoint: %s", requestCount, endpoint)
			log.Printf("[PancakePOS] [Lần thử %d/5] 📝 Chi tiết lỗi: %s", requestCount, err.Error())
			continue
		}

		statusCode := resp.StatusCode
		log.Printf("[PancakePOS] [Lần thử %d/5] Response Status Code: %d", requestCount, statusCode)

		// Kiểm tra mã trạng thái, nếu không phải 200 thì thử lại
		if statusCode != 200 {
			// Đọc response body để log lỗi
			bodyBytes, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			var errorCode interface{}
			if readErr == nil {
				logError("[PancakePOS] [Lần thử %d/5] ❌ LỖI: Response Body (raw): %s", requestCount, string(bodyBytes))
				var errorResult map[string]interface{}
				if err := json.Unmarshal(bodyBytes, &errorResult); err == nil {
					logError("[PancakePOS] [Lần thử %d/5] ❌ LỖI: Response Body (parsed): %+v", requestCount, errorResult)
					// In message lỗi nếu có
					if message, ok := errorResult["message"].(string); ok {
						log.Printf("[PancakePOS] [Lần thử %d/5] 📝 Message lỗi từ Pancake POS: %s", requestCount, message)
					}
					if ec, ok := errorResult["error_code"]; ok {
						errorCode = ec
						log.Printf("[PancakePOS] [Lần thử %d/5] 🔢 Error Code: %v", requestCount, errorCode)
					}
				}
			} else {
				log.Printf("[PancakePOS] [Lần thử %d/5] ❌ Không thể đọc response body: %v", requestCount, readErr)
			}
			// Ghi nhận lỗi để điều chỉnh rate limiter
			rateLimiter.RecordFailure(statusCode, errorCode)
			log.Printf("[PancakePOS] [Lần thử %d/5] ⚠️ Status Code: %d - Lấy danh sách variations thất bại. Thử lại", requestCount, statusCode)
			continue
		}

		// Đọc dữ liệu từ phản hồi
		bodyBytes, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			log.Printf("[PancakePOS] [Lần thử %d/5] ❌ Không thể đọc response body: %v", requestCount, readErr)
			continue
		}

		// Parse response - có thể là array trực tiếp hoặc object có field "variations" hoặc "data"
		var variationsArray []interface{}
		var result map[string]interface{}

		// Thử parse như object trước
		if err := json.Unmarshal(bodyBytes, &result); err == nil {
			// Nếu có field "variations" thì lấy từ đó
			if variationsRaw, ok := result["variations"]; ok {
				if variationsArrayRaw, ok := variationsRaw.([]interface{}); ok {
					variationsArray = variationsArrayRaw
				}
			} else if dataRaw, ok := result["data"]; ok {
				// Nếu có field "data" thì lấy từ đó (format: {"data": [...], "success": true})
				if dataArray, ok := dataRaw.([]interface{}); ok {
					variationsArray = dataArray
				}
			} else {
				// Nếu không có field "variations" hoặc "data", có thể là array trực tiếp
				// Thử parse lại như array
				if err := json.Unmarshal(bodyBytes, &variationsArray); err != nil {
					logError("[PancakePOS] [Lần thử %d/5] ❌ LỖI khi phân tích phản hồi JSON: %v", requestCount, err)
					log.Printf("[PancakePOS] [Lần thử %d/5] 📝 Response Body (raw): %s", requestCount, string(bodyBytes))
					continue
				}
			}
		} else {
			// Nếu không parse được như object, thử parse như array
			if err := json.Unmarshal(bodyBytes, &variationsArray); err != nil {
				logError("[PancakePOS] [Lần thử %d/5] ❌ LỖI khi phân tích phản hồi JSON: %v", requestCount, err)
				log.Printf("[PancakePOS] [Lần thử %d/5] 📝 Response Body (raw): %s", requestCount, string(bodyBytes))
				continue
			}
		}

		// Ghi nhận kết quả response để điều chỉnh rate limiter
		rateLimiter.RecordSuccess()

		log.Printf("[PancakePOS] Lấy danh sách variations thành công - Số lượng: %d", len(variationsArray))
		return variationsArray, nil
	}
}

// PancakePos_GetCategories lấy danh sách categories từ Pancake POS API
// apiKey: API key từ FolkForm
// shopId: ID của shop (integer)
// Trả về: []interface{} chứa danh sách categories
func PancakePos_GetCategories(apiKey string, shopId int) (categories []interface{}, err error) {
	log.Printf("[PancakePOS] Bắt đầu lấy danh sách categories từ Pancake POS - shopId: %d", shopId)
	log.Printf("[PancakePOS] Pancake POS Base URL: https://pos.pages.fm/api/v1")

	// Khởi tạo client
	client := httpclient.NewHttpClient("https://pos.pages.fm/api/v1", 60*time.Second)

	// Thiết lập params
	params := map[string]string{
		"api_key": apiKey,
	}

	// Số lần thử request
	requestCount := 0
	for {
		requestCount++
		log.Printf("[PancakePOS] [Lần thử %d/5] Bắt đầu lấy danh sách categories cho shopId: %d", requestCount, shopId)

		// Nếu số lần thử vượt quá 5 lần thì thoát vòng lặp
		if requestCount > 5 {
			logError("[PancakePOS] LỖI: Đã thử quá nhiều lần (%d/5). Thoát vòng lặp.", requestCount)
			return nil, errors.New("Đã thử quá nhiều lần. Thoát vòng lặp.")
		}

		// Sử dụng adaptive rate limiter để nghỉ trước khi gửi request
		rateLimiter := apputility.GetPancakeRateLimiter()
		rateLimiter.Wait()

		endpoint := fmt.Sprintf("/shops/%d/categories", shopId)
		log.Printf("[PancakePOS] [Lần thử %d/5] Gửi GET request đến endpoint: %s", requestCount, endpoint)
		log.Printf("[PancakePOS] [Lần thử %d/5] Request params: api_key (length: %d)", requestCount, len(apiKey))

		// Gửi yêu cầu GET
		resp, err := client.GET(endpoint, params)
		if err != nil {
			logError("[PancakePOS] [Lần thử %d/5] ❌ LỖI khi gọi API GET: %v", requestCount, err)
			log.Printf("[PancakePOS] [Lần thử %d/5] Request endpoint: %s", requestCount, endpoint)
			log.Printf("[PancakePOS] [Lần thử %d/5] 📝 Chi tiết lỗi: %s", requestCount, err.Error())
			continue
		}

		statusCode := resp.StatusCode
		log.Printf("[PancakePOS] [Lần thử %d/5] Response Status Code: %d", requestCount, statusCode)

		// Kiểm tra mã trạng thái, nếu không phải 200 thì thử lại
		if statusCode != 200 {
			// Đọc response body để log lỗi
			bodyBytes, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			var errorCode interface{}
			if readErr == nil {
				logError("[PancakePOS] [Lần thử %d/5] ❌ LỖI: Response Body (raw): %s", requestCount, string(bodyBytes))
				var errorResult map[string]interface{}
				if err := json.Unmarshal(bodyBytes, &errorResult); err == nil {
					logError("[PancakePOS] [Lần thử %d/5] ❌ LỖI: Response Body (parsed): %+v", requestCount, errorResult)
					// In message lỗi nếu có
					if message, ok := errorResult["message"].(string); ok {
						log.Printf("[PancakePOS] [Lần thử %d/5] 📝 Message lỗi từ Pancake POS: %s", requestCount, message)
					}
					if ec, ok := errorResult["error_code"]; ok {
						errorCode = ec
						log.Printf("[PancakePOS] [Lần thử %d/5] 🔢 Error Code: %v", requestCount, errorCode)
					}
				}
			} else {
				log.Printf("[PancakePOS] [Lần thử %d/5] ❌ Không thể đọc response body: %v", requestCount, readErr)
			}
			// Ghi nhận lỗi để điều chỉnh rate limiter
			rateLimiter.RecordFailure(statusCode, errorCode)
			log.Printf("[PancakePOS] [Lần thử %d/5] ⚠️ Status Code: %d - Lấy danh sách categories thất bại. Thử lại", requestCount, statusCode)
			continue
		}

		// Đọc dữ liệu từ phản hồi
		bodyBytes, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			log.Printf("[PancakePOS] [Lần thử %d/5] ❌ Không thể đọc response body: %v", requestCount, readErr)
			continue
		}

		// Parse response - có thể là array trực tiếp hoặc object có field "categories" hoặc "data"
		var categoriesArray []interface{}
		var result map[string]interface{}

		// Thử parse như object trước
		if err := json.Unmarshal(bodyBytes, &result); err == nil {
			// Nếu có field "categories" thì lấy từ đó
			if categoriesRaw, ok := result["categories"]; ok {
				if categoriesArrayRaw, ok := categoriesRaw.([]interface{}); ok {
					categoriesArray = categoriesArrayRaw
				}
			} else if dataRaw, ok := result["data"]; ok {
				// Nếu có field "data" thì lấy từ đó (format: {"data": [...], "success": true})
				if dataArray, ok := dataRaw.([]interface{}); ok {
					categoriesArray = dataArray
				}
			} else {
				// Nếu không có field "categories" hoặc "data", có thể là array trực tiếp
				// Thử parse lại như array
				if err := json.Unmarshal(bodyBytes, &categoriesArray); err != nil {
					logError("[PancakePOS] [Lần thử %d/5] ❌ LỖI khi phân tích phản hồi JSON: %v", requestCount, err)
					log.Printf("[PancakePOS] [Lần thử %d/5] 📝 Response Body (raw): %s", requestCount, string(bodyBytes))
					continue
				}
			}
		} else {
			// Nếu không parse được như object, thử parse như array
			if err := json.Unmarshal(bodyBytes, &categoriesArray); err != nil {
				logError("[PancakePOS] [Lần thử %d/5] ❌ LỖI khi phân tích phản hồi JSON: %v", requestCount, err)
				log.Printf("[PancakePOS] [Lần thử %d/5] 📝 Response Body (raw): %s", requestCount, string(bodyBytes))
				continue
			}
		}

		// Ghi nhận kết quả response để điều chỉnh rate limiter
		rateLimiter.RecordSuccess()

		log.Printf("[PancakePOS] Lấy danh sách categories thành công - Số lượng: %d", len(categoriesArray))
		return categoriesArray, nil
	}
}

// PancakePos_GetOrders lấy danh sách orders từ Pancake POS API
// apiKey: API key từ FolkForm (system: "Pancake POS")
// shopId: ID của shop (integer)
// pageNumber: Số trang (mặc định: 1)
// pageSize: Số lượng items mỗi trang (mặc định: 30, tối đa: 100)
// updateStatus: Sắp xếp theo thời gian ("inserted_at", "updated_at", "paid_at", etc.)
// Trả về: map[string]interface{} chứa orders và pagination
func PancakePos_GetOrders(apiKey string, shopId int, pageNumber int, pageSize int, updateStatus string) (result map[string]interface{}, err error) {
	log.Printf("[PancakePOS] Bắt đầu lấy danh sách orders từ Pancake POS - shopId: %d, page: %d, size: %d, updateStatus: %s", shopId, pageNumber, pageSize, updateStatus)
	log.Printf("[PancakePOS] Pancake POS Base URL: https://pos.pages.fm/api/v1")

	// Khởi tạo client
	client := httpclient.NewHttpClient("https://pos.pages.fm/api/v1", 60*time.Second)

	// Thiết lập params
	params := map[string]string{
		"api_key":     apiKey,
		"page_number": fmt.Sprintf("%d", pageNumber),
		"page_size":   fmt.Sprintf("%d", pageSize),
	}
	if updateStatus != "" {
		params["updateStatus"] = updateStatus
	}

	// Số lần thử request
	requestCount := 0
	for {
		requestCount++
		log.Printf("[PancakePOS] [Lần thử %d/5] Bắt đầu lấy danh sách orders cho shopId: %d", requestCount, shopId)

		// Nếu số lần thử vượt quá 5 lần thì thoát vòng lặp
		if requestCount > 5 {
			logError("[PancakePOS] LỖI: Đã thử quá nhiều lần (%d/5). Thoát vòng lặp.", requestCount)
			return nil, errors.New("Đã thử quá nhiều lần. Thoát vòng lặp.")
		}

		// Sử dụng adaptive rate limiter để nghỉ trước khi gửi request
		rateLimiter := apputility.GetPancakeRateLimiter()
		rateLimiter.Wait()

		endpoint := fmt.Sprintf("/shops/%d/orders", shopId)
		log.Printf("[PancakePOS] [Lần thử %d/5] Gửi GET request đến endpoint: %s", requestCount, endpoint)
		log.Printf("[PancakePOS] [Lần thử %d/5] Request params: api_key (length: %d), page_number=%d, page_size=%d, updateStatus=%s", requestCount, len(apiKey), pageNumber, pageSize, updateStatus)

		// Gửi yêu cầu GET
		resp, err := client.GET(endpoint, params)
		if err != nil {
			logError("[PancakePOS] [Lần thử %d/5] ❌ LỖI khi gọi API GET: %v", requestCount, err)
			log.Printf("[PancakePOS] [Lần thử %d/5] Request endpoint: %s", requestCount, endpoint)
			log.Printf("[PancakePOS] [Lần thử %d/5] 📝 Chi tiết lỗi: %s", requestCount, err.Error())
			continue
		}

		statusCode := resp.StatusCode
		log.Printf("[PancakePOS] [Lần thử %d/5] Response Status Code: %d", requestCount, statusCode)

		// Kiểm tra mã trạng thái, nếu không phải 200 thì thử lại
		if statusCode != 200 {
			// Đọc response body để log lỗi
			bodyBytes, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			var errorCode interface{}
			if readErr == nil {
				logError("[PancakePOS] [Lần thử %d/5] ❌ LỖI: Response Body (raw): %s", requestCount, string(bodyBytes))
				var errorResult map[string]interface{}
				if err := json.Unmarshal(bodyBytes, &errorResult); err == nil {
					logError("[PancakePOS] [Lần thử %d/5] ❌ LỖI: Response Body (parsed): %+v", requestCount, errorResult)
					// In message lỗi nếu có
					if message, ok := errorResult["message"].(string); ok {
						log.Printf("[PancakePOS] [Lần thử %d/5] 📝 Message lỗi từ Pancake POS: %s", requestCount, message)
					}
					if ec, ok := errorResult["error_code"]; ok {
						errorCode = ec
						log.Printf("[PancakePOS] [Lần thử %d/5] 🔢 Error Code: %v", requestCount, errorCode)
					}
				}
			} else {
				log.Printf("[PancakePOS] [Lần thử %d/5] ❌ Không thể đọc response body: %v", requestCount, readErr)
			}
			// Ghi nhận lỗi để điều chỉnh rate limiter
			rateLimiter.RecordFailure(statusCode, errorCode)
			log.Printf("[PancakePOS] [Lần thử %d/5] ⚠️ Status Code: %d - Lấy danh sách orders thất bại. Thử lại", requestCount, statusCode)
			continue
		}

		// Đọc dữ liệu từ phản hồi
		bodyBytes, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			log.Printf("[PancakePOS] [Lần thử %d/5] ❌ Không thể đọc response body: %v", requestCount, readErr)
			continue
		}

		// Parse response - có thể là object với field "data" hoặc array trực tiếp
		var resultMap map[string]interface{}
		var ordersArray []interface{}

		// Thử parse như object trước
		if err := json.Unmarshal(bodyBytes, &resultMap); err == nil {
			// Nếu có field "data", lấy từ đó
			if data, ok := resultMap["data"].([]interface{}); ok {
				ordersArray = data
				log.Printf("[PancakePOS] [Lần thử %d/5] Parse response thành công - Tìm thấy field 'data' với %d orders", requestCount, len(ordersArray))
			} else {
				// Nếu không có field "data", có thể toàn bộ response là array
				log.Printf("[PancakePOS] [Lần thử %d/5] Không tìm thấy field 'data' trong response object, thử parse như array", requestCount)
				// Thử parse lại như array
				if err := json.Unmarshal(bodyBytes, &ordersArray); err != nil {
					logError("[PancakePOS] [Lần thử %d/5] ❌ LỖI khi phân tích phản hồi JSON: %v", requestCount, err)
					log.Printf("[PancakePOS] [Lần thử %d/5] 📝 Response Body (raw): %s", requestCount, string(bodyBytes))
					continue
				}
			}
		} else {
			// Nếu không parse được như object, thử parse như array
			if err := json.Unmarshal(bodyBytes, &ordersArray); err != nil {
				logError("[PancakePOS] [Lần thử %d/5] ❌ LỖI khi phân tích phản hồi JSON: %v", requestCount, err)
				log.Printf("[PancakePOS] [Lần thử %d/5] 📝 Response Body (raw): %s", requestCount, string(bodyBytes))
				continue
			}
		}

		// Ghi nhận kết quả response để điều chỉnh rate limiter
		rateLimiter.RecordSuccess()

		// Tạo result map với orders và pagination
		result = map[string]interface{}{
			"orders": ordersArray,
		}

		// Thêm pagination nếu có
		if pagination, ok := resultMap["pagination"].(map[string]interface{}); ok {
			result["pagination"] = pagination
		} else {
			// Tạo pagination mặc định từ response
			result["pagination"] = map[string]interface{}{
				"page_number": pageNumber,
				"page_size":   pageSize,
				"total":       len(ordersArray),
			}
		}

		log.Printf("[PancakePOS] Lấy danh sách orders thành công - Số lượng: %d", len(ordersArray))
		return result, nil
	}
}
