package ghttp

import "fmt"

type TaskHelper struct {
}

func (h *TaskHelper) IsIndex(params map[string]any) bool {
	if rtn := params["[gone-http]is_index"]; rtn != nil {
		if is, ok := rtn.(bool); ok && is {
			return true
		}
	}

	return false
}

func (h *TaskHelper) GetNodeName(params map[string]any) string {
	if rtn := params["[gone-http]node_name"]; rtn != nil {
		return rtn.(string)
	}

	return ""
}

func (h *TaskHelper) GetNode(params map[string]any) RouteNode {
	if rtn := params["[gone-http]node"]; rtn != nil {
		return rtn.(RouteNode)
	}

	return nil
}

func (h *TaskHelper) GetID(name string, params map[string]any) string {
	// 1. Brace syntax: {param} stores as [gone-http]p:param
	if rtn := params[fmt.Sprintf("[gone-http]p:%s", name)]; rtn != nil {
		return rtn.(string)
	}

	// 2. Colon syntax: :param stores as [gone-http]param
	if rtn := params[fmt.Sprintf("[gone-http]%s", name)]; rtn != nil {
		return rtn.(string)
	}

	// 3. Default syntax: node name stores as [gone-http]name_id
	if rtn := params[fmt.Sprintf("[gone-http]%s_id", name)]; rtn != nil {
		return rtn.(string)
	}

	return ""
}

func (h *TaskHelper) LogExtend(key string, value any, params map[string]any) {
	if rtn := params["[gone-http]extend"]; rtn == nil {
		rtn = map[string]any{key: value}
		params["[gone-http]extend"] = rtn
	} else {
		rtn.(map[string]any)[key] = value
	}
}
