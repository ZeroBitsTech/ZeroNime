package httpserver

type Envelope struct {
	Data  any         `json:"data"`
	Meta  map[string]any `json:"meta"`
	Error *APIError   `json:"error"`
}

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func ok(data any, meta map[string]any) Envelope {
	if meta == nil {
		meta = map[string]any{}
	}
	return Envelope{Data: data, Meta: meta, Error: nil}
}

func errEnvelope(code, message string, meta map[string]any) Envelope {
	if meta == nil {
		meta = map[string]any{}
	}
	return Envelope{
		Data:  nil,
		Meta:  meta,
		Error: &APIError{Code: code, Message: message},
	}
}
