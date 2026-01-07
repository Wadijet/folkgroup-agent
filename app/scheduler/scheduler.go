/*
Package scheduler cung cấp chức năng quản lý và thực thi các tác vụ định kỳ (cron jobs).
Package này sử dụng thư viện robfig/cron để quản lý việc lập lịch các tác vụ.

Các tính năng chính:
- Khởi tạo và quản lý scheduler
- Thêm/xóa/theo dõi các jobs
- Đồng bộ hóa truy cập vào scheduler thông qua mutex
- Hỗ trợ định dạng cron expression với độ chính xác đến giây
*/
package scheduler

import (
	"context"
	"fmt"
	"log"
	"os"
	"runtime"
	"sync"

	"github.com/robfig/cron/v3"
)

// Scheduler đại diện cho một scheduler quản lý các cron jobs.
// Struct này đảm bảo thread-safe thông qua việc sử dụng RWMutex.
type Scheduler struct {
	// cron là instance của cron scheduler từ thư viện robfig/cron
	cron *cron.Cron
	// jobs lưu trữ map giữa tên job và ID của nó trong cron scheduler
	jobs map[string]cron.EntryID
	// jobObjects lưu trữ map giữa tên job và Job object để có thể chạy job ngay lập tức
	jobObjects map[string]Job
	// pausedJobs lưu trữ danh sách các job đang bị pause (tên job và schedule cũ)
	pausedJobs map[string]string
	// disabledJobs lưu trữ danh sách các job đang bị disable (tên job và schedule cũ)
	disabledJobs map[string]string
	// mu là mutex để đồng bộ hóa truy cập vào scheduler
	mu sync.RWMutex
}

// NewScheduler tạo một instance mới của Scheduler.
// Scheduler được khởi tạo với:
// - Cron scheduler có độ chính xác đến giây
// - Map rỗng để lưu trữ jobs
func NewScheduler() *Scheduler {
	return &Scheduler{
		// WithSeconds() cho phép định nghĩa cron expression với độ chính xác đến giây
		cron:         cron.New(cron.WithSeconds()),
		jobs:         make(map[string]cron.EntryID),
		jobObjects:   make(map[string]Job),
		pausedJobs:   make(map[string]string),
		disabledJobs: make(map[string]string),
	}
}

// Start khởi động scheduler.
// Sau khi gọi Start, scheduler sẽ bắt đầu thực thi các jobs theo lịch đã định nghĩa.
// Các jobs mới có thể được thêm vào ngay cả khi scheduler đang chạy.
func (s *Scheduler) Start() {
	log.Printf("[Scheduler] 🚀 Đang khởi động cron scheduler...")
	s.mu.RLock()
	jobCount := len(s.jobs)
	s.mu.RUnlock()
	log.Printf("[Scheduler] 📊 Số lượng jobs đã đăng ký: %d", jobCount)

	// Liệt kê tất cả jobs
	s.mu.RLock()
	for name := range s.jobs {
		log.Printf("[Scheduler]   - Job: %s", name)
	}
	s.mu.RUnlock()

	s.cron.Start()
	log.Printf("[Scheduler] ✅ Cron scheduler đã được khởi động!")
}

// Stop dừng scheduler một cách an toàn.
// - Dừng tất cả các jobs đang chạy
// - Đợi cho đến khi tất cả jobs hoàn thành
// - Trả về context để caller có thể theo dõi khi nào scheduler dừng hoàn toàn
func (s *Scheduler) Stop() context.Context {
	return s.cron.Stop()
}

// AddJob thêm một job mới vào scheduler.
// Tham số:
// - name: Tên định danh của job
// - spec: Biểu thức cron định nghĩa lịch chạy (vd: "0 0 * * *" - chạy lúc 00:00 mỗi ngày)
// - job: Hàm thực thi của job
// Trả về error nếu biểu thức cron không hợp lệ
func (s *Scheduler) AddJob(name string, spec string, job func()) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Nếu job đã tồn tại, xóa job cũ trước khi thêm job mới
	if id, exists := s.jobs[name]; exists {
		log.Printf("[Scheduler] Job %s đã tồn tại, đang xóa job cũ với ID: %d...", name, id)
		s.cron.Remove(id)
		delete(s.jobs, name)
	}

	// Thêm job mới vào cron scheduler
	log.Printf("[Scheduler] Đang thêm job vào cron: %s với spec: %s", name, spec)
	id, err := s.cron.AddFunc(spec, job)
	if err != nil {
		log.Printf("[Scheduler] ❌ Lỗi khi thêm job vào cron: %v", err)
		return err
	}

	// Lưu ID của job để có thể quản lý sau này
	s.jobs[name] = id
	log.Printf("[Scheduler] ✅ Job đã được thêm vào cron với ID: %d", id)
	return nil
}

// AddJobObject thêm một job object vào scheduler một cách tự động.
// Phương thức này tự động tạo wrapper function để gọi Execute() của job,
// giúp code gọn gàng hơn, không cần viết wrapper function mỗi lần.
// Tham số:
// - job: Job object implement interface Job (có Execute, GetName, GetSchedule)
// Trả về error nếu biểu thức cron không hợp lệ hoặc job không hợp lệ
func (s *Scheduler) AddJobObject(job Job) error {
	// Tự động lấy name và schedule từ job object
	name := job.GetName()
	spec := job.GetSchedule()

	log.Printf("[Scheduler] Đang đăng ký job: %s với cron: %s", name, spec)

	// Lưu job object để có thể chạy ngay lập tức sau này
	s.mu.Lock()
	s.jobObjects[name] = job
	s.mu.Unlock()

	// Tự động tạo wrapper function để gọi Execute()
	wrapperFunc := func() {
		// Bắt panic để tránh crash toàn bộ ứng dụng
		defer func() {
			if r := recover(); r != nil {
				// Lấy stack trace để debug
				buf := make([]byte, 4096)
				n := runtime.Stack(buf, false)
				stackTrace := string(buf[:n])

				// Log lỗi panic với đầy đủ thông tin
				log.Printf("[Scheduler] 🚨 PANIC trong job %s: %v", name, r)
				log.Printf("[Scheduler] 📋 Stack trace:\n%s", stackTrace)
				os.Stderr.Sync()
				os.Stdout.Sync()
			}
		}()

		// Đảm bảo log được flush ngay lập tức
		// Log package mặc định ghi vào os.Stderr, nên cần flush cả stderr
		log.Printf("[Scheduler] ⚡ Wrapper function được gọi cho job: %s", name)
		os.Stderr.Sync() // Force flush stderr (log package mặc định dùng stderr)
		os.Stdout.Sync() // Force flush stdout (nếu có set output)

		ctx := context.Background()
		if err := job.Execute(ctx); err != nil {
			// Log lỗi nếu có, có thể mở rộng để gửi alert, retry, etc.
			log.Printf("[Scheduler] ❌ Lỗi khi thực thi job %s: %v", job.GetName(), err)
			os.Stderr.Sync()
			os.Stdout.Sync()
		} else {
			log.Printf("[Scheduler] ✅ Job %s đã hoàn thành thành công", job.GetName())
			os.Stderr.Sync()
			os.Stdout.Sync()
		}
	}

	// Gọi AddJob với wrapper function đã tạo sẵn
	err := s.AddJob(name, spec, wrapperFunc)
	if err != nil {
		// Xóa job object nếu thêm vào cron thất bại
		s.mu.Lock()
		delete(s.jobObjects, name)
		s.mu.Unlock()
		log.Printf("[Scheduler] ❌ Lỗi khi đăng ký job %s: %v", name, err)
		return err
	}
	log.Printf("[Scheduler] ✅ Đã đăng ký job thành công: %s", name)
	return nil
}

// RemoveJob xóa một job khỏi scheduler dựa trên tên của job.
// Job sẽ không được lập lịch chạy nữa sau khi bị xóa.
// Nếu job không tồn tại, hàm này không làm gì cả.
func (s *Scheduler) RemoveJob(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if id, exists := s.jobs[name]; exists {
		s.cron.Remove(id)
		delete(s.jobs, name)
	}
	// Xóa job object và các trạng thái liên quan
	delete(s.jobObjects, name)
	delete(s.pausedJobs, name)
	delete(s.disabledJobs, name)
}

// GetJobs trả về danh sách các jobs đang được quản lý bởi scheduler.
// Trả về một bản sao của map jobs để tránh data race.
// Key là tên job, value là ID của job trong cron scheduler.
func (s *Scheduler) GetJobs() map[string]cron.EntryID {
	s.mu.RLock()
	defer s.mu.RUnlock()

	jobs := make(map[string]cron.EntryID)
	for k, v := range s.jobs {
		jobs[k] = v
	}
	return jobs
}

// GetJobObject trả về job object dựa trên tên job.
// Trả về nil nếu job không tồn tại.
func (s *Scheduler) GetJobObject(name string) Job {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.jobObjects[name]
}

// GetAllJobObjects trả về tất cả job objects (thread-safe)
func (s *Scheduler) GetAllJobObjects() map[string]Job {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	// Copy để tránh data race
	jobs := make(map[string]Job)
	for k, v := range s.jobObjects {
		jobs[k] = v
	}
	return jobs
}

// RunJobNow chạy một job ngay lập tức (không đợi lịch cron).
// Job sẽ chạy trong một goroutine riêng biệt.
func (s *Scheduler) RunJobNow(name string) error {
	s.mu.RLock()
	job, exists := s.jobObjects[name]
	s.mu.RUnlock()

	if !exists {
		return fmt.Errorf("job không tồn tại: %s", name)
	}

	log.Printf("[Scheduler] ▶️  Chạy job ngay lập tức: %s", name)
	
	// Chạy job trong goroutine để không block
	go func() {
		ctx := context.Background()
		if err := job.Execute(ctx); err != nil {
			log.Printf("[Scheduler] ❌ Lỗi khi chạy job %s: %v", name, err)
		} else {
			log.Printf("[Scheduler] ✅ Job %s đã hoàn thành", name)
		}
	}()

	return nil
}

// PauseJob tạm dừng một job (xóa khỏi cron nhưng giữ lại job object và schedule).
func (s *Scheduler) PauseJob(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, exists := s.jobObjects[name]
	if !exists {
		return fmt.Errorf("job không tồn tại: %s", name)
	}

	// Kiểm tra xem job đã bị pause chưa
	if _, alreadyPaused := s.pausedJobs[name]; alreadyPaused {
		log.Printf("[Scheduler] ⚠️  Job %s đã bị pause rồi", name)
		return nil
	}

	// Lưu schedule hiện tại
	schedule := job.GetSchedule()
	s.pausedJobs[name] = schedule

	// Xóa job khỏi cron scheduler
	if id, exists := s.jobs[name]; exists {
		s.cron.Remove(id)
		delete(s.jobs, name)
		log.Printf("[Scheduler] ⏸️  Đã pause job: %s", name)
	}

	return nil
}

// ResumeJob tiếp tục một job đã bị pause.
func (s *Scheduler) ResumeJob(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, exists := s.jobObjects[name]
	if !exists {
		return fmt.Errorf("job không tồn tại: %s", name)
	}

	// Kiểm tra xem job có đang bị pause không
	schedule, isPaused := s.pausedJobs[name]
	if !isPaused {
		log.Printf("[Scheduler] ⚠️  Job %s không bị pause", name)
		return nil
	}

	// Thêm lại job vào cron với schedule cũ
	wrapperFunc := func() {
		defer func() {
			if r := recover(); r != nil {
				buf := make([]byte, 4096)
				n := runtime.Stack(buf, false)
				stackTrace := string(buf[:n])
				log.Printf("[Scheduler] 🚨 PANIC trong job %s: %v", name, r)
				log.Printf("[Scheduler] 📋 Stack trace:\n%s", stackTrace)
				os.Stderr.Sync()
				os.Stdout.Sync()
			}
		}()

		log.Printf("[Scheduler] ⚡ Wrapper function được gọi cho job: %s", name)
		os.Stderr.Sync()
		os.Stdout.Sync()

		ctx := context.Background()
		if err := job.Execute(ctx); err != nil {
			log.Printf("[Scheduler] ❌ Lỗi khi thực thi job %s: %v", job.GetName(), err)
			os.Stderr.Sync()
			os.Stdout.Sync()
		} else {
			log.Printf("[Scheduler] ✅ Job %s đã hoàn thành thành công", job.GetName())
			os.Stderr.Sync()
			os.Stdout.Sync()
		}
	}

	id, err := s.cron.AddFunc(schedule, wrapperFunc)
	if err != nil {
		return fmt.Errorf("lỗi khi resume job %s: %v", name, err)
	}

	s.jobs[name] = id
	delete(s.pausedJobs, name)
	log.Printf("[Scheduler] ▶️  Đã resume job: %s", name)

	return nil
}

// DisableJob vô hiệu hóa một job (tương tự pause nhưng dùng cho disable command).
func (s *Scheduler) DisableJob(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, exists := s.jobObjects[name]
	if !exists {
		return fmt.Errorf("job không tồn tại: %s", name)
	}

	// Kiểm tra xem job đã bị disable chưa
	if _, alreadyDisabled := s.disabledJobs[name]; alreadyDisabled {
		log.Printf("[Scheduler] ⚠️  Job %s đã bị disable rồi", name)
		return nil
	}

	// Lưu schedule hiện tại
	schedule := job.GetSchedule()
	s.disabledJobs[name] = schedule

	// Xóa job khỏi cron scheduler
	if id, exists := s.jobs[name]; exists {
		s.cron.Remove(id)
		delete(s.jobs, name)
		log.Printf("[Scheduler] 🚫 Đã disable job: %s", name)
	}

	return nil
}

// EnableJob kích hoạt lại một job đã bị disable.
func (s *Scheduler) EnableJob(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, exists := s.jobObjects[name]
	if !exists {
		return fmt.Errorf("job không tồn tại: %s", name)
	}

	// Kiểm tra xem job có đang bị disable không
	schedule, isDisabled := s.disabledJobs[name]
	if !isDisabled {
		log.Printf("[Scheduler] ⚠️  Job %s không bị disable", name)
		return nil
	}

	// Thêm lại job vào cron với schedule cũ
	wrapperFunc := func() {
		defer func() {
			if r := recover(); r != nil {
				buf := make([]byte, 4096)
				n := runtime.Stack(buf, false)
				stackTrace := string(buf[:n])
				log.Printf("[Scheduler] 🚨 PANIC trong job %s: %v", name, r)
				log.Printf("[Scheduler] 📋 Stack trace:\n%s", stackTrace)
				os.Stderr.Sync()
				os.Stdout.Sync()
			}
		}()

		log.Printf("[Scheduler] ⚡ Wrapper function được gọi cho job: %s", name)
		os.Stderr.Sync()
		os.Stdout.Sync()

		ctx := context.Background()
		if err := job.Execute(ctx); err != nil {
			log.Printf("[Scheduler] ❌ Lỗi khi thực thi job %s: %v", job.GetName(), err)
			os.Stderr.Sync()
			os.Stdout.Sync()
		} else {
			log.Printf("[Scheduler] ✅ Job %s đã hoàn thành thành công", job.GetName())
			os.Stderr.Sync()
			os.Stdout.Sync()
		}
	}

	id, err := s.cron.AddFunc(schedule, wrapperFunc)
	if err != nil {
		return fmt.Errorf("lỗi khi enable job %s: %v", name, err)
	}

	s.jobs[name] = id
	delete(s.disabledJobs, name)
	log.Printf("[Scheduler] ✅ Đã enable job: %s", name)

	return nil
}

// UpdateJobSchedule cập nhật lịch chạy của một job.
func (s *Scheduler) UpdateJobSchedule(name string, newSchedule string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, exists := s.jobObjects[name]
	if !exists {
		return fmt.Errorf("job không tồn tại: %s", name)
	}

	// Xóa job cũ khỏi cron
	if id, exists := s.jobs[name]; exists {
		s.cron.Remove(id)
		delete(s.jobs, name)
	}

	// Thêm lại job với schedule mới
	wrapperFunc := func() {
		defer func() {
			if r := recover(); r != nil {
				buf := make([]byte, 4096)
				n := runtime.Stack(buf, false)
				stackTrace := string(buf[:n])
				log.Printf("[Scheduler] 🚨 PANIC trong job %s: %v", name, r)
				log.Printf("[Scheduler] 📋 Stack trace:\n%s", stackTrace)
				os.Stderr.Sync()
				os.Stdout.Sync()
			}
		}()

		log.Printf("[Scheduler] ⚡ Wrapper function được gọi cho job: %s", name)
		os.Stderr.Sync()
		os.Stdout.Sync()

		ctx := context.Background()
		if err := job.Execute(ctx); err != nil {
			log.Printf("[Scheduler] ❌ Lỗi khi thực thi job %s: %v", job.GetName(), err)
			os.Stderr.Sync()
			os.Stdout.Sync()
		} else {
			log.Printf("[Scheduler] ✅ Job %s đã hoàn thành thành công", job.GetName())
			os.Stderr.Sync()
			os.Stdout.Sync()
		}
	}

	id, err := s.cron.AddFunc(newSchedule, wrapperFunc)
	if err != nil {
		return fmt.Errorf("lỗi khi cập nhật schedule cho job %s: %v", name, err)
	}

	s.jobs[name] = id
	log.Printf("[Scheduler] 📅 Đã cập nhật schedule cho job: %s (schedule mới: %s)", name, newSchedule)

	return nil
}
