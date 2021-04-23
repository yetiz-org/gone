package http

import "fmt"

type Acceptance interface {
	Do(req *Request, resp *Response, params map[string]interface{}) error
}

type DispatchAcceptance struct {
}

func (a *DispatchAcceptance) Do(req *Request, resp *Response, params map[string]interface{}) error {
	return nil
}

func (a *DispatchAcceptance) LogExtend(key string, value interface{}, params map[string]interface{}) {
	if rtn := params["[gone-http]extend"]; rtn == nil {
		rtn = map[string]interface{}{key: value}
		params["[gone-http]extend"] = rtn
	} else {
		rtn.(map[string]interface{})[key] = value
	}
}

func (a *DispatchAcceptance) GetNodeName(params map[string]interface{}) string {
	if rtn := params["[gone-http]node_name"]; rtn != nil {
		return rtn.(string)
	}

	return ""
}

func (a *DispatchAcceptance) IsIndex(params map[string]interface{}) string {
	if rtn := params["[gone-http]is_index"]; rtn != nil {
		return rtn.(string)
	}

	return ""
}

func (a *DispatchAcceptance) GetID(name string, params map[string]interface{}) string {
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
