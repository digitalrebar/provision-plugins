package utils

import (
	"fmt"

	"github.com/digitalrebar/provision/v4/models"
)

func ValidateBooleanValue(param string, val interface{}) (bool, *models.Error) {
	if val != nil {
		if bval, ok := val.(bool); ok {
			return bval, nil
		} else {
			return false, MakeError(400, fmt.Sprintf("%s must be a boolean", param))
		}
	}
	return false, MakeError(400, fmt.Sprintf("Must specify %s", param))
}

func ValidateStringValue(param string, val interface{}) (string, *models.Error) {
	if val != nil {
		if sval, ok := val.(string); ok {
			return sval, nil
		} else {
			return "", MakeError(400, fmt.Sprintf("%s must be a string", param))
		}
	}
	return "", MakeError(400, fmt.Sprintf("Must specify %s", param))
}

func ValidateIntValue(param string, val interface{}) (int, *models.Error) {
	if val != nil {
		if ival, ok := val.(int); ok {
			return ival, nil
		} else if fval, ok := val.(float64); ok {
			return int(fval), nil
		} else {
			return 0, MakeError(400, fmt.Sprintf("%s must be a number", param))
		}
	}
	return 0, MakeError(400, fmt.Sprintf("Must specify %s", param))
}

func GetParamOrBoolean(m map[string]interface{}, p string, b bool) bool {
	if v, ok := m[p]; ok {
		if bval, ok := v.(bool); ok {
			return bval
		}
	}
	return b
}

func GetParamOrString(m map[string]interface{}, p, s string) string {
	if v, ok := m[p]; ok {
		if sval, ok := v.(string); ok {
			return sval
		}
	}
	return s
}

func GetParamOrInt(m map[string]interface{}, p string, i int) int {
	if v, ok := m[p]; ok {
		if ival, ok := v.(int); ok {
			return ival
		}
		if fval, ok := v.(float64); ok {
			return int(fval)
		}
	}
	return i
}

func GetParamOrInt64(m map[string]interface{}, p string, i int64) int64 {
	if v, ok := m[p]; ok {
		if ival, ok := v.(int64); ok {
			return ival
		}
		if fval, ok := v.(float64); ok {
			return int64(fval)
		}
	}
	return i
}
