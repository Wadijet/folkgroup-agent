/*
Package global chứa các biến toàn cục được sử dụng trong toàn bộ ứng dụng.
Các biến này bao gồm:
- GlobalConfig: Cấu hình của ứng dụng
- ApiToken: Token xác thực với FolkForm backend
- ActiveRoleId: Role ID hiện tại đang làm việc (cho Organization Context System)
- PanCake_FbPages: Cache danh sách Facebook pages trong memory
- NotificationRateLimiter: Rate limiter cho notifications
Tất cả các biến được bảo vệ bởi mutex để đảm bảo thread-safe.
*/
package global

import (
	"agent_pancake/config"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// GlobalConfig chứa cấu hình của ứng dụng (được load từ environment variables hoặc .env file)
var GlobalConfig *config.Configuration

// ApiToken là token xác thực với FolkForm backend (được set sau khi login)
var ApiToken string = ""

// ActiveRoleId là Role ID hiện tại đang làm việc (cho Organization Context System - API v3.2+)
// Header X-Active-Role-ID bắt buộc phải có trong mọi request đến FolkForm backend
var ActiveRoleId string = ""

// FbPage là struct chứa thông tin của một Facebook page
type FbPage struct {
	Id              primitive.ObjectID     `json:"id,omitempty" bson:"_id,omitempty"`        // ID của quyền
	PageName        string                 `json:"pageName" bson:"pageName"`                 // Tên của trang
	PageUsername    string                 `json:"pageUsername" bson:"pageUsername"`         // Tên người dùng của trang
	PageId          string                 `json:"pageId" bson:"pageId" index:"unique;text"` // ID của trang
	IsSync          bool                   `json:"isSync" bson:"isSync"`                     // Trạng thái đồng bộ
	AccessToken     string                 `json:"accessToken" bson:"accessToken"`
	PageAccessToken string                 `json:"pageAccessToken" bson:"pageAccessToken"` // Mã truy cập của trang
	ApiData         map[string]interface{} `json:"apiData" bson:"apiData"`                 // Dữ liệu API
	CreatedAt       int64                  `json:"createdAt" bson:"createdAt"`             // Thời gian tạo quyền
	UpdatedAt       int64                  `json:"updatedAt" bson:"updatedAt"`             // Thời gian cập nhật quyền
}

// PanCake_FbPages là cache danh sách Facebook pages trong memory
// Dữ liệu được đồng bộ từ FolkForm và được sử dụng để lấy page_access_token nhanh chóng
// Thay vì phải gọi API mỗi lần, ta cache trong memory để tăng hiệu năng
var PanCake_FbPages []FbPage

// PanCake_FbPagesMu là mutex để bảo vệ PanCake_FbPages khỏi race condition
// Sử dụng RWMutex để cho phép nhiều goroutines đọc đồng thời
var PanCake_FbPagesMu sync.RWMutex

// NotificationRateLimiter lưu trữ thời gian gửi notification cuối cùng cho mỗi conversation
// Mục đích: Tránh gửi notification quá nhiều lần cho cùng một conversation
// Key: conversationId (string)
// Value: time.Time (thời gian gửi notification cuối cùng)
// Dùng chung giữa các agent instances (nếu cùng process) hoặc có thể persist qua file
var NotificationRateLimiter = make(map[string]time.Time)

// NotificationRateLimiterMu là mutex để bảo vệ NotificationRateLimiter khỏi race condition
// Sử dụng RWMutex để cho phép nhiều goroutines đọc đồng thời
var NotificationRateLimiterMu sync.RWMutex
