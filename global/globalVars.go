package global

import (
	"agent_pancake/config"
	"sync"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

var GlobalConfig *config.Configuration
var ApiToken string = ""
var ActiveRoleId string = "" // Role ID hiện tại đang làm việc (cho Organization Context System)

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

var PanCake_FbPages []FbPage
var PanCake_FbPagesMu sync.RWMutex // Mutex để bảo vệ PanCake_FbPages khỏi race condition
