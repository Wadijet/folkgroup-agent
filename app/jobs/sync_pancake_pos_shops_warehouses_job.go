/*
Package jobs chứa các job cụ thể của ứng dụng.
File này chứa SyncPancakePosShopsWarehousesJob - job đồng bộ shop và warehouse từ Pancake POS.
*/
package jobs

import (
	"agent_pancake/app/integrations"
	"agent_pancake/app/scheduler"
	"context"
	"errors"
	"fmt"
	"log"
	"time"
)

// ANSI color codes cho terminal
const (
	colorReset = "\033[0m"
	colorRed   = "\033[31m"
)

// logError in log lỗi với màu đỏ để dễ theo dõi
func logError(format string, v ...interface{}) {
	message := fmt.Sprintf(format, v...)
	log.Printf("%s%s%s", colorRed, message, colorReset)
}

// SyncPancakePosShopsWarehousesJob là job đồng bộ shop và warehouse từ Pancake POS.
// Job này sẽ đồng bộ toàn bộ shops và warehouses từ Pancake POS về FolkForm.
// Sử dụng token lưu ở FolkForm với system: "Pancake POS".
// Sync Shop trước, Warehouse sau.
type SyncPancakePosShopsWarehousesJob struct {
	*scheduler.BaseJob
}

// NewSyncPancakePosShopsWarehousesJob tạo một instance mới của SyncPancakePosShopsWarehousesJob.
// Tham số:
// - name: Tên định danh của job
// - schedule: Biểu thức cron định nghĩa lịch chạy
// Trả về một instance của SyncPancakePosShopsWarehousesJob
func NewSyncPancakePosShopsWarehousesJob(name, schedule string) *SyncPancakePosShopsWarehousesJob {
	job := &SyncPancakePosShopsWarehousesJob{
		BaseJob: scheduler.NewBaseJob(name, schedule),
	}
	// Set callback function để BaseJob.Execute có thể gọi ExecuteInternal đúng cách
	job.BaseJob.SetExecuteInternalCallback(job.ExecuteInternal)
	return job
}

// ExecuteInternal thực thi logic đồng bộ shop và warehouse từ Pancake POS.
// Phương thức này gọi DoSyncPancakePosShopsWarehouses_v2() và thêm log wrapper cho job.
// Tham số:
// - ctx: Context để kiểm soát thời gian thực thi
// Trả về error nếu có lỗi xảy ra
func (j *SyncPancakePosShopsWarehousesJob) ExecuteInternal(ctx context.Context) error {
	startTime := time.Now()
	log.Printf("═══════════════════════════════════════════════════════════")
	log.Printf("🚀 JOB ĐÃ BẮT ĐẦU CHẠY: %s", j.GetName())
	log.Printf("📅 Lịch chạy: %s", j.GetSchedule())
	log.Printf("⏰ Thời gian bắt đầu: %s", startTime.Format("2006-01-02 15:04:05"))
	log.Printf("═══════════════════════════════════════════════════════════")

	// Gọi hàm logic thực sự
	err := DoSyncPancakePosShopsWarehouses_v2()
	if err != nil {
		duration := time.Since(startTime)
		log.Printf("═══════════════════════════════════════════════════════════")
		log.Printf("❌ JOB THẤT BẠI: %s", j.GetName())
		log.Printf("⏱️  Thời gian thực thi: %v", duration)
		log.Printf("❌ Lỗi: %v", err)
		log.Printf("═══════════════════════════════════════════════════════════")
		return err
	}

	duration := time.Since(startTime)
	log.Printf("═══════════════════════════════════════════════════════════")
	log.Printf("✅ JOB HOÀN THÀNH: %s", j.GetName())
	log.Printf("⏱️  Thời gian thực thi: %v", duration)
	log.Printf("⏰ Thời gian kết thúc: %s", time.Now().Format("2006-01-02 15:04:05"))
	log.Printf("═══════════════════════════════════════════════════════════")
	return nil
}

// DoSyncPancakePosShopsWarehouses_v2 thực thi logic đồng bộ shop và warehouse từ Pancake POS.
// Hàm này:
// 1. Lấy danh sách tokens từ FolkForm (system: "Pancake POS")
// 2. Với mỗi token, lấy danh sách shops từ Pancake POS
// 3. Upsert từng shop vào FolkForm
// 4. Với mỗi shop, lấy danh sách warehouses từ Pancake POS
// 5. Upsert từng warehouse vào FolkForm
// Hàm này có thể được gọi độc lập mà không cần thông qua job interface.
// Trả về error nếu có lỗi xảy ra
func DoSyncPancakePosShopsWarehouses_v2() error {
	// Thực hiện xác thực và đồng bộ dữ liệu cơ bản
	SyncBaseAuth()

	// Lấy danh sách tokens từ FolkForm với filter system: "Pancake POS"
	filter := `{"system":"Pancake POS"}`
	page := 1
	limit := 50

	log.Println("Bắt đầu đồng bộ shop và warehouse từ Pancake POS về FolkForm...")

	for {
		// Dừng nửa giây trước khi tiếp tục
		time.Sleep(100 * time.Millisecond)

		// Lấy danh sách access token với filter system: "Pancake POS"
		accessTokens, err := integrations.FolkForm_GetAccessTokens(page, limit, filter)
		if err != nil {
			logError("Lỗi khi lấy danh sách access token: %v", err)
			return errors.New("Lỗi khi lấy danh sách access token")
		}

		// Xử lý response - có thể là pagination object hoặc array trực tiếp
		items, itemCount, err := parseResponseData(accessTokens)
		if err != nil {
			logError("[DoSyncPancakePosShopsWarehouses_v2] LỖI khi parse response: %v", err)
			return err
		}
		log.Printf("[DoSyncPancakePosShopsWarehouses_v2] Nhận được %d access tokens (system: Pancake POS, page=%d, limit=%d)", len(items), page, limit)

		if itemCount > 0 && len(items) > 0 {
			// Với mỗi token
			for _, item := range items {
				// Dừng nửa giây trước khi tiếp tục
				time.Sleep(100 * time.Millisecond)

				// Chuyển item từ interface{} sang dạng map[string]interface{}
				itemMap, ok := item.(map[string]interface{})
				if !ok {
					logError("[DoSyncPancakePosShopsWarehouses_v2] LỖI: Item không phải là map: %T", item)
					continue
				}

				// Lấy api_key từ item (đã được filter ở server, chỉ còn tokens có system: "Pancake POS")
				apiKey, ok := itemMap["value"].(string)
				if !ok {
					logError("[DoSyncPancakePosShopsWarehouses_v2] LỖI: Không tìm thấy field 'value' trong item")
					continue
				}

				log.Printf("[DoSyncPancakePosShopsWarehouses_v2] Đang đồng bộ với API key (system: Pancake POS, length: %d)", len(apiKey))

				// 1. Đồng bộ Shops
				log.Println("[DoSyncPancakePosShopsWarehouses_v2] Bắt đầu đồng bộ shops...")
				shops, err := integrations.PancakePos_GetShops(apiKey)
				if err != nil {
					logError("[DoSyncPancakePosShopsWarehouses_v2] LỖI khi lấy danh sách shops: %v", err)
					// Tiếp tục với token tiếp theo nếu lỗi
					continue
				}

				log.Printf("[DoSyncPancakePosShopsWarehouses_v2] Nhận được %d shops", len(shops))

				// Upsert từng shop vào FolkForm
				for _, shop := range shops {
					// Dừng nửa giây trước khi tiếp tục
					time.Sleep(100 * time.Millisecond)

					shopMap, ok := shop.(map[string]interface{})
					if !ok {
						logError("[DoSyncPancakePosShopsWarehouses_v2] LỖI: Shop không phải là map: %T", shop)
						continue
					}

					_, err := integrations.FolkForm_UpsertShop(shopMap)
					if err != nil {
						logError("[DoSyncPancakePosShopsWarehouses_v2] LỖI khi upsert shop: %v", err)
						// Tiếp tục với shop tiếp theo nếu lỗi
						continue
					}
				}

				log.Printf("[DoSyncPancakePosShopsWarehouses_v2] Đã đồng bộ %d shops thành công", len(shops))

				// 2. Đồng bộ Warehouses (cho mỗi shop)
				log.Println("[DoSyncPancakePosShopsWarehouses_v2] Bắt đầu đồng bộ warehouses...")
				for _, shop := range shops {
					// Dừng nửa giây trước khi tiếp tục
					time.Sleep(100 * time.Millisecond)

					shopMap, ok := shop.(map[string]interface{})
					if !ok {
						logError("[DoSyncPancakePosShopsWarehouses_v2] LỖI: Shop không phải là map: %T", shop)
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
							logError("[DoSyncPancakePosShopsWarehouses_v2] LỖI: shopId không phải là số: %T", shopIdRaw)
							continue
						}
					} else {
						logError("[DoSyncPancakePosShopsWarehouses_v2] LỖI: Không tìm thấy field 'id' trong shop")
						continue
					}

					// Lấy danh sách warehouses cho shop này
					warehouses, err := integrations.PancakePos_GetWarehouses(apiKey, shopId)
					if err != nil {
						logError("[DoSyncPancakePosShopsWarehouses_v2] LỖI khi lấy danh sách warehouses cho shopId %d: %v", shopId, err)
						// Tiếp tục với shop tiếp theo nếu lỗi
						continue
					}

					log.Printf("[DoSyncPancakePosShopsWarehouses_v2] Nhận được %d warehouses cho shopId: %d", len(warehouses), shopId)

					// Upsert từng warehouse vào FolkForm
					for idx, warehouse := range warehouses {
						// Dừng nửa giây trước khi tiếp tục
						time.Sleep(100 * time.Millisecond)

						warehouseMap, ok := warehouse.(map[string]interface{})
						if !ok {
							logError("[DoSyncPancakePosShopsWarehouses_v2] LỖI: Warehouse không phải là map: %T", warehouse)
							continue
						}

						// Log warehouse data để debug
						if id, ok := warehouseMap["id"]; ok {
							log.Printf("[DoSyncPancakePosShopsWarehouses_v2] Đang upsert warehouse [%d/%d] - id: %v (type: %T)", idx+1, len(warehouses), id, id)
						} else {
							logError("[DoSyncPancakePosShopsWarehouses_v2] CẢNH BÁO: Warehouse [%d/%d] không có field 'id' - data: %+v", idx+1, len(warehouses), warehouseMap)
						}

						_, err := integrations.FolkForm_UpsertWarehouse(warehouseMap)
						if err != nil {
							logError("[DoSyncPancakePosShopsWarehouses_v2] LỖI khi upsert warehouse [%d/%d]: %v", idx+1, len(warehouses), err)
							// Tiếp tục với warehouse tiếp theo nếu lỗi
							continue
						}
						log.Printf("[DoSyncPancakePosShopsWarehouses_v2] ✅ Đã upsert warehouse [%d/%d] thành công", idx+1, len(warehouses))
					}

					log.Printf("[DoSyncPancakePosShopsWarehouses_v2] Đã đồng bộ %d warehouses cho shopId: %d", len(warehouses), shopId)
				}

				log.Printf("[DoSyncPancakePosShopsWarehouses_v2] Đã hoàn thành đồng bộ cho API key (length: %d)", len(apiKey))
			}

		} else {
			log.Println("[DoSyncPancakePosShopsWarehouses_v2] Không còn access token nào. Kết thúc.")
			break
		}

		page++
		continue
	}

	log.Println("Đồng bộ shop và warehouse từ Pancake POS về FolkForm thành công")
	return nil
}

// parseResponseData xử lý response data an toàn - hỗ trợ cả array và pagination object
// Trả về items ([]interface{}) và itemCount (float64)
// Hàm này được copy từ bridge.go để sử dụng trong job
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
