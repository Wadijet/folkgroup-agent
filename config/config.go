package config

import (
	"fmt"
	"log"

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
	cfg := Configuration{}
	
	// Bước 1: Thử parse từ environment variables trước (ưu tiên)
	// env.Parse sẽ đọc từ os.Getenv()
	// Systemd EnvironmentFile sẽ load env vars vào os.Getenv() trước khi chạy
	err := env.Parse(&cfg)
	if err != nil {
		// Nếu parse từ env vars thất bại, thử load từ file .env
		log.Printf("Không thể parse từ environment variables: %v, thử load từ file .env\n", err)
	} else {
		// Kiểm tra xem có ít nhất 1 biến được set không (để biết có phải từ systemd không)
		// Nếu tất cả đều empty, có thể là chưa có env vars, thử load file .env
		if cfg.FirebaseApiKey == "" && cfg.FirebaseEmail == "" && cfg.AgentId == "" {
			log.Printf("Environment variables chưa được set, thử load từ file .env\n")
		} else {
			// Có env vars từ systemd, dùng luôn
			log.Printf("Đã đọc cấu hình từ environment variables (systemd EnvironmentFile)\n")
			return &cfg
		}
	}
	
	// Bước 2: Fallback về file .env (cho development)
	envPath := filepath.Join(".env")
	err = godotenv.Load(envPath)
	if err != nil {
		log.Printf("Không tìm thấy file .env tại %s (sẽ dùng environment variables nếu có)\n", envPath)
	} else {
		log.Printf("Đã load file .env từ %s\n", envPath)
	}
	
	// Parse lại sau khi load file .env (có thể override env vars nếu file .env có giá trị)
	err = env.Parse(&cfg)
	if err != nil {
		fmt.Printf("Lỗi khi parse config: %+v\n", err)
	}

	return &cfg
}
