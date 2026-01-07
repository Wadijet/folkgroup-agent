/*
Package jobs chứa các job cụ thể của ứng dụng.
File này chứa SyncPancakePosProductsJob - job đồng bộ products, variations và categories từ Pancake POS.
*/
package jobs

import (
	"agent_pancake/app/integrations"
	"agent_pancake/app/scheduler"
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/sirupsen/logrus"
)

// SyncPancakePosProductsJob là job đồng bộ products, variations và categories từ Pancake POS.
// Job này sẽ đồng bộ toàn bộ products, variations và categories từ Pancake POS về FolkForm.
// Sử dụng token lưu ở FolkForm với system: "Pancake POS".
// Sync Products trước, Variations sau (cho mỗi product), Categories cuối cùng.
type SyncPancakePosProductsJob struct {
	*scheduler.BaseJob
}

// NewSyncPancakePosProductsJob tạo một instance mới của SyncPancakePosProductsJob.
// Tham số:
// - name: Tên định danh của job
// - schedule: Biểu thức cron định nghĩa lịch chạy
// Trả về một instance của SyncPancakePosProductsJob
func NewSyncPancakePosProductsJob(name, schedule string) *SyncPancakePosProductsJob {
	job := &SyncPancakePosProductsJob{
		BaseJob: scheduler.NewBaseJob(name, schedule),
	}
	// Set callback function để BaseJob.Execute có thể gọi ExecuteInternal đúng cách
	job.BaseJob.SetExecuteInternalCallback(job.ExecuteInternal)
	return job
}

// ExecuteInternal thực thi logic đồng bộ products, variations và categories từ Pancake POS.
// Phương thức này gọi DoSyncPancakePosProducts_v2() và thêm log wrapper cho job.
// Tham số:
// - ctx: Context để kiểm soát thời gian thực thi
// Trả về error nếu có lỗi xảy ra
func (j *SyncPancakePosProductsJob) ExecuteInternal(ctx context.Context) error {
	startTime := time.Now()
	LogJobStart(j.GetName(), j.GetSchedule()).WithFields(map[string]interface{}{
		"start_time": startTime.Format("2006-01-02 15:04:05"),
	}).Info("🚀 JOB ĐÃ BẮT ĐẦU CHẠY")

	// Gọi hàm logic thực sự
	err := DoSyncPancakePosProducts_v2()
	duration := time.Since(startTime)
	durationMs := duration.Milliseconds()

	if err != nil {
		LogJobError(j.GetName(), err, duration.String(), durationMs)
		return err
	}

	LogJobEnd(j.GetName(), duration.String(), durationMs)
	return nil
}

// DoSyncPancakePosProducts_v2 thực thi logic đồng bộ products, variations và categories từ Pancake POS.
// Hàm này:
//  1. Lấy danh sách tokens từ FolkForm (system: "Pancake POS")
//  2. Với mỗi token, lấy danh sách shops từ Pancake POS
//  3. Với mỗi shop:
//     a. Sync Products (pagination)
//     b. Với mỗi product, sync Variations (nếu cần)
//     c. Sync Categories cho shop
//
// Hàm này có thể được gọi độc lập mà không cần thông qua job interface.
// Trả về error nếu có lỗi xảy ra
func DoSyncPancakePosProducts_v2() error {
	// Lấy logger riêng cho job này
	// File log sẽ là: logs/sync-pancake-pos-products-job.log
	jobLogger := GetJobLoggerByName("sync-pancake-pos-products-job")

	// Thực hiện xác thực và đồng bộ dữ liệu cơ bản
	SyncBaseAuth()

	// Lấy danh sách tokens từ FolkForm với filter system: "Pancake POS"
	filter := `{"system":"Pancake POS"}`
	page := 1
	// Lấy limit từ config động (số lượng access tokens lấy mỗi lần)
	// Nếu không có config, sử dụng default value 50
	// Config này có thể được thay đổi từ server mà không cần restart bot
	limit := GetJobConfigInt("sync-pancake-pos-products-job", "pageSize", 50)

	jobLogger.Info("Bắt đầu đồng bộ products, variations và categories từ Pancake POS về FolkForm...")

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
		items, itemCount, err := parseResponseDataProducts(accessTokens)
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

				// 1. Lấy danh sách shops
				shops, err := integrations.PancakePos_GetShops(apiKey)
				if err != nil {
					jobLogger.WithError(err).Error("LỖI khi lấy danh sách shops")
					// Tiếp tục với token tiếp theo nếu lỗi
					continue
				}

				jobLogger.WithField("count", len(shops)).Info("Nhận được shops")

				// 2. Với mỗi shop
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

					jobLogger.WithField("shop_id", shopId).Info("Bắt đầu sync cho shop")

					// 3. Đồng bộ Products (pagination)
					jobLogger.WithField("shop_id", shopId).Info("Bắt đầu đồng bộ products cho shop")
					pageNumber := 1
					// Lấy pageSize từ config động (có thể thay đổi từ server)
					// Nếu không có config, sử dụng default value 100
					// Config này có thể được thay đổi từ server mà không cần restart bot
					pageSize := GetJobConfigInt("sync-pancake-pos-products-job", "pageSize", 100)

					for {
						// Dừng nửa giây trước khi tiếp tục
						time.Sleep(100 * time.Millisecond)

						products, err := integrations.PancakePos_GetProducts(apiKey, shopId, pageNumber, pageSize)
						if err != nil {
							jobLogger.WithError(err).WithField("shop_id", shopId).Error("LỖI khi lấy danh sách products")
							break
						}

						if len(products) == 0 {
							jobLogger.WithField("shop_id", shopId).Info("Không còn products nào, dừng sync")
							break
						}

						jobLogger.WithFields(logrus.Fields{
							"shop_id":     shopId,
							"count":       len(products),
							"page_number": pageNumber,
						}).Info("Nhận được products")

						// Upsert từng product vào FolkForm
						for idx, product := range products {
							// Dừng nửa giây trước khi tiếp tục
							time.Sleep(100 * time.Millisecond)

							productMap, ok := product.(map[string]interface{})
							if !ok {
								jobLogger.WithField("product_type", fmt.Sprintf("%T", product)).Error("LỖI: Product không phải là map")
								continue
							}

							// Log product data để debug
							if id, ok := productMap["id"]; ok {
								jobLogger.WithFields(logrus.Fields{
									"index":    idx + 1,
									"total":    len(products),
									"id":       id,
									"id_type":  fmt.Sprintf("%T", id),
									"shop_id":  shopId,
								}).Debug("Đang upsert product")
							} else {
								jobLogger.WithFields(logrus.Fields{
									"index": idx + 1,
									"total": len(products),
									"data":  productMap,
								}).Warn("CẢNH BÁO: Product không có field 'id'")
							}

							// Đảm bảo shop_id có trong product data (vì API không trả về)
							if _, ok := productMap["shop_id"]; !ok {
								productMap["shop_id"] = shopId
								jobLogger.WithField("shop_id", shopId).Debug("Thêm shop_id vào product data")
							}

							_, err := integrations.FolkForm_UpsertProductFromPos(productMap, shopId)
							if err != nil {
								jobLogger.WithError(err).WithFields(logrus.Fields{
									"index":   idx + 1,
									"total":   len(products),
									"shop_id": shopId,
								}).Error("LỖI khi upsert product")
								// Tiếp tục với product tiếp theo nếu lỗi
								continue
							}
							jobLogger.WithFields(logrus.Fields{
								"index":   idx + 1,
								"total":   len(products),
								"shop_id": shopId,
							}).Debug("✅ Đã upsert product thành công")

							// 4. Đồng bộ Variations cho product này
							// Lưu ý: Product có thể đã có variations trong product data (nested)
							// Hoặc cần gọi API riêng để lấy variations
							// Từ data mẫu, variations đã có trong product.variations[]
							if variationsRaw, ok := productMap["variations"]; ok {
								if variationsArray, ok := variationsRaw.([]interface{}); ok && len(variationsArray) > 0 {
									jobLogger.WithFields(logrus.Fields{
										"variations_count": len(variationsArray),
										"shop_id":          shopId,
									}).Info("Product có variations trong product data, bắt đầu sync...")

									// Upsert từng variation vào FolkForm
									for varIdx, variation := range variationsArray {
										// Dừng nửa giây trước khi tiếp tục
										time.Sleep(100 * time.Millisecond)

										variationMap, ok := variation.(map[string]interface{})
										if !ok {
											jobLogger.WithField("variation_type", fmt.Sprintf("%T", variation)).Error("LỖI: Variation không phải là map")
											continue
										}

										// Đảm bảo shop_id có trong variation data (nếu chưa có)
										if _, ok := variationMap["shop_id"]; !ok {
											variationMap["shop_id"] = shopId
										}

										_, err := integrations.FolkForm_UpsertVariationFromPos(variationMap)
										if err != nil {
											jobLogger.WithError(err).WithFields(logrus.Fields{
												"index":   varIdx + 1,
												"total":   len(variationsArray),
												"shop_id": shopId,
											}).Error("LỖI khi upsert variation")
											// Tiếp tục với variation tiếp theo nếu lỗi
											continue
										}
										jobLogger.WithFields(logrus.Fields{
											"index":   varIdx + 1,
											"total":   len(variationsArray),
											"shop_id": shopId,
										}).Debug("✅ Đã upsert variation thành công")
									}
								}
							} else {
								// Nếu không có variations trong product data, có thể gọi API riêng
								// Nhưng cần productId là UUID string, không phải số
								if productIdRaw, ok := productMap["id"]; ok {
									var productIdStr string
									switch v := productIdRaw.(type) {
									case string:
										productIdStr = v
									case float64:
										productIdStr = fmt.Sprintf("%.0f", v)
									case int:
										productIdStr = strconv.Itoa(v)
									case int64:
										productIdStr = strconv.FormatInt(v, 10)
									default:
										jobLogger.WithField("product_id_type", fmt.Sprintf("%T", productIdRaw)).Warn("⚠️ Không thể convert productId sang string")
										continue
									}

									if productIdStr != "" {
										// Gọi API để lấy variations (nếu cần)
										// Lưu ý: PancakePos_GetVariations expect productId là int, nhưng thực tế là UUID string
										// Có thể cần update hàm PancakePos_GetVariations để accept UUID string
										// Hoặc bỏ qua và chỉ sync variations từ product data
										jobLogger.WithFields(logrus.Fields{
											"product_id": productIdStr,
											"shop_id":    shopId,
										}).Debug("Product không có variations trong data (UUID string, không thể gọi API với int)")
									}
								}
							}
						}

						if len(products) < pageSize {
							jobLogger.WithFields(logrus.Fields{
								"shop_id":  shopId,
								"count":    len(products),
								"page_size": pageSize,
							}).Info("Đã lấy hết products")
							break
						}

						pageNumber++
					}

					jobLogger.WithField("shop_id", shopId).Info("Đã đồng bộ products cho shop")

					// 5. Đồng bộ Categories cho shop này
					jobLogger.WithField("shop_id", shopId).Info("Bắt đầu đồng bộ categories cho shop")
					categories, err := integrations.PancakePos_GetCategories(apiKey, shopId)
					if err != nil {
						jobLogger.WithError(err).WithField("shop_id", shopId).Error("LỖI khi lấy danh sách categories")
						// Tiếp tục với shop tiếp theo nếu lỗi
						continue
					}

					jobLogger.WithFields(logrus.Fields{
						"count":   len(categories),
						"shop_id": shopId,
					}).Info("Nhận được categories cho shop")

					// Upsert từng category vào FolkForm
					for idx, category := range categories {
						// Dừng nửa giây trước khi tiếp tục
						time.Sleep(100 * time.Millisecond)

						categoryMap, ok := category.(map[string]interface{})
						if !ok {
							jobLogger.WithField("category_type", fmt.Sprintf("%T", category)).Error("LỖI: Category không phải là map")
							continue
						}

						// Log category data để debug
						if id, ok := categoryMap["id"]; ok {
							jobLogger.WithFields(logrus.Fields{
								"index":    idx + 1,
								"total":    len(categories),
								"id":       id,
								"id_type":  fmt.Sprintf("%T", id),
								"shop_id":  shopId,
							}).Debug("Đang upsert category")
						} else {
							jobLogger.WithFields(logrus.Fields{
								"index":   idx + 1,
								"total":   len(categories),
								"data":    categoryMap,
								"shop_id": shopId,
							}).Warn("CẢNH BÁO: Category không có field 'id'")
						}

						_, err := integrations.FolkForm_UpsertCategoryFromPos(categoryMap)
						if err != nil {
							jobLogger.WithError(err).WithFields(logrus.Fields{
								"index":   idx + 1,
								"total":   len(categories),
								"shop_id": shopId,
							}).Error("LỖI khi upsert category")
							// Tiếp tục với category tiếp theo nếu lỗi
							continue
						}
						jobLogger.WithFields(logrus.Fields{
							"index":   idx + 1,
							"total":   len(categories),
							"shop_id": shopId,
						}).Debug("✅ Đã upsert category thành công")
					}

					jobLogger.WithFields(logrus.Fields{
						"count":   len(categories),
						"shop_id": shopId,
					}).Info("Đã đồng bộ categories cho shop")
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

	jobLogger.Info("✅ Đồng bộ products, variations và categories từ Pancake POS về FolkForm thành công")
	return nil
}

// parseResponseDataProducts xử lý response data an toàn - hỗ trợ cả array và pagination object
// Trả về items ([]interface{}) và itemCount (float64)
// Hàm này được copy từ sync_pancake_pos_shops_warehouses_job.go để sử dụng trong job này
func parseResponseDataProducts(response map[string]interface{}) (items []interface{}, itemCount float64, err error) {
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
