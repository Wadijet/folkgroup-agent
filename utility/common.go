/*
Package utility chứa các hàm tiện ích chung được sử dụng trong toàn bộ ứng dụng.
File common.go chứa các hàm helper:
- GoProtect: Bảo vệ hàm khỏi panic
- UnixMilli: Chuyển đổi time.Time sang Unix timestamp (milliseconds)
- String2ObjectID/ObjectID2String: Chuyển đổi giữa string và MongoDB ObjectID
*/
package utility

import (
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// GoProtect là một hàm bao bọc (wrapper) giúp bảo vệ một hàm khác khỏi bị panic
// Nếu xảy ra panic trong hàm f(), GoProtect sẽ bắt lại và in ra lỗi thay vì làm chương trình dừng hẳn
// Tham số:
//   - f: Hàm cần được bảo vệ (không có tham số và không trả về giá trị)
// Ví dụ sử dụng:
//   GoProtect(func() {
//       // Code có thể panic
//   })
func GoProtect(f func()) {
	defer func() {
		// Sử dụng recover() để bắt lỗi panic nếu có
		if err := recover(); err != nil {
			fmt.Printf("Đã bắt lỗi panic: %v\n", err)
		}
	}()

	// Gọi hàm f() được truyền vào
	f()
}

// UnixMilli chuyển đổi time.Time sang Unix timestamp tính bằng milliseconds
// Tham số:
//   - t: Thời gian cần chuyển đổi
// Trả về:
//   - int64: Unix timestamp tính bằng milliseconds
// Ví dụ: time.Now() -> 1699123456789 (milliseconds)
func UnixMilli(t time.Time) int64 {
	return t.Round(time.Millisecond).UnixNano() / (int64(time.Millisecond) / int64(time.Nanosecond))
}

// CurrentTimeInMilli lấy thời gian hiện tại tính bằng milliseconds
// Hàm này là wrapper của UnixMilli(time.Now()) để tiện sử dụng
// Trả về:
//   - int64: Unix timestamp hiện tại tính bằng milliseconds
// Ví dụ: 1699123456789
func CurrentTimeInMilli() int64 {
	return UnixMilli(time.Now())
}

// String2ObjectID chuyển đổi chuỗi hex string thành MongoDB ObjectID
// Tham số:
//   - id: Chuỗi hex string (ví dụ: "507f1f77bcf86cd799439011")
// Trả về:
//   - primitive.ObjectID: ObjectID tương ứng, hoặc primitive.NilObjectID nếu chuỗi không hợp lệ
func String2ObjectID(id string) primitive.ObjectID {
	objectId, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return primitive.NilObjectID
	}
	return objectId
}

// ObjectID2String chuyển đổi MongoDB ObjectID thành chuỗi hex string
// Tham số:
//   - id: ObjectID cần chuyển đổi
// Trả về:
//   - string: Chuỗi hex string (ví dụ: "507f1f77bcf86cd799439011")
func ObjectID2String(id primitive.ObjectID) string {
	stringObjectID := id.Hex()
	return stringObjectID
}
