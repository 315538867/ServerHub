package notify

// Severity 标识通知严重程度。
type Severity string

const (
	SeverityInfo  Severity = "info"
	SeverityWarn  Severity = "warn"
	SeverityError Severity = "error"
)

// Message 是单条待发送通知。
type Message struct {
	Severity Severity
	Title    string
	Body     string
	Tags     map[string]string
}
