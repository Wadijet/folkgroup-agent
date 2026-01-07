/*
Package httpclient cung cấp HTTP client đơn giản để gọi API.
Client hỗ trợ các phương thức GET, POST, PUT, DELETE với timeout và custom headers.
*/
package httpclient

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"time"
)

// HttpClient struct chứa thông tin cấu hình cho HTTP client
type HttpClient struct {
	BaseURL    string            // Base URL của API (ví dụ: "https://api.example.com")
	HTTPClient *http.Client      // HTTP client từ standard library
	Headers    map[string]string // Custom headers (Authorization, Content-Type, etc.)
}

// NewHttpClient tạo một HttpClient mới với base URL và timeout
// Tham số:
//   - baseURL: Base URL của API (ví dụ: "https://api.example.com")
//   - timeout: Timeout cho mỗi request (ví dụ: 10 * time.Second)
// Trả về:
//   - *HttpClient: Instance mới của HttpClient
func NewHttpClient(baseURL string, timeout time.Duration) *HttpClient {
	return &HttpClient{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: timeout,
		},
		Headers: make(map[string]string),
	}
}

// SetHeader thêm hoặc cập nhật header cho client
// Header sẽ được thêm vào tất cả các request sau đó
// Tham số:
//   - key: Tên header (ví dụ: "Authorization", "Content-Type")
//   - value: Giá trị header (ví dụ: "Bearer token123", "application/json")
func (c *HttpClient) SetHeader(key, value string) {
	c.Headers[key] = value
}

// makeRequest tạo và gửi yêu cầu HTTP chung (internal method)
// Tham số:
//   - method: HTTP method (GET, POST, PUT, DELETE)
//   - endpoint: Endpoint path (ví dụ: "/v1/users")
//   - body: Request body (sẽ được marshal thành JSON nếu không nil)
//   - params: Query parameters (sẽ được thêm vào URL)
// Trả về:
//   - *http.Response: HTTP response từ server
//   - error: Lỗi nếu có trong quá trình tạo hoặc gửi request
func (c *HttpClient) makeRequest(method, endpoint string, body interface{}, params map[string]string) (*http.Response, error) {
	// Tạo URL với endpoint
	fullURL, err := url.Parse(c.BaseURL + endpoint)
	if err != nil {
		return nil, err
	}

	// Thêm query params vào URL nếu có
	if params != nil {
		query := fullURL.Query()
		for key, value := range params {
			query.Set(key, value)
		}
		fullURL.RawQuery = query.Encode()
	}

	// Xử lý body nếu không nil
	var requestBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		requestBody = bytes.NewBuffer(jsonBody)
	}

	// Tạo yêu cầu
	req, err := http.NewRequest(method, fullURL.String(), requestBody)
	if err != nil {
		return nil, err
	}

	// Gắn header
	for key, value := range c.Headers {
		req.Header.Set(key, value)
	}

	// Nếu body là JSON
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// Gửi yêu cầu
	return c.HTTPClient.Do(req)
}

// GET gửi yêu cầu HTTP GET
// Tham số:
//   - endpoint: Endpoint path (ví dụ: "/v1/users")
//   - params: Query parameters (sẽ được thêm vào URL)
// Trả về:
//   - *http.Response: HTTP response từ server
//   - error: Lỗi nếu có
func (c *HttpClient) GET(endpoint string, params map[string]string) (*http.Response, error) {
	return c.makeRequest(http.MethodGet, endpoint, nil, params)
}

// POST gửi yêu cầu HTTP POST với body
// Tham số:
//   - endpoint: Endpoint path (ví dụ: "/v1/users")
//   - body: Request body (sẽ được marshal thành JSON)
//   - params: Query parameters (sẽ được thêm vào URL)
// Trả về:
//   - *http.Response: HTTP response từ server
//   - error: Lỗi nếu có
func (c *HttpClient) POST(endpoint string, body interface{}, params map[string]string) (*http.Response, error) {
	return c.makeRequest(http.MethodPost, endpoint, body, params)
}

// PUT gửi yêu cầu HTTP PUT với body
// Tham số:
//   - endpoint: Endpoint path (ví dụ: "/v1/users/123")
//   - body: Request body (sẽ được marshal thành JSON)
//   - params: Query parameters (sẽ được thêm vào URL)
// Trả về:
//   - *http.Response: HTTP response từ server
//   - error: Lỗi nếu có
func (c *HttpClient) PUT(endpoint string, body interface{}, params map[string]string) (*http.Response, error) {
	return c.makeRequest(http.MethodPut, endpoint, body, params)
}

// DELETE gửi yêu cầu HTTP DELETE
// Tham số:
//   - endpoint: Endpoint path (ví dụ: "/v1/users/123")
//   - params: Query parameters (sẽ được thêm vào URL)
// Trả về:
//   - *http.Response: HTTP response từ server
//   - error: Lỗi nếu có
func (c *HttpClient) DELETE(endpoint string, params map[string]string) (*http.Response, error) {
	return c.makeRequest(http.MethodDelete, endpoint, nil, params)
}

// ParseJSONResponse chuyển đổi phản hồi HTTP thành JSON object
// Hàm này đọc response body và unmarshal vào struct hoặc map được truyền vào
// Tham số:
//   - resp: HTTP response từ server
//   - v: Pointer đến struct hoặc map để unmarshal JSON vào (ví dụ: &result)
// Trả về:
//   - error: Lỗi nếu status code không phải 2xx hoặc không thể parse JSON
// Lưu ý: Hàm này tự động đóng response body sau khi đọc
func ParseJSONResponse(resp *http.Response, v interface{}) error {
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return errors.New("API trả về mã lỗi: " + resp.Status)
	}
	defer resp.Body.Close()

	return json.NewDecoder(resp.Body).Decode(v)
}
