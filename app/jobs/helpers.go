/*
Package jobs chứa các job cụ thể của ứng dụng.
File này chứa các hàm helper chung được sử dụng bởi nhiều job.
*/
package jobs

import (
	"agent_pancake/app/integrations"
	"agent_pancake/global"
)

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
