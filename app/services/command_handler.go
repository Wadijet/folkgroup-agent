/*
Package services chứa các services hỗ trợ cho agent.
File này xử lý commands từ server (stop, start, restart, reload_config, run_job, etc.).
*/
package services

import (
	"agent_pancake/app/scheduler"
	"fmt"
	"log"
	"os"
	"time"
)

// CommandHandler xử lý commands từ server
type CommandHandler struct {
	scheduler    *scheduler.Scheduler
	configManager *ConfigManager
}

// NewCommandHandler tạo một instance mới của CommandHandler
func NewCommandHandler(s *scheduler.Scheduler, cm *ConfigManager) *CommandHandler {
	return &CommandHandler{
		scheduler:     s,
		configManager: cm,
	}
}

// ExecuteCommand thực thi command từ server
func (h *CommandHandler) ExecuteCommand(cmd *AgentCommand) error {
	log.Printf("[CommandHandler] Thực thi command: %s (type: %s, target: %s)", 
		cmd.ID, cmd.Type, cmd.Target)

	switch cmd.Type {
	case "stop":
		return h.handleStopCommand(cmd)
	case "start":
		return h.handleStartCommand(cmd)
	case "restart":
		return h.handleRestartCommand(cmd)
	case "reload_config":
		return h.handleReloadConfigCommand(cmd)
	case "shutdown":
		return h.handleShutdownCommand(cmd)
	case "run_job":
		return h.handleRunJobCommand(cmd)
	case "pause_job":
		return h.handlePauseJobCommand(cmd)
	case "resume_job":
		return h.handleResumeJobCommand(cmd)
	case "disable_job":
		return h.handleDisableJobCommand(cmd)
	case "enable_job":
		return h.handleEnableJobCommand(cmd)
	case "update_job_schedule":
		return h.handleUpdateJobScheduleCommand(cmd)
	default:
		log.Printf("[CommandHandler] ❌ Command type không hợp lệ: %s", cmd.Type)
		return nil
	}
}

// handleStopCommand xử lý command stop bot
// Dừng scheduler nhưng không thoát ứng dụng
func (h *CommandHandler) handleStopCommand(cmd *AgentCommand) error {
	log.Printf("[CommandHandler] ⏸️  Dừng bot...")
	if h.scheduler == nil {
		return fmt.Errorf("scheduler không tồn tại")
	}
	
	// Dừng scheduler (các jobs sẽ không chạy nữa)
	ctx := h.scheduler.Stop()
	
	// Đợi một chút để scheduler dừng hoàn toàn
	select {
	case <-ctx.Done():
		log.Printf("[CommandHandler] ✅ Bot đã dừng thành công")
	case <-time.After(5 * time.Second):
		log.Printf("[CommandHandler] ⚠️  Timeout khi đợi bot dừng")
	}
	
	return nil
}

// handleStartCommand xử lý command start bot
// Khởi động lại scheduler
func (h *CommandHandler) handleStartCommand(cmd *AgentCommand) error {
	log.Printf("[CommandHandler] ▶️  Khởi động bot...")
	if h.scheduler == nil {
		return fmt.Errorf("scheduler không tồn tại")
	}
	
	// Khởi động scheduler
	h.scheduler.Start()
	log.Printf("[CommandHandler] ✅ Bot đã được khởi động")
	
	return nil
}

// handleRestartCommand xử lý command restart bot
// Dừng và khởi động lại scheduler
func (h *CommandHandler) handleRestartCommand(cmd *AgentCommand) error {
	log.Printf("[CommandHandler] 🔄 Khởi động lại bot...")
	if h.scheduler == nil {
		return fmt.Errorf("scheduler không tồn tại")
	}
	
	// Dừng scheduler
	ctx := h.scheduler.Stop()
	
	// Đợi scheduler dừng hoàn toàn
	select {
	case <-ctx.Done():
		log.Printf("[CommandHandler] ✅ Scheduler đã dừng")
	case <-time.After(5 * time.Second):
		log.Printf("[CommandHandler] ⚠️  Timeout khi đợi scheduler dừng")
	}
	
	// Khởi động lại scheduler
	h.scheduler.Start()
	log.Printf("[CommandHandler] ✅ Bot đã được khởi động lại")
	
	return nil
}

// handleReloadConfigCommand xử lý command reload config
// Load lại config từ file local hoặc server
func (h *CommandHandler) handleReloadConfigCommand(cmd *AgentCommand) error {
	log.Printf("[CommandHandler] 🔄 Reload config...")
	if h.configManager == nil {
		return fmt.Errorf("config manager không tồn tại")
	}
	
	// Load lại config từ file local với fallback về default
	if err := h.configManager.LoadLocalConfigWithFallback(); err != nil {
		log.Printf("[CommandHandler] ❌ Lỗi khi reload config: %v", err)
		return fmt.Errorf("lỗi khi reload config: %v", err)
	}
	
	log.Printf("[CommandHandler] ✅ Đã reload config thành công")
	return nil
}

// handleShutdownCommand xử lý command shutdown bot
// Dừng scheduler và thoát ứng dụng
func (h *CommandHandler) handleShutdownCommand(cmd *AgentCommand) error {
	log.Printf("[CommandHandler] ⏹️  Shutdown bot...")
	if h.scheduler == nil {
		return fmt.Errorf("scheduler không tồn tại")
	}
	
	// Dừng scheduler
	ctx := h.scheduler.Stop()
	
	// Đợi scheduler dừng hoàn toàn
	select {
	case <-ctx.Done():
		log.Printf("[CommandHandler] ✅ Scheduler đã dừng")
	case <-time.After(5 * time.Second):
		log.Printf("[CommandHandler] ⚠️  Timeout khi đợi scheduler dừng")
	}
	
	log.Printf("[CommandHandler] 🛑 Đang thoát ứng dụng...")
	// Thoát ứng dụng
	os.Exit(0)
	
	return nil
}

// handleRunJobCommand xử lý command run job ngay lập tức
// Cải thiện: Chạy job sync và theo dõi trạng thái để báo lại server
func (h *CommandHandler) handleRunJobCommand(cmd *AgentCommand) error {
	jobName := cmd.Target
	if jobName == "" {
		return fmt.Errorf("tên job không được để trống")
	}
	
	log.Printf("[CommandHandler] ▶️  Chạy job ngay: %s", jobName)
	if h.scheduler == nil {
		return fmt.Errorf("scheduler không tồn tại")
	}
	
	// Chạy job sync để đợi kết quả và theo dõi trạng thái
	// Lưu ý: Có thể block nếu job chạy lâu, nhưng cần để báo lại server về kết quả
	err, result := h.scheduler.RunJobNowSync(jobName)
	if err != nil {
		log.Printf("[CommandHandler] ❌ Lỗi khi chạy job %s: %v", jobName, err)
		return fmt.Errorf("lỗi khi chạy job %s: %v", jobName, err)
	}
	
	if result != nil {
		if result.Success {
			log.Printf("[CommandHandler] ✅ Job %s đã hoàn thành thành công (duration: %.2fs)", jobName, result.Duration)
		} else {
			log.Printf("[CommandHandler] ❌ Job %s thực thi thất bại: %s (duration: %.2fs)", jobName, result.Error, result.Duration)
		}
	} else {
		log.Printf("[CommandHandler] ✅ Đã khởi động job %s", jobName)
	}
	
	return nil
}

// handlePauseJobCommand xử lý command pause job
func (h *CommandHandler) handlePauseJobCommand(cmd *AgentCommand) error {
	jobName := cmd.Target
	if jobName == "" {
		return fmt.Errorf("tên job không được để trống")
	}
	
	log.Printf("[CommandHandler] ⏸️  Pause job: %s", jobName)
	if h.scheduler == nil {
		return fmt.Errorf("scheduler không tồn tại")
	}
	
	// Pause job
	if err := h.scheduler.PauseJob(jobName); err != nil {
		log.Printf("[CommandHandler] ❌ Lỗi khi pause job %s: %v", jobName, err)
		return fmt.Errorf("lỗi khi pause job %s: %v", jobName, err)
	}
	
	log.Printf("[CommandHandler] ✅ Đã pause job %s", jobName)
	return nil
}

// handleResumeJobCommand xử lý command resume job
func (h *CommandHandler) handleResumeJobCommand(cmd *AgentCommand) error {
	jobName := cmd.Target
	if jobName == "" {
		return fmt.Errorf("tên job không được để trống")
	}
	
	log.Printf("[CommandHandler] ▶️  Resume job: %s", jobName)
	if h.scheduler == nil {
		return fmt.Errorf("scheduler không tồn tại")
	}
	
	// Resume job
	if err := h.scheduler.ResumeJob(jobName); err != nil {
		log.Printf("[CommandHandler] ❌ Lỗi khi resume job %s: %v", jobName, err)
		return fmt.Errorf("lỗi khi resume job %s: %v", jobName, err)
	}
	
	log.Printf("[CommandHandler] ✅ Đã resume job %s", jobName)
	return nil
}

// handleDisableJobCommand xử lý command disable job
func (h *CommandHandler) handleDisableJobCommand(cmd *AgentCommand) error {
	jobName := cmd.Target
	if jobName == "" {
		return fmt.Errorf("tên job không được để trống")
	}
	
	log.Printf("[CommandHandler] 🚫 Disable job: %s", jobName)
	if h.scheduler == nil {
		return fmt.Errorf("scheduler không tồn tại")
	}
	
	// Disable job
	if err := h.scheduler.DisableJob(jobName); err != nil {
		log.Printf("[CommandHandler] ❌ Lỗi khi disable job %s: %v", jobName, err)
		return fmt.Errorf("lỗi khi disable job %s: %v", jobName, err)
	}
	
	log.Printf("[CommandHandler] ✅ Đã disable job %s", jobName)
	return nil
}

// handleEnableJobCommand xử lý command enable job
func (h *CommandHandler) handleEnableJobCommand(cmd *AgentCommand) error {
	jobName := cmd.Target
	if jobName == "" {
		return fmt.Errorf("tên job không được để trống")
	}
	
	log.Printf("[CommandHandler] ✅ Enable job: %s", jobName)
	if h.scheduler == nil {
		return fmt.Errorf("scheduler không tồn tại")
	}
	
	// Enable job
	if err := h.scheduler.EnableJob(jobName); err != nil {
		log.Printf("[CommandHandler] ❌ Lỗi khi enable job %s: %v", jobName, err)
		return fmt.Errorf("lỗi khi enable job %s: %v", jobName, err)
	}
	
	log.Printf("[CommandHandler] ✅ Đã enable job %s", jobName)
	return nil
}

// handleUpdateJobScheduleCommand xử lý command update job schedule
func (h *CommandHandler) handleUpdateJobScheduleCommand(cmd *AgentCommand) error {
	jobName := cmd.Target
	if jobName == "" {
		return fmt.Errorf("tên job không được để trống")
	}
	
	// Lấy schedule mới từ params
	newSchedule, ok := cmd.Params["schedule"].(string)
	if !ok || newSchedule == "" {
		return fmt.Errorf("schedule mới không hợp lệ hoặc không được cung cấp")
	}
	
	log.Printf("[CommandHandler] 📅 Update schedule cho job: %s (schedule mới: %s)", jobName, newSchedule)
	if h.scheduler == nil {
		return fmt.Errorf("scheduler không tồn tại")
	}
	
	// Cập nhật schedule
	if err := h.scheduler.UpdateJobSchedule(jobName, newSchedule); err != nil {
		log.Printf("[CommandHandler] ❌ Lỗi khi cập nhật schedule cho job %s: %v", jobName, err)
		return fmt.Errorf("lỗi khi cập nhật schedule cho job %s: %v", jobName, err)
	}
	
	log.Printf("[CommandHandler] ✅ Đã cập nhật schedule cho job %s", jobName)
	return nil
}
