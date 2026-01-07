package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/caarlos0/env"
	"github.com/joho/godotenv"
	"agent_pancake/utility/logger"
)

// Configuration chứa thông tin tĩnh cần thiết để chạy ứng dụng
// Nó chứa thông tin cơ sở dữ liệu
type Configuration struct {
	FirebaseApiKey   string `env:"FIREBASE_API_KEY,required"`  // Firebase API Key để đăng nhập
	FirebaseEmail    string `env:"FIREBASE_EMAIL,required"`    // Email để đăng nhập Firebase
	FirebasePassword string `env:"FIREBASE_PASSWORD,required"` // Password để đăng nhập Firebase
	AgentId          string `env:"AGENT_ID,required"`          // ID của agent
	ApiBaseUrl       string `env:"API_BASE_URL,required"`      // Địa chỉ server API
	PancakeBaseUrl   string `env:"PANCAKE_BASE_URL,required"`  // Địa chỉ server Pancake
}

// LogConfig trả về cấu hình logger từ environment variables
func LogConfig() *logger.Config {
	return logger.NewConfig()
}

// NewConfig sẽ đọc dữ liệu cấu hình từ environment variables hoặc file .env
// Ưu tiên: Environment variables (systemd EnvironmentFile) > File .env (development)
func NewConfig(files ...string) *Configuration {
	log.Println("[Config] ========================================")
	log.Println("[Config] Bắt đầu đọc cấu hình...")
	
	cfg := Configuration{}
	
	// Bước 1: Thử parse từ environment variables trước (ưu tiên)
	log.Println("[Config] [Bước 1/2] Kiểm tra environment variables (systemd EnvironmentFile)...")
	// env.Parse sẽ đọc từ os.Getenv()
	// Systemd EnvironmentFile sẽ load env vars vào os.Getenv() trước khi chạy
	err := env.Parse(&cfg)
	if err != nil {
		// Nếu parse từ env vars thất bại, thử load từ file .env
		log.Printf("[Config] [Bước 1/2] ❌ Không thể parse từ environment variables: %v", err)
		log.Println("[Config] [Bước 1/2] Thử load từ file .env...")
	} else {
		// Kiểm tra xem có ít nhất 1 biến được set không (để biết có phải từ systemd không)
		// Nếu tất cả đều empty, có thể là chưa có env vars, thử load file .env
		if cfg.FirebaseApiKey == "" && cfg.FirebaseEmail == "" && cfg.AgentId == "" {
			log.Println("[Config] [Bước 1/2] ⚠️  Environment variables chưa được set (tất cả đều empty)")
			log.Println("[Config] [Bước 1/2] Thử load từ file .env...")
		} else {
			// Có env vars từ systemd, dùng luôn
			log.Println("[Config] [Bước 1/2] ✅ Đã đọc cấu hình từ environment variables (systemd EnvironmentFile)")
			log.Printf("[Config] [Bước 1/2] Config values:")
			log.Printf("[Config]   • FIREBASE_API_KEY: %s...%s (length: %d)", 
				maskString(cfg.FirebaseApiKey, 10, false), 
				maskString(cfg.FirebaseApiKey, 10, true),
				len(cfg.FirebaseApiKey))
			log.Printf("[Config]   • FIREBASE_EMAIL: %s", cfg.FirebaseEmail)
			log.Printf("[Config]   • FIREBASE_PASSWORD: %s (length: %d)", maskPassword(cfg.FirebasePassword), len(cfg.FirebasePassword))
			log.Printf("[Config]   • AGENT_ID: %s", cfg.AgentId)
			log.Printf("[Config]   • API_BASE_URL: %s", cfg.ApiBaseUrl)
			log.Printf("[Config]   • PANCAKE_BASE_URL: %s", cfg.PancakeBaseUrl)
			log.Println("[Config] ========================================")
			return &cfg
		}
	}
	
	// Bước 2: Fallback về file env (cho development)
	log.Println("[Config] [Bước 2/2] Load từ file env (development)...")
	
	// Tìm file env theo thứ tự ưu tiên:
	// 1. Tham số files được truyền vào (nếu có)
	// 2. Biến môi trường ENV_FILE_PATH hoặc CONFIG_PATH
	// 3. Đường dẫn mặc định cho VPS: /home/dungdm/folkform/config/agent.env
	// 4. Đường dẫn tương đối: agent.env hoặc .env (cho development)
	var envPath string
	var found bool
	
	// Ưu tiên 1: Tham số files được truyền vào
	if len(files) > 0 && files[0] != "" {
		envPath = files[0]
		log.Printf("[Config] [Bước 2/2] Sử dụng đường dẫn từ tham số: %s", envPath)
		found = true
	} else {
		// Ưu tiên 2: Biến môi trường ENV_FILE_PATH hoặc CONFIG_PATH
		if envPath = os.Getenv("ENV_FILE_PATH"); envPath != "" {
			log.Printf("[Config] [Bước 2/2] Sử dụng đường dẫn từ ENV_FILE_PATH: %s", envPath)
			found = true
		} else if envPath = os.Getenv("CONFIG_PATH"); envPath != "" {
			// Nếu CONFIG_PATH là thư mục, thêm agent.env vào
			if info, err := os.Stat(envPath); err == nil && info.IsDir() {
				envPath = filepath.Join(envPath, "agent.env")
			}
			log.Printf("[Config] [Bước 2/2] Sử dụng đường dẫn từ CONFIG_PATH: %s", envPath)
			found = true
		} else {
			// Ưu tiên 3: Đường dẫn mặc định cho VPS
			vpsPath := "/home/dungdm/folkform/config/agent.env"
			if _, err := os.Stat(vpsPath); err == nil {
				envPath = vpsPath
				log.Printf("[Config] [Bước 2/2] Tìm thấy file agent.env tại đường dẫn VPS mặc định: %s", envPath)
				found = true
			} else {
				// Ưu tiên 4: Đường dẫn tương đối (cho development)
				// Thử agent.env trước, sau đó thử .env
				envPath = filepath.Join("agent.env")
				log.Printf("[Config] [Bước 2/2] Sử dụng đường dẫn tương đối: %s", envPath)
				found = false // Sẽ kiểm tra sau
			}
		}
	}
	
	// Thử load file env
	log.Printf("[Config] [Bước 2/2] Tìm file env tại: %s", envPath)
	err = godotenv.Load(envPath)
	if err != nil {
		if found {
			// Nếu đã chỉ định đường dẫn cụ thể nhưng không tìm thấy, log warning
			log.Printf("[Config] [Bước 2/2] ⚠️  Không tìm thấy file env tại đường dẫn đã chỉ định: %s", envPath)
		} else {
			// Nếu là đường dẫn tương đối, thử thêm các vị trí khác
			log.Printf("[Config] [Bước 2/2] ❌ Không tìm thấy file env tại %s", envPath)
			
			// Thử thêm các vị trí khác (ưu tiên agent.env, sau đó .env)
			alternativePaths := []string{
				"/home/dungdm/folkform/config/agent.env", // VPS path với agent.env
				filepath.Join("config", "agent.env"),      // config/agent.env
				filepath.Join(".env"),                      // .env (backward compatibility)
				"/home/dungdm/folkform/config/.env",        // VPS path với .env (backward compatibility)
			}
			
			for _, altPath := range alternativePaths {
				log.Printf("[Config] [Bước 2/2] Thử đường dẫn thay thế: %s", altPath)
				if err2 := godotenv.Load(altPath); err2 == nil {
					envPath = altPath
					log.Printf("[Config] [Bước 2/2] ✅ Đã tìm thấy file env tại: %s", envPath)
					err = nil
					break
				}
			}
			
			if err != nil {
				log.Printf("[Config] [Bước 2/2] Error: %v", err)
				log.Println("[Config] [Bước 2/2] Sẽ dùng environment variables nếu có")
			}
		}
	} else {
		log.Printf("[Config] [Bước 2/2] ✅ Đã load file env từ %s", envPath)
	}
	
	// Parse lại sau khi load file .env (có thể override env vars nếu file .env có giá trị)
	log.Println("[Config] [Bước 2/2] Parse config từ file .env...")
	err = env.Parse(&cfg)
	if err != nil {
		log.Printf("[Config] [Bước 2/2] ❌ Lỗi khi parse config: %+v", err)
		fmt.Printf("Lỗi khi parse config: %+v\n", err)
	} else {
		log.Println("[Config] [Bước 2/2] ✅ Parse config thành công")
		log.Printf("[Config] [Bước 2/2] Config values:")
		log.Printf("[Config]   • FIREBASE_API_KEY: %s...%s (length: %d)", 
			maskString(cfg.FirebaseApiKey, 10, false), 
			maskString(cfg.FirebaseApiKey, 10, true),
			len(cfg.FirebaseApiKey))
		log.Printf("[Config]   • FIREBASE_EMAIL: %s", cfg.FirebaseEmail)
		log.Printf("[Config]   • FIREBASE_PASSWORD: %s (length: %d)", maskPassword(cfg.FirebasePassword), len(cfg.FirebasePassword))
		log.Printf("[Config]   • AGENT_ID: %s", cfg.AgentId)
		log.Printf("[Config]   • API_BASE_URL: %s", cfg.ApiBaseUrl)
		log.Printf("[Config]   • PANCAKE_BASE_URL: %s", cfg.PancakeBaseUrl)
	}

	log.Println("[Config] ========================================")
	return &cfg
}

// Helper function để mask string (chỉ hiển thị đầu và cuối)
func maskString(s string, visibleLen int, fromEnd bool) string {
	if len(s) <= visibleLen*2 {
		return "***"
	}
	if fromEnd {
		return s[len(s)-visibleLen:]
	}
	return s[:visibleLen]
}

// Helper function để mask password
func maskPassword(pwd string) string {
	if len(pwd) == 0 {
		return "(empty)"
	}
	if len(pwd) <= 4 {
		return "****"
	}
	return pwd[:2] + "****" + pwd[len(pwd)-2:]
}
