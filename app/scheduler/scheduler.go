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
		cron: cron.New(cron.WithSeconds()),
		jobs: make(map[string]cron.EntryID),
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
