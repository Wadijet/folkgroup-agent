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
	"time"

	"github.com/sirupsen/logrus"
)

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
	LogJobStart(j.GetName(), j.GetSchedule()).WithFields(map[string]interface{}{
		"start_time": startTime.Format("2006-01-02 15:04:05"),
	}).Info("🚀 JOB ĐÃ BẮT ĐẦU CHẠY")

	// Gọi hàm logic thực sự
	err := DoSyncPancakePosShopsWarehouses_v2()
	duration := time.Since(startTime)
	durationMs := duration.Milliseconds()

	if err != nil {
		LogJobError(j.GetName(), err, duration.String(), durationMs)
		return err
	}

	LogJobEnd(j.GetName(), duration.String(), durationMs)
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
	// Lấy logger riêng cho job này
	// File log sẽ là: logs/sync-pancake-pos-shops-warehouses-job.log
	jobLogger := GetJobLoggerByName("sync-pancake-pos-shops-warehouses-job")

	// Thực hiện xác thực và đồng bộ dữ liệu cơ bản
	SyncBaseAuth()

	// Lấy danh sách tokens từ FolkForm với filter system: "Pancake POS"
	filter := `{"system":"Pancake POS"}`
	page := 1
	limit := 50

	jobLogger.Info("Bắt đầu đồng bộ shop và warehouse từ Pancake POS về FolkForm...")

	for {
		// Dừng nửa giây trước khi tiếp tục
		time.Sleep(100 * time.Millisecond)

		// Lấy danh sách access token với filter system: "Pancake POS"
		accessTokens, err := integrations.FolkForm_GetAccessTokens(page, limit, filter)
		if err != nil {
			jobLogger.WithError(err).Error("Lỗi khi lấy danh sách access token")
			return errors.New("Lỗi khi lấy danh sách access token")
		}

		// Xử lý response - có thể là pagination object hoặc array trực tiếp
		items, itemCount, err := parseResponseData(accessTokens)
		if err != nil {
			jobLogger.WithError(err).Error("LỖI khi parse response")
			return err
		}
		jobLogger.WithFields(logrus.Fields{
			"count": len(items),
			"page":  page,
			"limit": limit,
		}).Info("Nhận được access tokens (system: Pancake POS)")

		if itemCount > 0 && len(items) > 0 {
			// Với mỗi token
			for _, item := range items {
				// Dừng nửa giây trước khi tiếp tục
				time.Sleep(100 * time.Millisecond)

				// Chuyển item từ interface{} sang dạng map[string]interface{}
				itemMap, ok := item.(map[string]interface{})
				if !ok {
					jobLogger.WithField("item_type", fmt.Sprintf("%T", item)).Error("LỖI: Item không phải là map")
					continue
				}

				// Lấy api_key từ item (đã được filter ở server, chỉ còn tokens có system: "Pancake POS")
				apiKey, ok := itemMap["value"].(string)
				if !ok {
					jobLogger.Error("LỖI: Không tìm thấy field 'value' trong item")
					continue
				}

				jobLogger.WithField("api_key_length", len(apiKey)).Info("Đang đồng bộ với API key (system: Pancake POS)")

				// 1. Đồng bộ Shops
				jobLogger.Info("Bắt đầu đồng bộ shops...")
				shops, err := integrations.PancakePos_GetShops(apiKey)
				if err != nil {
					jobLogger.WithError(err).Error("LỖI khi lấy danh sách shops")
					// Tiếp tục với token tiếp theo nếu lỗi
					continue
				}

				jobLogger.WithField("count", len(shops)).Info("Nhận được shops")

				// Upsert từng shop vào FolkForm
				for _, shop := range shops {
					// Dừng nửa giây trước khi tiếp tục
					time.Sleep(100 * time.Millisecond)

					shopMap, ok := shop.(map[string]interface{})
					if !ok {
						jobLogger.WithField("shop_type", fmt.Sprintf("%T", shop)).Error("LỖI: Shop không phải là map")
						continue
					}

					_, err := integrations.FolkForm_UpsertShop(shopMap)
					if err != nil {
						jobLogger.WithError(err).Error("LỖI khi upsert shop")
						// Tiếp tục với shop tiếp theo nếu lỗi
						continue
					}
				}

				jobLogger.WithField("count", len(shops)).Info("Đã đồng bộ shops thành công")

				// 2. Đồng bộ Warehouses (cho mỗi shop)
				jobLogger.Info("Bắt đầu đồng bộ warehouses...")
				for _, shop := range shops {
					// Dừng nửa giây trước khi tiếp tục
					time.Sleep(100 * time.Millisecond)

					shopMap, ok := shop.(map[string]interface{})
					if !ok {
						jobLogger.WithField("shop_type", fmt.Sprintf("%T", shop)).Error("LỖI: Shop không phải là map")
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
							jobLogger.WithField("shop_id_type", fmt.Sprintf("%T", shopIdRaw)).Error("LỖI: shopId không phải là số")
							continue
						}
					} else {
						jobLogger.Error("LỖI: Không tìm thấy field 'id' trong shop")
						continue
					}

					// Lấy danh sách warehouses cho shop này
					warehouses, err := integrations.PancakePos_GetWarehouses(apiKey, shopId)
					if err != nil {
						jobLogger.WithError(err).WithField("shop_id", shopId).Error("LỖI khi lấy danh sách warehouses")
						// Tiếp tục với shop tiếp theo nếu lỗi
						continue
					}

					jobLogger.WithFields(logrus.Fields{
						"count":   len(warehouses),
						"shop_id": shopId,
					}).Info("Nhận được warehouses cho shop")

					// Upsert từng warehouse vào FolkForm
					for idx, warehouse := range warehouses {
						// Dừng nửa giây trước khi tiếp tục
						time.Sleep(100 * time.Millisecond)

						warehouseMap, ok := warehouse.(map[string]interface{})
						if !ok {
							jobLogger.WithField("warehouse_type", fmt.Sprintf("%T", warehouse)).Error("LỖI: Warehouse không phải là map")
							continue
						}

						// Log warehouse data để debug
						if id, ok := warehouseMap["id"]; ok {
							jobLogger.WithFields(logrus.Fields{
								"index": idx + 1,
								"total": len(warehouses),
								"id":    id,
								"id_type": fmt.Sprintf("%T", id),
							}).Debug("Đang upsert warehouse")
						} else {
							jobLogger.WithFields(logrus.Fields{
								"index": idx + 1,
								"total": len(warehouses),
								"data":  warehouseMap,
							}).Warn("CẢNH BÁO: Warehouse không có field 'id'")
						}

						_, err := integrations.FolkForm_UpsertWarehouse(warehouseMap)
						if err != nil {
							jobLogger.WithError(err).WithFields(logrus.Fields{
								"index": idx + 1,
								"total": len(warehouses),
							}).Error("LỖI khi upsert warehouse")
							// Tiếp tục với warehouse tiếp theo nếu lỗi
							continue
						}
						jobLogger.WithFields(logrus.Fields{
							"index": idx + 1,
							"total": len(warehouses),
						}).Debug("✅ Đã upsert warehouse thành công")
					}

					jobLogger.WithFields(logrus.Fields{
						"count":   len(warehouses),
						"shop_id": shopId,
					}).Info("Đã đồng bộ warehouses cho shop")
				}

				jobLogger.WithField("api_key_length", len(apiKey)).Info("Đã hoàn thành đồng bộ cho API key")
			}

		} else {
			jobLogger.Info("Không còn access token nào. Kết thúc.")
			break
		}

		page++
		continue
	}

	jobLogger.Info("Đồng bộ shop và warehouse từ Pancake POS về FolkForm thành công")
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
