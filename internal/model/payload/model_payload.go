package payload

// Pagination bounds applied when the client omits or overshoots them.
const (
	DefaultPage    = 1
	DefaultPerPage = 10
	MaxPerPage     = 100
)

// SortRequest is one ordering instruction. Sorting is a list rather than a map
// because Go randomises map iteration order.
type SortRequest struct {
	Field     string `json:"field" validate:"required,max=64"`
	Direction string `json:"direction" validate:"omitempty,oneof=asc desc ASC DESC"`
}

// PageRequest is embedded by every paginated list request.
type PageRequest struct {
	Page    int           `json:"page" validate:"gte=1"`
	PerPage int           `json:"per_page" validate:"gte=1,lte=100"`
	Sort    []SortRequest `json:"sort" validate:"omitempty,max=5,dive"`
	Include []string      `json:"include" validate:"omitempty,max=10,dive,max=64"`
}

// Normalize applies defaults and clamps. Call it before validation.
func (p *PageRequest) Normalize() {
	if p.Page < 1 {
		p.Page = DefaultPage
	}
	if p.PerPage < 1 {
		p.PerPage = DefaultPerPage
	}
	if p.PerPage > MaxPerPage {
		p.PerPage = MaxPerPage
	}
}

// Offset is the SQL offset for the requested page.
func (p PageRequest) Offset() int {
	return (p.Page - 1) * p.PerPage
}

// PageMetadata describes the slice of data returned.
type PageMetadata struct {
	Page      int   `json:"page"`
	PerPage   int   `json:"per_page"`
	TotalData int64 `json:"total_data"`
	TotalPage int64 `json:"total_page"`
}

// PageResponse is the paginated payload returned to clients.
type PageResponse[T any] struct {
	Data     []T          `json:"data"`
	PageMeta PageMetadata `json:"paging"`
}

// NewPageResponse constructs a PageResponse calculating total pages from total data.
func NewPageResponse[T any](data []T, page, perPage int, totalData int64) *PageResponse[T] {
	totalPage := int64(0)
	if perPage > 0 && totalData > 0 {
		totalPage = (totalData + int64(perPage) - 1) / int64(perPage)
	}
	return &PageResponse[T]{
		Data: data,
		PageMeta: PageMetadata{
			Page:      page,
			PerPage:   perPage,
			TotalData: totalData,
			TotalPage: totalPage,
		},
	}
}

// Response is the success envelope.
type Response struct {
	Status       string `json:"status"`
	StatusCode   int    `json:"status_code"`
	ResponseData any    `json:"response_data"`
	RequestID    string `json:"request_id,omitempty"`
}

// ErrorDetail is the client-safe description of a failure.
type ErrorDetail struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Details   any    `json:"details,omitempty"`
	Path      string `json:"path"`
	Timestamp string `json:"timestamp"`
}

// ResponseError is the failure envelope.
type ResponseError struct {
	Status     string      `json:"status"`
	StatusCode int         `json:"status_code"`
	Error      ErrorDetail `json:"error"`
	RequestID  string      `json:"request_id,omitempty"`
}
