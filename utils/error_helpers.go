package utils

import (
	"fmt"

	"github.com/digitalrebar/provision/v4/models"
)

var pluginName string = "unknown"

func MakeError(code int, msg string) *models.Error {
	return &models.Error{
		Code:  code,
		Model: "plugin",
		Key:   pluginName,
		Type:  "rpc", Messages: []string{msg},
	}
}

func ConvertError(code int, err error) *models.Error {
	if err == nil {
		return nil
	}
	if merr, ok := err.(*models.Error); ok {
		merr.Code = code
		return merr
	}
	return MakeError(code, fmt.Sprintf("Err: %s", err.Error()))
}

func SetErrorName(name string) {
	pluginName = name
}
