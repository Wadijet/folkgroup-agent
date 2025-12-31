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

// NewConfig sẽ đọc dữ liệu cấu hình từ file .env được cung cấp
func NewConfig(files ...string) *Configuration {
	err := godotenv.Load(filepath.Join(".env")) // Tải cấu hình từ file .env
	if err != nil {
		log.Printf("Không tìm thấy file .env %q\n", files)
	}

	cfg := Configuration{}

	// Phân tích env vào cấu hình
	err = env.Parse(&cfg)
	if err != nil {
		fmt.Printf("%+v\n", err)
	}

	return &cfg
}
