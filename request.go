package aiproxy

type Server struct {
	Apitype string `json:"apitype"`
	Baseurl string `json:"baseurl"`
	Apikey  string `json:"apikey"`
	Model   string `json:"model"`
	Ability string `json:"ability"`
}

type RequestBody struct {
	LlmServer []Server `json:"llm_server"`
}

type ResponseBody struct {
	Status string `json:"status"` // ok, fail
	Reason string `json:"reason"` // fail reason
}

const (
	STATUS_OK   = "ok"
	STATUS_FAIL = "fail"
)
