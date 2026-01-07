/*
Package integrations chứa các hàm tích hợp với các hệ thống bên ngoài.
File localData.go quản lý dữ liệu local (trong memory) để cache thông tin pages từ FolkForm.
Dữ liệu được lưu trong global.PanCake_FbPages và được bảo vệ bởi mutex để tránh race condition.
*/
package integrations

import (
	"errors"
	"log"
	"time"

	"agent_pancake/global"

	"go.mongodb.org/mongo-driver/bson"
)

// Local_SyncPagesFolkformToLocal đồng bộ danh sách trang Facebook từ server FolkForm về local memory
// Hàm này lấy danh sách pages từ FolkForm và lưu vào global.PanCake_FbPages để cache
// Dữ liệu được bảo vệ bởi mutex để đảm bảo thread-safe
// Trả về:
//   - error: Lỗi nếu có trong quá trình đồng bộ
func Local_SyncPagesFolkformToLocal() (resultErr error) {
	limit := 50
	page := 0

	for {
		// Dừng nửa giây trước khi tiếp tục
		time.Sleep(100 * time.Millisecond)

		// Lấy danh sách các pages từ server FolkForm
		resultPages, err := FolkForm_GetFbPages(page, limit)
		if err != nil {
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
						return err
					}

					err = bson.Unmarshal(bsonBytes, &cloudFbPage)
					if err != nil {
						global.PanCake_FbPagesMu.Unlock()
						return err
					}

					// Append cloudFbPage to global.PanCake_FbPages
					global.PanCake_FbPages = append(global.PanCake_FbPages, cloudFbPage)
				}
				global.PanCake_FbPagesMu.Unlock()
			}

			page++
			continue
		} else {
			break
		}

	}

	return nil
}

// local_UpdatePagesAccessToken cập nhật page_access_token cho page có pageId tương ứng vào biến local global.PanCake_FbPages
// Hàm này được sử dụng nội bộ để cập nhật token trong memory cache
// Tham số:
//   - pageId: ID của page cần cập nhật
//   - page_access_token: Token mới cần cập nhật
// Trả về:
//   - error: Lỗi nếu không tìm thấy page hoặc có lỗi khi cập nhật
func local_UpdatePagesAccessToken(pageId string, page_access_token string) (resultErr error) {
	// Tìm index của page (với mutex để tránh race condition)
	global.PanCake_FbPagesMu.RLock()
	foundIndex := -1
	for index, page := range global.PanCake_FbPages {
		if page.PageId == pageId {
			foundIndex = index
			break
		}
	}
	global.PanCake_FbPagesMu.RUnlock()

	// Nếu không tìm thấy, trả về nil (không có lỗi)
	if foundIndex == -1 {
		return nil
	}

	// Cập nhật page_access_token (với mutex và kiểm tra bounds để tránh panic)
	global.PanCake_FbPagesMu.Lock()
	defer global.PanCake_FbPagesMu.Unlock()

	// Kiểm tra lại bounds vì slice có thể đã thay đổi trong lúc unlock
	if foundIndex >= 0 && foundIndex < len(global.PanCake_FbPages) {
		// Kiểm tra lại pageId để đảm bảo đúng page
		if global.PanCake_FbPages[foundIndex].PageId == pageId {
			global.PanCake_FbPages[foundIndex].PageAccessToken = page_access_token
			return nil
		}
	}

	// Nếu không tìm thấy page sau khi lock lại, tìm lại trong slice
	for i, page := range global.PanCake_FbPages {
		if page.PageId == pageId {
			global.PanCake_FbPages[i].PageAccessToken = page_access_token
			return nil
		}
	}

	return nil
}

// Local_UpdatePagesAccessToken cập nhật page_access_token cho page có pageId tương ứng
// Hàm này:
//   1. Tìm page trong global.PanCake_FbPages
//   2. Gọi Pancake API để generate page_access_token mới
//   3. Cập nhật token vào local cache
// Tham số:
//   - pageId: ID của page cần cập nhật token
// Trả về:
//   - error: Lỗi nếu không tìm thấy page, không thể generate token, hoặc có lỗi khi cập nhật
func Local_UpdatePagesAccessToken(pageId string) (resultErr error) {
	// Tìm page và lấy thông tin cần thiết (với mutex để tránh race condition)
	global.PanCake_FbPagesMu.RLock()
	var foundIndex int = -1
	var accessToken string
	for i, page := range global.PanCake_FbPages {
		if page.PageId == pageId {
			foundIndex = i
			accessToken = page.AccessToken
			break
		}
	}
	global.PanCake_FbPagesMu.RUnlock()

	// Nếu không tìm thấy page, trả về lỗi
	if foundIndex == -1 {
		return errors.New("Không tìm thấy page")
	}

	// Gọi hàm PanCake_GeneratePageAccessToken để lấy page_access_token (không cần lock vì chỉ đọc accessToken)
	resultGeneratePageAccessToken, err := PanCake_GeneratePageAccessToken(pageId, accessToken)
	if err != nil {
		log.Println("Lỗi khi lấy page access token: ", err)
		return err
	}

	// chuyển resultGeneratePageAccessToken từ interface{} sang dạng map[string]interface{}
	page_access_token := resultGeneratePageAccessToken["page_access_token"].(string)

	// Cập nhật page_access_token (với mutex và kiểm tra bounds để tránh panic)
	global.PanCake_FbPagesMu.Lock()
	defer global.PanCake_FbPagesMu.Unlock()

	// Kiểm tra lại bounds vì slice có thể đã thay đổi trong lúc gọi API
	if foundIndex >= 0 && foundIndex < len(global.PanCake_FbPages) {
		// Kiểm tra lại pageId để đảm bảo đúng page
		if global.PanCake_FbPages[foundIndex].PageId == pageId {
			global.PanCake_FbPages[foundIndex].PageAccessToken = page_access_token
			return nil
		}
	}

	// Nếu không tìm thấy page sau khi lock lại, có thể slice đã bị thay đổi
	// Tìm lại page trong slice
	for i, page := range global.PanCake_FbPages {
		if page.PageId == pageId {
			global.PanCake_FbPages[i].PageAccessToken = page_access_token
			return nil
		}
	}

	return errors.New("Không tìm thấy page sau khi cập nhật")
}

// Local_GetPageAccessToken lấy page_access_token từ biến local global.PanCake_FbPages
// Hàm này tìm page trong cache và trả về page_access_token
// Nếu không tìm thấy, trả về lỗi để caller có thể gọi Local_UpdatePagesAccessToken
// Tham số:
//   - pageId: ID của page cần lấy token
// Trả về:
//   - pageAccessToken: Token của page, hoặc rỗng nếu không tìm thấy
//   - error: Lỗi nếu không tìm thấy page
func Local_GetPageAccessToken(pageId string) (pageAccessToken string, resultErr error) {
	// Find page in global.PanCake_FbPages (với mutex để tránh race condition)
	global.PanCake_FbPagesMu.RLock()
	defer global.PanCake_FbPagesMu.RUnlock()

	for _, page := range global.PanCake_FbPages {
		if page.PageId == pageId {
			return page.PageAccessToken, nil
		}
	}
	return "", errors.New("Không tìm thấy page")
}
