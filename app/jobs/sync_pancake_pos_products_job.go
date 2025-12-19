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
	"log"
	"strconv"
	"time"
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
	log.Printf("═══════════════════════════════════════════════════════════")
	log.Printf("🚀 JOB ĐÃ BẮT ĐẦU CHẠY: %s", j.GetName())
	log.Printf("📅 Lịch chạy: %s", j.GetSchedule())
	log.Printf("⏰ Thời gian bắt đầu: %s", startTime.Format("2006-01-02 15:04:05"))
	log.Printf("═══════════════════════════════════════════════════════════")

	// Gọi hàm logic thực sự
	err := DoSyncPancakePosProducts_v2()
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
	// Thực hiện xác thực và đồng bộ dữ liệu cơ bản
	SyncBaseAuth()

	// Lấy danh sách tokens từ FolkForm với filter system: "Pancake POS"
	filter := `{"system":"Pancake POS"}`
	page := 1
	limit := 50

	log.Println("Bắt đầu đồng bộ products, variations và categories từ Pancake POS về FolkForm...")

	for {
		// Dừng nửa giây trước khi tiếp tục
		time.Sleep(100 * time.Millisecond)

		// Lấy danh sách access token với filter system: "Pancake POS"
		accessTokens, err := integrations.FolkForm_GetAccessTokens(page, limit, filter)
		if err != nil {
			log.Printf("❌ Lỗi khi lấy danh sách access token: %v", err)
			return errors.New("Lỗi khi lấy danh sách access token")
		}

		// Xử lý response - có thể là pagination object hoặc array trực tiếp
		items, itemCount, err := parseResponseDataProducts(accessTokens)
		if err != nil {
			log.Printf("❌ [DoSyncPancakePosProducts_v2] LỖI khi parse response: %v", err)
			return err
		}
		log.Printf("[DoSyncPancakePosProducts_v2] Nhận được %d access tokens (system: Pancake POS, page=%d, limit=%d)", len(items), page, limit)

		if itemCount > 0 && len(items) > 0 {
			// Với mỗi token
			for _, item := range items {
				// Dừng nửa giây trước khi tiếp tục
				time.Sleep(100 * time.Millisecond)

				// Chuyển item từ interface{} sang dạng map[string]interface{}
				itemMap, ok := item.(map[string]interface{})
				if !ok {
					log.Printf("❌ [DoSyncPancakePosProducts_v2] LỖI: Item không phải là map: %T", item)
					continue
				}

				// Lấy api_key từ item (đã được filter ở server, chỉ còn tokens có system: "Pancake POS")
				apiKey, ok := itemMap["value"].(string)
				if !ok {
					log.Printf("❌ [DoSyncPancakePosProducts_v2] LỖI: Không tìm thấy field 'value' trong item")
					continue
				}

				log.Printf("[DoSyncPancakePosProducts_v2] Đang đồng bộ với API key (system: Pancake POS, length: %d)", len(apiKey))

				// 1. Lấy danh sách shops
				shops, err := integrations.PancakePos_GetShops(apiKey)
				if err != nil {
					log.Printf("❌ [DoSyncPancakePosProducts_v2] LỖI khi lấy danh sách shops: %v", err)
					// Tiếp tục với token tiếp theo nếu lỗi
					continue
				}

				log.Printf("[DoSyncPancakePosProducts_v2] Nhận được %d shops", len(shops))

				// 2. Với mỗi shop
				for _, shop := range shops {
					// Dừng nửa giây trước khi tiếp tục
					time.Sleep(100 * time.Millisecond)

					shopMap, ok := shop.(map[string]interface{})
					if !ok {
						log.Printf("❌ [DoSyncPancakePosProducts_v2] LỖI: Shop không phải là map: %T", shop)
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
							log.Printf("❌ [DoSyncPancakePosProducts_v2] LỖI: shopId không phải là số: %T", shopIdRaw)
							continue
						}
					} else {
						log.Printf("❌ [DoSyncPancakePosProducts_v2] LỖI: Không tìm thấy field 'id' trong shop")
						continue
					}

					log.Printf("[DoSyncPancakePosProducts_v2] Bắt đầu sync cho shopId: %d", shopId)

					// 3. Đồng bộ Products (pagination)
					log.Printf("[DoSyncPancakePosProducts_v2] Bắt đầu đồng bộ products cho shopId: %d", shopId)
					pageNumber := 1
					pageSize := 100

					for {
						// Dừng nửa giây trước khi tiếp tục
						time.Sleep(100 * time.Millisecond)

						products, err := integrations.PancakePos_GetProducts(apiKey, shopId, pageNumber, pageSize)
						if err != nil {
							log.Printf("❌ [DoSyncPancakePosProducts_v2] LỖI khi lấy danh sách products cho shopId %d: %v", shopId, err)
							break
						}

						if len(products) == 0 {
							log.Printf("[DoSyncPancakePosProducts_v2] ShopId %d - Không còn products nào, dừng sync", shopId)
							break
						}

						log.Printf("[DoSyncPancakePosProducts_v2] ShopId %d - Nhận được %d products (page_number=%d)", shopId, len(products), pageNumber)

						// Upsert từng product vào FolkForm
						for idx, product := range products {
							// Dừng nửa giây trước khi tiếp tục
							time.Sleep(100 * time.Millisecond)

							productMap, ok := product.(map[string]interface{})
							if !ok {
								log.Printf("❌ [DoSyncPancakePosProducts_v2] LỖI: Product không phải là map: %T", product)
								continue
							}

							// Log product data để debug
							if id, ok := productMap["id"]; ok {
								log.Printf("[DoSyncPancakePosProducts_v2] Đang upsert product [%d/%d] - id: %v (type: %T)", idx+1, len(products), id, id)
							} else {
								log.Printf("⚠️ [DoSyncPancakePosProducts_v2] CẢNH BÁO: Product [%d/%d] không có field 'id' - data: %+v", idx+1, len(products), productMap)
							}

							// Đảm bảo shop_id có trong product data (vì API không trả về)
							if _, ok := productMap["shop_id"]; !ok {
								productMap["shop_id"] = shopId
								log.Printf("[DoSyncPancakePosProducts_v2] Thêm shop_id vào product data: %d", shopId)
							}

							_, err := integrations.FolkForm_UpsertProductFromPos(productMap, shopId)
							if err != nil {
								log.Printf("❌ [DoSyncPancakePosProducts_v2] LỖI khi upsert product [%d/%d]: %v", idx+1, len(products), err)
								// Tiếp tục với product tiếp theo nếu lỗi
								continue
							}
							log.Printf("[DoSyncPancakePosProducts_v2] ✅ Đã upsert product [%d/%d] thành công", idx+1, len(products))

							// 4. Đồng bộ Variations cho product này
							// Lưu ý: Product có thể đã có variations trong product data (nested)
							// Hoặc cần gọi API riêng để lấy variations
							// Từ data mẫu, variations đã có trong product.variations[]
							if variationsRaw, ok := productMap["variations"]; ok {
								if variationsArray, ok := variationsRaw.([]interface{}); ok && len(variationsArray) > 0 {
									log.Printf("[DoSyncPancakePosProducts_v2] Product có %d variations trong product data, bắt đầu sync...", len(variationsArray))

									// Upsert từng variation vào FolkForm
									for varIdx, variation := range variationsArray {
										// Dừng nửa giây trước khi tiếp tục
										time.Sleep(100 * time.Millisecond)

										variationMap, ok := variation.(map[string]interface{})
										if !ok {
											log.Printf("❌ [DoSyncPancakePosProducts_v2] LỖI: Variation không phải là map: %T", variation)
											continue
										}

										// Đảm bảo shop_id có trong variation data (nếu chưa có)
										if _, ok := variationMap["shop_id"]; !ok {
											variationMap["shop_id"] = shopId
										}

										_, err := integrations.FolkForm_UpsertVariationFromPos(variationMap)
										if err != nil {
											log.Printf("❌ [DoSyncPancakePosProducts_v2] LỖI khi upsert variation [%d/%d]: %v", varIdx+1, len(variationsArray), err)
											// Tiếp tục với variation tiếp theo nếu lỗi
											continue
										}
										log.Printf("[DoSyncPancakePosProducts_v2] ✅ Đã upsert variation [%d/%d] thành công", varIdx+1, len(variationsArray))
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
										log.Printf("⚠️ [DoSyncPancakePosProducts_v2] Không thể convert productId sang string: %T", productIdRaw)
										continue
									}

									if productIdStr != "" {
										// Gọi API để lấy variations (nếu cần)
										// Lưu ý: PancakePos_GetVariations expect productId là int, nhưng thực tế là UUID string
										// Có thể cần update hàm PancakePos_GetVariations để accept UUID string
										// Hoặc bỏ qua và chỉ sync variations từ product data
										log.Printf("[DoSyncPancakePosProducts_v2] Product không có variations trong data, productId: %s (UUID string, không thể gọi API với int)", productIdStr)
									}
								}
							}
						}

						if len(products) < pageSize {
							log.Printf("[DoSyncPancakePosProducts_v2] ShopId %d - Đã lấy hết products (len=%d < page_size=%d)", shopId, len(products), pageSize)
							break
						}

						pageNumber++
					}

					log.Printf("[DoSyncPancakePosProducts_v2] Đã đồng bộ products cho shopId: %d", shopId)

					// 5. Đồng bộ Categories cho shop này
					log.Printf("[DoSyncPancakePosProducts_v2] Bắt đầu đồng bộ categories cho shopId: %d", shopId)
					categories, err := integrations.PancakePos_GetCategories(apiKey, shopId)
					if err != nil {
						log.Printf("❌ [DoSyncPancakePosProducts_v2] LỖI khi lấy danh sách categories cho shopId %d: %v", shopId, err)
						// Tiếp tục với shop tiếp theo nếu lỗi
						continue
					}

					log.Printf("[DoSyncPancakePosProducts_v2] Nhận được %d categories cho shopId: %d", len(categories), shopId)

					// Upsert từng category vào FolkForm
					for idx, category := range categories {
						// Dừng nửa giây trước khi tiếp tục
						time.Sleep(100 * time.Millisecond)

						categoryMap, ok := category.(map[string]interface{})
						if !ok {
							log.Printf("❌ [DoSyncPancakePosProducts_v2] LỖI: Category không phải là map: %T", category)
							continue
						}

						// Log category data để debug
						if id, ok := categoryMap["id"]; ok {
							log.Printf("[DoSyncPancakePosProducts_v2] Đang upsert category [%d/%d] - id: %v (type: %T)", idx+1, len(categories), id, id)
						} else {
							log.Printf("⚠️ [DoSyncPancakePosProducts_v2] CẢNH BÁO: Category [%d/%d] không có field 'id' - data: %+v", idx+1, len(categories), categoryMap)
						}

						_, err := integrations.FolkForm_UpsertCategoryFromPos(categoryMap)
						if err != nil {
							log.Printf("❌ [DoSyncPancakePosProducts_v2] LỖI khi upsert category [%d/%d]: %v", idx+1, len(categories), err)
							// Tiếp tục với category tiếp theo nếu lỗi
							continue
						}
						log.Printf("[DoSyncPancakePosProducts_v2] ✅ Đã upsert category [%d/%d] thành công", idx+1, len(categories))
					}

					log.Printf("[DoSyncPancakePosProducts_v2] Đã đồng bộ %d categories cho shopId: %d", len(categories), shopId)
				}

				log.Printf("[DoSyncPancakePosProducts_v2] Đã hoàn thành đồng bộ cho API key (length: %d)", len(apiKey))
			}

		} else {
			log.Println("[DoSyncPancakePosProducts_v2] Không còn access token nào. Kết thúc.")
			break
		}

		page++
		continue
	}

	log.Println("Đồng bộ products, variations và categories từ Pancake POS về FolkForm thành công")
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
