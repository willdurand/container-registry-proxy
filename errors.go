package main

const (
	ERROR_UNKNOWN = "UNKNOWN"
)

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Detail  string `json:"detail"`
}

type apiErrors struct {
	Errors []apiError `json:"errors"`
}

func makeError(code, message string) apiErrors {
	return apiErrors{
		Errors: []apiError{
			{Code: code, Message: message},
		},
	}
}
