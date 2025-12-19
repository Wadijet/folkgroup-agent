/*
Package jobs chứa các job cụ thể của ứng dụng.
File này chứa các hàm helper chung được sử dụng bởi nhiều job.
*/
package jobs

import (
	"agent_pancake/app/integrations"
	"log"
)

// SyncBaseAuth thực hiện xác thực và đồng bộ dữ liệu cơ bản.
// Hàm này được sử dụng chung bởi các job để đảm bảo đã đăng nhập và đồng bộ pages.
func SyncBaseAuth() {
	// Nếu chưa đăng nhập thì đăng nhập
	_, err := integrations.FolkForm_CheckIn()
	if err != nil {
		log.Println("Chưa đăng nhập, tiến hành đăng nhập...")
		integrations.FolkForm_Login()
		integrations.FolkForm_CheckIn()
	}

	// Đồng bộ danh sách các pages từ pancake sang folkform
	err = integrations.Bridge_SyncPages()
	if err != nil {
		log.Println("Lỗi khi đồng bộ trang:", err)
	} else {
		log.Println("Đồng bộ trang thành công")
	}

	// Đồng bộ danh sách các pages từ pancake sang folkform
	err = integrations.Bridge_UpdatePagesAccessToken_toFolkForm()
	if err != nil {
		log.Println("Lỗi khi đồng bộ trang:", err)
	} else {
		log.Println("Đồng bộ trang thành công")
	}

	// Đồng bộ danh sách các pages từ folkform sang local
	err = integrations.Local_SyncPagesFolkformToLocal()
	if err != nil {
		log.Println("Lỗi khi đồng bộ trang:", err)
	} else {
		log.Println("Đồng bộ trang từ folkform sang local thành công")
	}
}
