/*
Package jobs chứa các job cụ thể của ứng dụng.
File này chứa các hàm helper chung được sử dụng bởi nhiều job.
*/
package jobs

import (
	"agent_pancake/app/integrations"
	"agent_pancake/app/services"
	"agent_pancake/global"
)

// ========================================
// HÀM HELPER ĐỂ LẤY CONFIG CHO JOBS
// ========================================
// Các hàm này giúp jobs có thể đọc config động từ ConfigManager
// Config có thể được thay đổi từ server và jobs sẽ tự động sử dụng giá trị mới
// Tất cả các hàm đều thread-safe và có fallback về default value nếu không tìm thấy config

// GetJobConfigValue lấy giá trị config cho một field cụ thể của job
// Tham số:
//   - jobName: Tên của job (ví dụ: "sync-incremental-conversations-job")
//   - fieldName: Tên của field config cần lấy (ví dụ: "pageSize", "timeout", "maxRetries")
// Trả về:
//   - interface{}: Giá trị của field (có thể là bất kỳ kiểu nào: int, string, bool, map, array)
//   - bool: true nếu tìm thấy config, false nếu không tìm thấy hoặc ConfigManager chưa được khởi tạo
// Lưu ý: Nếu ConfigManager chưa được khởi tạo (nil), hàm sẽ trả về (nil, false)
func GetJobConfigValue(jobName, fieldName string) (interface{}, bool) {
	configManager := services.GetGlobalConfigManager()
	if configManager == nil {
		// ConfigManager chưa được khởi tạo → trả về false
		// Điều này có thể xảy ra nếu hàm được gọi trước khi main() khởi tạo ConfigManager
		return nil, false
	}
	return configManager.GetJobConfigValue(jobName, fieldName)
}

// GetJobConfigInt lấy giá trị int từ config với fallback về default value
// Tham số:
//   - jobName: Tên của job (ví dụ: "sync-incremental-conversations-job")
//   - fieldName: Tên của field config cần lấy (ví dụ: "pageSize", "timeout")
//   - defaultValue: Giá trị mặc định nếu không tìm thấy config hoặc giá trị không phải int
// Trả về:
//   - int: Giá trị int từ config, hoặc defaultValue nếu không tìm thấy/không hợp lệ
// Ví dụ sử dụng:
//   pageSize := GetJobConfigInt("sync-incremental-conversations-job", "pageSize", 50)
//   // Nếu config có pageSize=100 → trả về 100
//   // Nếu không có config → trả về 50 (default)
func GetJobConfigInt(jobName, fieldName string, defaultValue int) int {
	configManager := services.GetGlobalConfigManager()
	if configManager == nil {
		// ConfigManager chưa được khởi tạo → trả về default value
		return defaultValue
	}
	return configManager.GetJobConfigInt(jobName, fieldName, defaultValue)
}

// GetJobConfigBool lấy giá trị bool từ config với fallback về default value
// Tham số:
//   - jobName: Tên của job (ví dụ: "sync-incremental-conversations-job")
//   - fieldName: Tên của field config cần lấy (ví dụ: "enabled")
//   - defaultValue: Giá trị mặc định nếu không tìm thấy config hoặc giá trị không phải bool
// Trả về:
//   - bool: Giá trị bool từ config, hoặc defaultValue nếu không tìm thấy/không hợp lệ
// Ví dụ sử dụng:
//   enabled := GetJobConfigBool("sync-incremental-conversations-job", "enabled", true)
//   // Nếu config có enabled=false → trả về false
//   // Nếu không có config → trả về true (default)
func GetJobConfigBool(jobName, fieldName string, defaultValue bool) bool {
	configManager := services.GetGlobalConfigManager()
	if configManager == nil {
		// ConfigManager chưa được khởi tạo → trả về default value
		return defaultValue
	}
	return configManager.GetJobConfigBool(jobName, fieldName, defaultValue)
}

// GetJobConfigString lấy giá trị string từ config với fallback về default value
// Tham số:
//   - jobName: Tên của job (ví dụ: "sync-incremental-conversations-job")
//   - fieldName: Tên của field config cần lấy (ví dụ: "schedule")
//   - defaultValue: Giá trị mặc định nếu không tìm thấy config hoặc giá trị không phải string
// Trả về:
//   - string: Giá trị string từ config, hoặc defaultValue nếu không tìm thấy/không hợp lệ
// Ví dụ sử dụng:
//   schedule := GetJobConfigString("sync-incremental-conversations-job", "schedule", "0 */1 * * * *")
//   // Nếu config có schedule="0 */5 * * * *" → trả về "0 */5 * * * *"
//   // Nếu không có config → trả về "0 */1 * * * *" (default)
func GetJobConfigString(jobName, fieldName string, defaultValue string) string {
	configManager := services.GetGlobalConfigManager()
	if configManager == nil {
		// ConfigManager chưa được khởi tạo → trả về default value
		return defaultValue
	}
	return configManager.GetJobConfigString(jobName, fieldName, defaultValue)
}

// SyncBaseAuth thực hiện xác thực và đồng bộ dữ liệu cơ bản.
// Hàm này được sử dụng chung bởi các job để đảm bảo đã đăng nhập và đồng bộ pages.
// Cập nhật: Thêm logic lấy Active Role ID cho Organization Context System (Version 3.2)
func SyncBaseAuth() {
	// Đảm bảo logger đã được khởi tạo
	if JobLogger == nil {
		InitJobLogger()
	}

	// Nếu chưa đăng nhập thì đăng nhập
	_, err := integrations.FolkForm_CheckIn()
	if err != nil {
		JobLogger.Info("Chưa đăng nhập, tiến hành đăng nhập...")
		integrations.FolkForm_Login()
		integrations.FolkForm_CheckIn()
	}

	// Lấy role ID nếu chưa có (Organization Context System - Version 3.2)
	// Backend sẽ tự động detect role đầu tiên nếu không có header X-Active-Role-ID
	// Nhưng nên lấy và lưu để đảm bảo context đúng
	if global.ActiveRoleId == "" {
		JobLogger.Info("Chưa có Active Role ID, đang lấy danh sách roles...")
		roles, err := integrations.FolkForm_GetRoles()
		if err != nil {
			JobLogger.WithError(err).Warn("Lỗi khi lấy roles (Backend sẽ tự động detect role đầu tiên)")
			// Tiếp tục, backend sẽ tự động detect role đầu tiên nếu không có header
		} else if len(roles) > 0 {
			// Lấy role đầu tiên
			if firstRole, ok := roles[0].(map[string]interface{}); ok {
				if roleId, ok := firstRole["id"].(string); ok && roleId != "" {
					global.ActiveRoleId = roleId
					JobLogger.WithField("role_id", roleId).Info("✅ Đã lưu Active Role ID")
				} else if roleId, ok := firstRole["roleId"].(string); ok && roleId != "" {
					global.ActiveRoleId = roleId
					JobLogger.WithField("role_id", roleId).Info("✅ Đã lưu Active Role ID")
				} else {
					JobLogger.Warn("⚠️  Không tìm thấy id hoặc roleId trong role đầu tiên")
				}
			} else {
				JobLogger.Warn("⚠️  Role đầu tiên không phải là map")
			}
		} else {
			JobLogger.Warn("⚠️  Không tìm thấy roles nào (Backend sẽ tự động detect)")
		}
	} else {
		JobLogger.WithField("role_id", global.ActiveRoleId).Debug("✅ Đã có Active Role ID")
	}

	// Đồng bộ danh sách các pages từ pancake sang folkform
	err = integrations.Bridge_SyncPages()
	if err != nil {
		JobLogger.WithError(err).Error("Lỗi khi đồng bộ trang")
	} else {
		JobLogger.Debug("Đồng bộ trang thành công")
	}

	// Đồng bộ danh sách các pages từ pancake sang folkform
	err = integrations.Bridge_UpdatePagesAccessToken_toFolkForm()
	if err != nil {
		JobLogger.WithError(err).Error("Lỗi khi đồng bộ trang")
	} else {
		JobLogger.Debug("Đồng bộ trang thành công")
	}

	// Đồng bộ danh sách các pages từ folkform sang local
	err = integrations.Local_SyncPagesFolkformToLocal()
	if err != nil {
		JobLogger.WithError(err).Error("Lỗi khi đồng bộ trang")
	} else {
		JobLogger.Debug("Đồng bộ trang từ folkform sang local thành công")
	}
}
