package http

import "fmt"

type Acceptance interface {
	Do(req *Request, resp *Response, params map[string]any) error
	SkipMethodOptions() bool
}

type DispatchAcceptance struct {
}

// SkipMethodOptions helpful for CORS xhr preflight
func (a *DispatchAcceptance) SkipMethodOptions() bool {
	return false
}

func (a *DispatchAcceptance) Do(req *Request, resp *Response, params map[string]any) error {
	return nil
}

func (a *DispatchAcceptance) LogExtend(key string, value any, params map[string]any) {
	if rtn := params["[gone-http]extend"]; rtn == nil {
		rtn = map[string]any{key: value}
		params["[gone-http]extend"] = rtn
	} else {
		rtn.(map[string]any)[key] = value
	}
}

func (a *DispatchAcceptance) GetNodeName(params map[string]any) string {
	if rtn := params["[gone-http]node_name"]; rtn != nil {
		return rtn.(string)
	}

	return ""
}

func (a *DispatchAcceptance) IsIndex(params map[string]any) string {
	if rtn := params["[gone-http]is_index"]; rtn != nil {
		return rtn.(string)
	}

	return ""
}

func (a *DispatchAcceptance) GetID(name string, params map[string]any) string {
	if rtn := params[fmt.Sprintf("[gone-http]%s_id", name)]; rtn != nil {
		return rtn.(string)
	}

	return ""
}

type DefaultAcceptanceInterrupt struct {
}

func (DefaultAcceptanceInterrupt) Error() string {
	return "AcceptanceInterrupt"
}

var AcceptanceInterrupt = &DefaultAcceptanceInterrupt{}
