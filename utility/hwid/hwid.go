/*
Package hwid cung cấp các hàm để lấy và tạo Hardware ID (HWID) từ máy tính.
HWID được tạo từ MAC Address của network interface và được hash bằng MD5.
Package này hỗ trợ cross-platform (Windows, Linux, macOS) với nhiều phương pháp fallback.
*/
package hwid

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// getMACAddress lấy MAC Address của network interface đầu tiên hợp lệ
// Hàm này thử nhiều phương pháp để đảm bảo lấy được MAC address trên mọi hệ thống:
//   1. Dùng net.Interfaces() (cross-platform, đáng tin cậy nhất)
//   2. Dùng lệnh getmac trên Windows (fallback)
//   3. Dùng lệnh ipconfig trên Windows (fallback cuối cùng)
// Trả về:
//   - string: MAC Address dạng "XX-XX-XX-XX-XX-XX" (uppercase)
//   - error: Lỗi nếu không thể lấy MAC Address từ bất kỳ phương pháp nào
func getMACAddress() (string, error) {
	// Phương pháp 1: Dùng net package của Go (cross-platform, đáng tin cậy nhất)
	mac, err := getMACAddressFromNetInterfaces()
	if err == nil && mac != "" {
		return mac, nil
	}

	// Phương pháp 2: Dùng lệnh getmac trên Windows (với đường dẫn đầy đủ)
	if runtime.GOOS == "windows" {
		mac, err := getMACAddressFromGetmac()
		if err == nil && mac != "" {
			return mac, nil
		}
	}

	// Phương pháp 3: Dùng lệnh ipconfig trên Windows
	if runtime.GOOS == "windows" {
		mac, err := getMACAddressFromIpconfig()
		if err == nil && mac != "" {
			return mac, nil
		}
	}

	return "", fmt.Errorf("không thể lấy MAC Address từ bất kỳ phương pháp nào")
}

// getMACAddressFromNetInterfaces lấy MAC address từ network interfaces (phương pháp đáng tin cậy nhất)
func getMACAddressFromNetInterfaces() (string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}

	// Tìm interface đầu tiên có MAC address hợp lệ (không phải loopback, không phải virtual)
	for _, iface := range interfaces {
		// Bỏ qua loopback và các interface không có hardware address
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if iface.Flags&net.FlagUp == 0 {
			continue
		}

		// Lấy MAC address
		hwAddr := iface.HardwareAddr
		if len(hwAddr) > 0 {
			// Chuyển đổi sang định dạng string với dấu "-"
			mac := strings.ToUpper(hwAddr.String())
			return mac, nil
		}
	}

	return "", fmt.Errorf("không tìm thấy network interface hợp lệ")
}

// getMACAddressFromGetmac lấy MAC address từ lệnh getmac trên Windows
func getMACAddressFromGetmac() (string, error) {
	// Thử đường dẫn đầy đủ đến getmac.exe
	getmacPaths := []string{
		"getmac",                                    // Thử PATH trước
		filepath.Join("C:", "Windows", "System32", "getmac.exe"),
		filepath.Join("C:", "Windows", "SysWOW64", "getmac.exe"),
	}

	for _, getmacPath := range getmacPaths {
		cmd := exec.Command(getmacPath, "/fo", "csv", "/nh")
		output, err := cmd.Output()
		if err != nil {
			continue // Thử đường dẫn tiếp theo
		}

		// Parse output: format CSV là "Physical Address,Transport Name"
		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			// Parse CSV: lấy phần đầu tiên (Physical Address)
			parts := strings.Split(line, ",")
			if len(parts) > 0 {
				mac := strings.Trim(strings.TrimSpace(parts[0]), "\"")
				// Kiểm tra xem có phải MAC address hợp lệ không (chứa "-" hoặc ":")
				if strings.Contains(mac, "-") || strings.Contains(mac, ":") {
					// Chuẩn hóa về định dạng có dấu "-"
					mac = strings.ReplaceAll(mac, ":", "-")
					return strings.ToUpper(mac), nil
				}
			}
		}
	}

	return "", fmt.Errorf("không thể lấy MAC từ getmac")
}

// getMACAddressFromIpconfig lấy MAC address từ lệnh ipconfig trên Windows
func getMACAddressFromIpconfig() (string, error) {
	// Thử đường dẫn đầy đủ đến ipconfig.exe
	ipconfigPaths := []string{
		"ipconfig",                                    // Thử PATH trước
		filepath.Join("C:", "Windows", "System32", "ipconfig.exe"),
		filepath.Join("C:", "Windows", "SysWOW64", "ipconfig.exe"),
	}

	for _, ipconfigPath := range ipconfigPaths {
		cmd := exec.Command(ipconfigPath, "/all")
		output, err := cmd.Output()
		if err != nil {
			continue // Thử đường dẫn tiếp theo
		}

		// Parse output để tìm dòng "Physical Address"
		lines := strings.Split(string(output), "\n")
		for i, line := range lines {
			line = strings.TrimSpace(line)
			// Tìm dòng chứa "Physical Address"
			if strings.Contains(strings.ToLower(line), "physical address") {
				// Lấy MAC address từ dòng này hoặc dòng tiếp theo
				// Format thường là: "   Physical Address. . . . . . . . : XX-XX-XX-XX-XX-XX"
				parts := strings.Split(line, ":")
				if len(parts) > 1 {
					mac := strings.TrimSpace(parts[len(parts)-1])
					if strings.Contains(mac, "-") || strings.Contains(mac, ":") {
						// Chuẩn hóa về định dạng có dấu "-"
						mac = strings.ReplaceAll(mac, ":", "-")
						return strings.ToUpper(mac), nil
					}
				}
				// Nếu không tìm thấy trong dòng này, thử dòng tiếp theo
				if i+1 < len(lines) {
					nextLine := strings.TrimSpace(lines[i+1])
					if strings.Contains(nextLine, "-") || strings.Contains(nextLine, ":") {
						mac := strings.ReplaceAll(nextLine, ":", "-")
						return strings.ToUpper(mac), nil
					}
				}
			}
		}
	}

	return "", fmt.Errorf("không thể lấy MAC từ ipconfig")
}

// GenerateHardwareID tạo Hardware ID từ MAC Address bằng cách hash MD5
// Hardware ID được dùng để định danh duy nhất cho mỗi máy tính
// Tham số: Không có
// Trả về:
//   - string: Hardware ID (MD5 hash của MAC Address, dạng hex string 32 ký tự)
//   - error: Lỗi nếu không thể lấy MAC Address
// Ví dụ: "a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6"
func GenerateHardwareID() (string, error) {
	macAddress, err := getMACAddress()
	if err != nil {
		return "", err
	}

	// Hash MAC Address bằng MD5
	hash := md5.New()
	hash.Write([]byte(macAddress))
	return hex.EncodeToString(hash.Sum(nil)), nil
}
